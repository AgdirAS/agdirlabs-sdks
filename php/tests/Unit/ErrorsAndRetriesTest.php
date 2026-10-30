<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Tests\Unit;

use AgdirLabs\Email\Exception\AgdirEmailException;
use AgdirLabs\Email\Tests\Support\MockClient;
use GuzzleHttp\Psr7\Response;
use PHPUnit\Framework\TestCase;

final class ErrorsAndRetriesTest extends TestCase
{
    private const EMAIL = ['from' => 'a@b.co', 'to' => 'c@d.co', 'subject' => 's', 'text' => 't'];

    /** @param array<string, string> $headers */
    private static function apiError(int $status, string $name, string $message, array $headers = []): Response
    {
        return new Response($status, ['Content-Type' => 'application/json'] + $headers, json_encode([
            'statusCode' => $status, 'name' => $name, 'message' => $message,
        ], JSON_THROW_ON_ERROR));
    }

    private static function catch(callable $fn): AgdirEmailException
    {
        try {
            $fn();
        } catch (AgdirEmailException $e) {
            return $e;
        }
        self::fail('expected an AgdirEmailException');
    }

    public function testThrowsWithTheApiCodeStatusAndMessage(): void
    {
        $mock = new MockClient(self::apiError(422, 'validation_error', 'Invalid `to` field'));
        $e = self::catch(fn () => $mock->client()->emails->send(self::EMAIL));
        $this->assertSame('validation_error', $e->errorCode);
        $this->assertSame('validation_error', $e->getErrorCode());
        $this->assertSame(422, $e->status);
        $this->assertSame(422, $e->getCode());
        $this->assertSame('Invalid `to` field', $e->getMessage());
        $this->assertNull($e->retryAfter);
        $this->assertSame(1, $mock->count());
    }

    public function testNonJsonErrorBodiesFallBackToTheStatus(): void
    {
        $mock = new MockClient(new Response(502, ['Content-Type' => 'text/html'], '<html>Bad gateway</html>'));
        $e = self::catch(fn () => $mock->client(maxRetries: 0)->domains->get('d'));
        $this->assertSame('Request failed with HTTP 502', $e->getMessage());
        $this->assertSame('internal_server_error', $e->errorCode);
        $this->assertSame(502, $e->status);
    }

    public function testRetriesRateLimitsAfterRetryAfterReusingTheIdempotencyKey(): void
    {
        $mock = new MockClient(
            self::apiError(429, 'rate_limit_exceeded', 'Too many requests', ['retry-after' => '1']),
            MockClient::json(['id' => 'em_1']),
        );
        $this->assertSame('em_1', $mock->client()->emails->send(self::EMAIL)->id);
        $this->assertSame(2, $mock->count());
        $this->assertSame([1.0], $mock->sleeps);
        $key = $mock->requests[0]->getHeaderLine('Idempotency-Key');
        $this->assertNotSame('', $key);
        $this->assertSame($key, $mock->requests[1]->getHeaderLine('Idempotency-Key'));
        $this->assertSame((string) $mock->requests[0]->getBody(), (string) $mock->requests[1]->getBody());
    }

    public function testRateLimitsAreRetriedEvenWithoutAnIdempotencyKey(): void
    {
        $mock = new MockClient(
            self::apiError(429, 'rate_limit_exceeded', 'slow down', ['retry-after' => '0']),
            MockClient::json(['object' => 'domain', 'id' => 'd']),
        );
        $this->assertSame('d', $mock->client()->domains->delete('d')->id);
    }

    public function testExposesRetryAfterWhenRetriesAreExhausted(): void
    {
        $mock = new MockClient(self::apiError(429, 'rate_limit_exceeded', 'slow down', ['retry-after' => '2']));
        $e = self::catch(fn () => $mock->client(maxRetries: 0)->emails->send(self::EMAIL));
        $this->assertSame('rate_limit_exceeded', $e->errorCode);
        $this->assertSame(2.0, $e->retryAfter);
    }

    public function testDoesNotRetryQuotaErrors(): void
    {
        foreach (['daily_quota_exceeded', 'monthly_quota_exceeded'] as $code) {
            $mock = new MockClient(self::apiError(429, $code, 'Quota reached'));
            $e = self::catch(fn () => $mock->client()->emails->send(self::EMAIL));
            $this->assertSame($code, $e->errorCode);
            $this->assertSame(1, $mock->count());
        }
    }

    public function testRetriesGetServerErrorsAndNetworkFailuresThenGivesUpAfterMaxRetries(): void
    {
        $mock = new MockClient(
            self::apiError(503, 'internal_server_error', 'down'),
            MockClient::networkError(),
            MockClient::json(['object' => 'list', 'has_more' => false, 'data' => []]),
        );
        $this->assertSame([], $mock->client()->domains->list()->data);
        $this->assertSame(3, $mock->count());
        $this->assertCount(2, $mock->sleeps);
        $this->assertGreaterThanOrEqual(0.25, $mock->sleeps[0]);
        $this->assertLessThanOrEqual(0.5, $mock->sleeps[0]);
        $this->assertGreaterThanOrEqual(0.5, $mock->sleeps[1]);
        $this->assertLessThanOrEqual(1.0, $mock->sleeps[1]);

        $mock = new MockClient(
            self::apiError(500, 'internal_server_error', 'a'),
            self::apiError(500, 'internal_server_error', 'b'),
            self::apiError(500, 'internal_server_error', 'c'),
        );
        $e = self::catch(fn () => $mock->client()->domains->get('d'));
        $this->assertSame('c', $e->getMessage());
        $this->assertSame(3, $mock->count());
    }

    public function testDoesNotRetryANonIdempotentPostAfterANetworkFailure(): void
    {
        $mock = new MockClient(MockClient::networkError());
        $e = self::catch(fn () => $mock->client()->domains->create('acme.com'));
        $this->assertSame('network_error', $e->errorCode);
        $this->assertSame(0, $e->status);
        $this->assertSame(1, $mock->count());
        $this->assertInstanceOf(\Throwable::class, $e->getPrevious());
    }

    public function testDoesNotRetryAServerErrorOnANonIdempotentRequest(): void
    {
        $mock = new MockClient(self::apiError(500, 'internal_server_error', 'boom'));
        self::catch(fn () => $mock->client()->webhooks->create('https://a.co/h', ['email.sent']));
        $this->assertSame(1, $mock->count());
    }

    public function testReportsTimeouts(): void
    {
        $mock = new MockClient(MockClient::networkError('cURL error 28: Operation timed out after 30001 milliseconds'));
        $e = self::catch(fn () => $mock->client(maxRetries: 0)->domains->get('d'));
        $this->assertSame('timeout', $e->errorCode);
        $this->assertSame(0, $e->status);
    }
}
