<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Tests\Unit;

use AgdirLabs\Email\Exception\WebhookVerificationException;
use AgdirLabs\Email\Model\WebhookEventType;
use AgdirLabs\Email\Webhook;
use PHPUnit\Framework\TestCase;

final class WebhookVerifyTest extends TestCase
{
    private const NOW = 1_793_350_000;

    private string $secret;
    private string $payload;

    protected function setUp(): void
    {
        $this->secret = 'whsec_' . base64_encode(random_bytes(24));
        $this->payload = json_encode([
            'type' => 'email.bounced',
            'created_at' => '2026-10-30T09:00:00.000Z',
            'data' => [
                'created_at' => '2026-10-30T08:59:00.000Z',
                'email_id' => 'em_1',
                'message_id' => '<abc@eu-north-1.amazonses.com>',
                'from' => 'Acme <hello@acme.com>',
                'to' => ['a@example.com'],
                'subject' => 'Hi',
                'tags' => ['kind' => 'welcome'],
                'bounce' => ['message' => 'Mailbox does not exist', 'subType' => 'General', 'type' => 'Permanent'],
            ],
        ], JSON_THROW_ON_ERROR);
    }

    private static function sign(string $secret, string $id, int $timestamp, string $payload): string
    {
        $key = base64_decode(substr($secret, strlen('whsec_')), true);

        return 'v1,' . base64_encode(hash_hmac('sha256', "{$id}.{$timestamp}.{$payload}", (string) $key, true));
    }

    /** @return array<string, string> */
    private function headers(?string $signature = null, int $timestamp = self::NOW): array
    {
        return [
            'webhook-id' => 'msg_1',
            'webhook-timestamp' => (string) $timestamp,
            'webhook-signature' => $signature ?? self::sign($this->secret, 'msg_1', $timestamp, $this->payload),
        ];
    }

    private function now(int $offset = 0): \DateTimeImmutable
    {
        return (new \DateTimeImmutable())->setTimestamp(self::NOW + $offset);
    }

    public function testAcceptsAValidSignatureAndReturnsTheEvent(): void
    {
        $event = Webhook::verify($this->payload, $this->headers(), $this->secret, now: $this->now());

        $this->assertSame(WebhookEventType::Bounced, $event->type);
        $this->assertSame('2026-10-30T09:00:00+00:00', $event->createdAt->format(DATE_ATOM));
        $this->assertSame('em_1', $event->data->emailId);
        $this->assertSame('<abc@eu-north-1.amazonses.com>', $event->data->messageId);
        $this->assertSame(['a@example.com'], $event->data->to);
        $this->assertSame(['kind' => 'welcome'], $event->data->tags);
        $this->assertNotNull($event->data->bounce);
        $this->assertSame('Permanent', $event->data->bounce->type);
        $this->assertSame('General', $event->data->bounce->subType);
        $this->assertNull($event->data->click);
        $this->assertNull($event->data->failed);
    }

    public function testAcceptsCaseInsensitiveListHeadersObjectsAndOneOfSeveralSignatures(): void
    {
        $good = self::sign($this->secret, 'msg_1', self::NOW, $this->payload);
        $other = self::sign('whsec_' . base64_encode('other-secret-bytes'), 'msg_1', self::NOW, $this->payload);
        $headers = [
            'Webhook-Id' => ['msg_1'],
            'Webhook-Timestamp' => [(string) self::NOW],
            'Webhook-Signature' => ["{$other} {$good}"],
        ];
        $this->assertSame('em_1', Webhook::verify($this->payload, $headers, $this->secret, now: $this->now())->data->emailId);

        $bag = new class ($this->headers()) {
            /** @param array<string, string> $h */
            public function __construct(private array $h)
            {
            }

            public function get(string $name): ?string
            {
                return $this->h[strtolower($name)] ?? null;
            }
        };
        $this->assertSame('em_1', Webhook::verify($this->payload, $bag, $this->secret, now: $this->now())->data->emailId);

        $psr7 = new \GuzzleHttp\Psr7\Request('POST', 'https://acme.com/hook', $this->headers(), $this->payload);
        $this->assertSame('em_1', Webhook::verify((string) $psr7->getBody(), $psr7, $this->secret, now: $this->now())->data->emailId);

        $server = [
            'HTTP_WEBHOOK_ID' => 'msg_1',
            'HTTP_WEBHOOK_TIMESTAMP' => (string) self::NOW,
            'HTTP_WEBHOOK_SIGNATURE' => $good,
        ];
        $this->assertSame('em_1', Webhook::verify($this->payload, $server, $this->secret, now: $this->now())->data->emailId);
    }

