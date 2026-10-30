<?php

declare(strict_types=1);

namespace AgdirLabs\Email;

use AgdirLabs\Email\Internal\Transport;
use AgdirLabs\Email\Resource\Domains;
use AgdirLabs\Email\Resource\Emails;
use AgdirLabs\Email\Resource\Webhooks;
use Http\Discovery\Psr17FactoryDiscovery;
use Http\Discovery\Psr18ClientDiscovery;
use Psr\Http\Client\ClientInterface;
use Psr\Http\Message\RequestFactoryInterface;
use Psr\Http\Message\StreamFactoryInterface;

/**
 * Agdir Email API client.
 *
 * ```php
 * $agdir = new AgdirEmail('re_…');
 * $agdir->emails->send(['from' => …, 'to' => …, 'subject' => …, 'html' => …]);
 * ```
 */
final class AgdirEmail
{
    public const VERSION = '0.1.0';
    public const USER_AGENT = 'agdirlabs-email-php/' . self::VERSION;
    public const DEFAULT_BASE_URL = 'https://api.agdir.email';

    public readonly Emails $emails;
    public readonly Domains $domains;
    public readonly Webhooks $webhooks;
    public readonly string $baseUrl;

    /**
     * @param string|null                  $apiKey     defaults to the `AGDIR_EMAIL_API_KEY` environment variable
     * @param string|null                  $baseUrl    defaults to `AGDIR_EMAIL_BASE_URL`, else https://api.agdir.email
     * @param int                          $maxRetries retries for rate limits, server errors and network failures; 0 disables
     * @param float                        $timeout    seconds per attempt; applies to the auto-discovered Symfony or Guzzle
     *                                                 client (configure your own `$httpClient` yourself)
     * @param ClientInterface|null         $httpClient any PSR-18 client; discovered (Guzzle, Symfony HttpClient, …) if omitted
     * @param (\Closure(float): void)|null $sleep      @internal for tests
     */
    public function __construct(
        ?string $apiKey = null,
        ?string $baseUrl = null,
        int $maxRetries = 2,
        float $timeout = 30.0,
        ?ClientInterface $httpClient = null,
        ?RequestFactoryInterface $requestFactory = null,
        ?StreamFactoryInterface $streamFactory = null,
        ?\Closure $sleep = null,
    ) {
        $apiKey = self::nonEmpty($apiKey) ?? self::env('AGDIR_EMAIL_API_KEY');
        if ($apiKey === null) {
            throw new \InvalidArgumentException(
                'Missing API key: pass it to new AgdirEmail(…) or set AGDIR_EMAIL_API_KEY. Create one at https://app.agdir.email.',
            );
        }
        if ($maxRetries < 0) {
            throw new \InvalidArgumentException('maxRetries must be a non-negative integer.');
        }
        if ($timeout <= 0) {
            throw new \InvalidArgumentException('timeout must be greater than 0 seconds.');
        }

        $this->baseUrl = self::nonEmpty($baseUrl) ?? self::env('AGDIR_EMAIL_BASE_URL') ?? self::DEFAULT_BASE_URL;
        $transport = new Transport(
            apiKey: $apiKey,
            baseUrl: $this->baseUrl,
            client: $httpClient ?? self::discoverClient($timeout),
            requestFactory: $requestFactory ?? Psr17FactoryDiscovery::findRequestFactory(),
            streamFactory: $streamFactory ?? Psr17FactoryDiscovery::findStreamFactory(),
            maxRetries: $maxRetries,
            timeout: $timeout,
            sleep: $sleep,
        );
        $this->emails = new Emails($transport);
        $this->domains = new Domains($transport);
        $this->webhooks = new Webhooks($transport);
    }

    /** Prefers a client we can give a timeout to; falls back to whatever PSR-18 client is installed. */
    private static function discoverClient(float $timeout): ClientInterface
    {
        if (class_exists(\Symfony\Component\HttpClient\Psr18Client::class)
            && class_exists(\Symfony\Component\HttpClient\HttpClient::class)) {
            return new \Symfony\Component\HttpClient\Psr18Client(
                \Symfony\Component\HttpClient\HttpClient::create(['timeout' => $timeout, 'max_duration' => $timeout]),
            );
        }
        if (class_exists(\GuzzleHttp\Client::class)) {
            return new \GuzzleHttp\Client(['timeout' => $timeout, 'connect_timeout' => min($timeout, 10.0)]);
        }

        return Psr18ClientDiscovery::find();
    }

    private static function env(string $name): ?string
    {
        // Empty values (e.g. `AGDIR_EMAIL_BASE_URL=` in a .env file) count as unset.
        $value = $_ENV[$name] ?? $_SERVER[$name] ?? getenv($name);

        return is_string($value) ? self::nonEmpty($value) : null;
    }

    private static function nonEmpty(?string $value): ?string
    {
        return $value === null || $value === '' ? null : $value;
    }
}
