<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Internal;

use AgdirLabs\Email\AgdirEmail;
use AgdirLabs\Email\Exception\AgdirEmailException;
use Psr\Http\Client\ClientExceptionInterface;
use Psr\Http\Client\ClientInterface;
use Psr\Http\Message\RequestFactoryInterface;
use Psr\Http\Message\ResponseInterface;
use Psr\Http\Message\StreamFactoryInterface;

/**
 * Sends API requests, retrying rate limits, server errors and network failures when safe.
 *
 * @internal
 */
final class Transport
{
    private const RETRY_STATUSES = [408, 500, 502, 503, 504];

    /** @var \Closure(float): void */
    private readonly \Closure $sleep;

    /**
     * @param (\Closure(float): void)|null $sleep seconds; injectable for tests
     */
    public function __construct(
        private readonly string $apiKey,
        public readonly string $baseUrl,
        private readonly ClientInterface $client,
        private readonly RequestFactoryInterface $requestFactory,
        private readonly StreamFactoryInterface $streamFactory,
        public readonly int $maxRetries,
        private readonly float $timeout,
        ?\Closure $sleep = null,
    ) {
        $this->sleep = $sleep ?? static function (float $seconds): void {
            usleep((int) round($seconds * 1_000_000));
        };
    }

    /**
     * @param array<string, scalar|null> $query
     * @param array<string, string|null> $headers
     * @param bool                       $retryable safe to repeat: GETs, and POSTs that carry an idempotency key
     *
     * @return array<mixed>
     */
    public function request(
        string $method,
        string $path,
        bool $retryable,
        array $query = [],
        mixed $body = null,
        array $headers = [],
    ): array {
        $url = rtrim($this->baseUrl, '/') . $path;
        $query = array_filter($query, static fn ($v) => $v !== null);
        if ($query !== []) {
            $url .= '?' . http_build_query($query, '', '&', PHP_QUERY_RFC3986);
        }

        $request = $this->requestFactory->createRequest($method, $url)
            ->withHeader('Authorization', 'Bearer ' . $this->apiKey)
            ->withHeader('User-Agent', AgdirEmail::USER_AGENT)
            ->withHeader('Accept', 'application/json');
        if ($body !== null) {
            $json = json_encode($body, JSON_THROW_ON_ERROR | JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
            $request = $request
                ->withHeader('Content-Type', 'application/json')
                ->withBody($this->streamFactory->createStream($json));
        }
        foreach ($headers as $name => $value) {
            if ($value !== null) {
                $request = $request->withHeader($name, $value);
            }
        }

        for ($attempt = 0; ; $attempt++) {
            $canRetry = $attempt < $this->maxRetries;
            $stream = $request->getBody();
            if ($attempt > 0 && $stream->isSeekable()) {
                $stream->rewind();
            }
            try {
                $response = $this->client->sendRequest($request);
            } catch (ClientExceptionInterface $e) {
                // A timed-out POST may have been accepted: only retry it when it is idempotent.
                if ($canRetry && $retryable) {
                    ($this->sleep)(self::backoff($attempt));
                    continue;
                }
                $timedOut = self::isTimeout($e);
                throw new AgdirEmailException(
                    $timedOut
                        ? sprintf('Request timed out after %s s', rtrim(rtrim(number_format($this->timeout, 3, '.', ''), '0'), '.'))
                        : 'Could not reach the Agdir Email API: ' . $e->getMessage(),
                    $timedOut ? AgdirEmailException::TIMEOUT : AgdirEmailException::NETWORK_ERROR,
                    0,
                    null,
                    $e,
                );
            }

            $status = $response->getStatusCode();
            if ($status >= 200 && $status < 300) {
                return self::decode($response);
            }

            $error = self::toError($response);
            // Rate limits are refused before any work is done, so they are always safe to retry.
            $retry = $canRetry && (
                $error->errorCode === AgdirEmailException::RATE_LIMIT_EXCEEDED
                || ($retryable && in_array($status, self::RETRY_STATUSES, true))
            );
            if (!$retry) {
                throw $error;
            }
            ($this->sleep)($error->retryAfter ?? self::backoff($attempt));
        }
    }

    /** Exponential backoff with jitter: ~0.5 s, 1 s, 2 s … capped at 8 s. */
    private static function backoff(int $attempt): float
    {
        $base = min(8.0, 0.5 * 2 ** $attempt);

        return $base / 2 + (mt_rand() / mt_getrandmax()) * ($base / 2);
    }

    /** @return array<mixed> */
    private static function decode(ResponseInterface $response): array
    {
        $raw = (string) $response->getBody();
        if ($raw === '') {
            return [];
        }
        try {
            $data = json_decode($raw, true, 512, JSON_THROW_ON_ERROR);
        } catch (\JsonException $e) {
            throw new AgdirEmailException(
                sprintf('Expected a JSON response, got HTTP %d with a non-JSON body', $response->getStatusCode()),
                'invalid_response',
                $response->getStatusCode(),
                null,
                $e,
            );
        }

        return is_array($data) ? $data : [];
    }

    private static function toError(ResponseInterface $response): AgdirEmailException
    {
        $status = $response->getStatusCode();
        $body = [];
        try {
            $decoded = json_decode((string) $response->getBody(), true, 512, JSON_THROW_ON_ERROR);
            if (is_array($decoded)) {
                $body = $decoded;
            }
        } catch (\JsonException) {
            // Not JSON (e.g. a proxy error page) — fall back to the status line.
        }
        $message = isset($body['message']) && is_string($body['message'])
            ? $body['message']
            : sprintf('Request failed with HTTP %d', $status);
        $code = isset($body['name']) && is_string($body['name'])
            ? $body['name']
            : ($status >= 500 ? AgdirEmailException::INTERNAL_SERVER_ERROR : 'unknown_error');

        return new AgdirEmailException($message, $code, $status, self::retryAfter($response));
    }

    private static function retryAfter(ResponseInterface $response): ?float
    {
        $raw = trim($response->getHeaderLine('retry-after'));
        if ($raw === '') {
            return null;
        }
        if (is_numeric($raw)) {
            return max(0.0, (float) $raw);
        }
        $date = strtotime($raw);

        return $date === false ? null : max(0.0, (float) ($date - time()));
    }

    private static function isTimeout(\Throwable $e): bool
    {
        for ($t = $e; $t !== null; $t = $t->getPrevious()) {
            $message = strtolower($t->getMessage());
            if (str_contains($message, 'timed out') || str_contains($message, 'timeout')
                || str_contains($message, 'max duration') || str_contains($message, 'curl error 28')) {
                return true;
            }
        }

        return false;
    }
}