    public function testMapsClickAndFailedEvents(): void
    {
        $payload = json_encode([
            'type' => 'email.clicked',
            'created_at' => '2026-10-30T09:00:00Z',
            'data' => [
                'created_at' => '2026-10-30T09:00:00Z', 'email_id' => 'em_2', 'from' => 'a@b.co', 'to' => 'c@d.co', 'subject' => 's',
                'click' => ['ipAddress' => '1.2.3.4', 'link' => 'https://x.co', 'timestamp' => '2026-10-30T09:01:00Z', 'userAgent' => 'UA'],
                'failed' => ['reason' => 'nope'],
            ],
        ], JSON_THROW_ON_ERROR);
        $headers = ['webhook-id' => 'm', 'webhook-timestamp' => (string) self::NOW, 'webhook-signature' => self::sign($this->secret, 'm', self::NOW, $payload)];
        $event = Webhook::verify($payload, $headers, $this->secret, now: $this->now());
        $this->assertSame(['c@d.co'], $event->data->to);
        $this->assertSame([], $event->data->tags);
        $this->assertNull($event->data->messageId);
        $this->assertNotNull($event->data->click);
        $this->assertSame('https://x.co', $event->data->click->link);
        $this->assertSame('1.2.3.4', $event->data->click->ipAddress);
        $this->assertSame('nope', $event->data->failed?->reason);
    }

    public function testRejectsATamperedBody(): void
    {
        $this->expectException(WebhookVerificationException::class);
        $this->expectExceptionMessage('signature does not match');
        Webhook::verify(str_replace('em_1', 'em_2', $this->payload), $this->headers(), $this->secret, now: $this->now());
    }

    public function testRejectsAWrongSecret(): void
    {
        $this->expectException(WebhookVerificationException::class);
        Webhook::verify($this->payload, $this->headers(), 'whsec_' . base64_encode('wrong'), now: $this->now());
    }

    public function testRejectsMissingHeaders(): void
    {
        foreach (['webhook-id', 'webhook-timestamp', 'webhook-signature'] as $missing) {
            $headers = $this->headers();
            unset($headers[$missing]);
            try {
                Webhook::verify($this->payload, $headers, $this->secret, now: $this->now());
                $this->fail("accepted without {$missing}");
            } catch (WebhookVerificationException $e) {
                $this->assertSame('Missing webhook signature headers', $e->getMessage());
            }
        }
    }

    public function testRejectsTimestampsOutsideTheTolerance(): void
    {
        foreach ([-301, 301] as $offset) {
            try {
                Webhook::verify($this->payload, $this->headers(), $this->secret, now: $this->now($offset));
                $this->fail("accepted a timestamp {$offset} s off");
            } catch (WebhookVerificationException $e) {
                $this->assertStringContainsString('tolerance', $e->getMessage());
            }
        }
        // Within a custom tolerance.
        $this->assertSame('em_1', Webhook::verify($this->payload, $this->headers(), $this->secret, tolerance: 600, now: $this->now(500))->data->emailId);
    }

    public function testRejectsNonV1SignaturesAndAnInvalidSecret(): void
    {
        $sig = substr(self::sign($this->secret, 'msg_1', self::NOW, $this->payload), 3);
        try {
            Webhook::verify($this->payload, $this->headers('v2,' . $sig), $this->secret, now: $this->now());
            $this->fail('accepted a v2 signature');
        } catch (WebhookVerificationException) {
        }
        $this->expectExceptionMessage('not valid base64');
        Webhook::verify($this->payload, $this->headers(), 'whsec_***', now: $this->now());
    }
}
