<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Tests\Support;

use AgdirLabs\Email\AgdirEmail;
use GuzzleHttp\Psr7\HttpFactory;
use GuzzleHttp\Psr7\Response;
use Psr\Http\Client\ClientInterface;
use Psr\Http\Client\NetworkExceptionInterface;
use Psr\Http\Message\RequestInterface;
use Psr\Http\Message\ResponseInterface;

/** A scripted PSR-18 client that records requests. */
final class MockClient implements ClientInterface
{
    /** @var list<RequestInterface> */
    public array $requests = [];

    /** @var list<ResponseInterface|\Throwable> */
    private array $replies;

    /** @var list<float> */
    public array $sleeps = [];

    public function __construct(ResponseInterface|\Throwable ...$replies)
    {
        $this->replies = array_values($replies);
    }

    /** @param array<string, string> $headers */
    public static function json(mixed $body, int $status = 200, array $headers = []): ResponseInterface
    {
        return new Response($status, ['Content-Type' => 'application/json'] + $headers, json_encode($body, JSON_THROW_ON_ERROR));
    }

    public static function networkError(string $message = 'Connection refused'): \Throwable
    {
        return new class ($message) extends \RuntimeException implements NetworkExceptionInterface {
            public function getRequest(): RequestInterface
            {
                return new \GuzzleHttp\Psr7\Request('GET', 'https://example.com');
            }
        };
    }

    public function sendRequest(RequestInterface $request): ResponseInterface
    {
        $this->requests[] = $request;
        $reply = array_shift($this->replies);
        if ($reply === null) {
            throw new \LogicException('MockClient: no reply scripted for ' . $request->getMethod() . ' ' . $request->getUri());
        }
        if ($reply instanceof \Throwable) {
            throw $reply;
        }

        return $reply;
    }

    public function client(int $maxRetries = 2, string $baseUrl = 'https://api.test'): AgdirEmail
    {
        $factory = new HttpFactory();

        return new AgdirEmail(
            apiKey: 're_test',
            baseUrl: $baseUrl,
            maxRetries: $maxRetries,
            httpClient: $this,
            requestFactory: $factory,
            streamFactory: $factory,
            sleep: function (float $seconds): void {
                $this->sleeps[] = $seconds;
            },
        );
    }

    /** @return array<mixed>|null */
    public function body(int $index): ?array
    {
        $raw = (string) $this->requests[$index]->getBody();

        return $raw === '' ? null : json_decode($raw, true, 512, JSON_THROW_ON_ERROR);
    }

    public function count(): int
    {
        return count($this->requests);
    }

    public function remaining(): int
    {
        return count($this->replies);
    }
}
