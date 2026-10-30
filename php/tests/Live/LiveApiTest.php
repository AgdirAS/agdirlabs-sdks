<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Tests\Live;

use AgdirLabs\Email\AgdirEmail;
use AgdirLabs\Email\Exception\AgdirEmailException;
use AgdirLabs\Email\Mailer\AgdirTransport;
use AgdirLabs\Email\Model\DomainStatus;
use AgdirLabs\Email\Model\EmailStatus;
use AgdirLabs\Email\Model\WebhookEventType;
use AgdirLabs\Email\Model\WebhookStatus;
use PHPUnit\Framework\TestCase;
use Symfony\Component\Mime\Email;

/**
 * Runs against a real Agdir Email API. Opt-in:
 *
 *   AGDIR_EMAIL_LIVE_BASE_URL=https://your-test-api.example \
 *   AGDIR_EMAIL_LIVE_API_KEY=re_… AGDIR_EMAIL_LIVE_FROM=hi@your-verified-domain.com \
 *   vendor/bin/phpunit --testsuite live
 *
 * Never point it at production.
 */
final class LiveApiTest extends TestCase
{
    private static string $baseUrl;
    private static string $apiKey;
    private static string $from;

    public static function setUpBeforeClass(): void
    {
        self::$baseUrl = (string) getenv('AGDIR_EMAIL_LIVE_BASE_URL');
        self::$apiKey = (string) getenv('AGDIR_EMAIL_LIVE_API_KEY');
        self::$from = (string) (getenv('AGDIR_EMAIL_LIVE_FROM') ?: 'hi@php.sdk-test.example.com');
    }

    protected function setUp(): void
    {
        if (self::$baseUrl === '' || self::$apiKey === '') {
            $this->markTestSkipped('Set AGDIR_EMAIL_LIVE_BASE_URL and AGDIR_EMAIL_LIVE_API_KEY to run live tests.');
        }
    }

    private static function client(?string $apiKey = null): AgdirEmail
    {
        // Generous retries: the default rate limit is 10 requests/second per team.
        return new AgdirEmail(apiKey: $apiKey ?? self::$apiKey, baseUrl: self::$baseUrl, maxRetries: 6);
    }

    /**
     * @param array<string, mixed> $extra
     *
     * @return array<string, mixed>
     */
    private static function email(array $extra = []): array
    {
        return $extra + [
            'from' => 'PHP SDK <' . self::$from . '>',
            'to' => 'delivered@example.com',
            'subject' => 'PHP SDK live test',
            'text' => 'Hello from agdirlabs/email',
            'tags' => ['sdk' => 'php'],
        ];
    }

    public function testScheduleGetCancel(): void
    {
        $agdir = self::client();
        $sent = $agdir->emails->send(self::email(['scheduledAt' => new \DateTimeImmutable('+1 hour')]));

        $email = $agdir->emails->get($sent->id);
        $this->assertSame(EmailStatus::Scheduled, $email->status);
        $this->assertSame(['delivered@example.com'], $email->to);
        $this->assertSame([], $email->cc);
        $this->assertNotNull($email->scheduledAt);

        $this->assertSame($sent->id, $agdir->emails->cancel($sent->id)->id);
        $this->assertSame(EmailStatus::Canceled, $agdir->emails->get($sent->id)->status);
    }

    public function testIdempotentReplayReturnsTheSameId(): void
    {
        $agdir = self::client();
        $key = 'php-live-' . bin2hex(random_bytes(8));
        $email = self::email(['scheduledAt' => new \DateTimeImmutable('+1 hour')]);
        $first = $agdir->emails->send($email, idempotencyKey: $key);
        $second = $agdir->emails->send($email, idempotencyKey: $key);
        $this->assertSame($first->id, $second->id);
        $agdir->emails->cancel($first->id);
    }

    public function testPermissiveBatchReportsTheBadRecipient(): void
    {
        $result = self::client()->emails->sendBatch(
            [self::email(), self::email(['to' => 'not-an-email']), self::email()],
            validation: 'permissive',
        );
        $this->assertCount(2, $result->data);
        $this->assertCount(1, $result->errors);
        $this->assertSame(1, $result->errors[0]->index);
    }

    public function testValidationError(): void
    {
        try {
            self::client()->emails->send(self::email(['to' => 'not-an-email']));
            $this->fail('expected a validation error');
        } catch (AgdirEmailException $e) {
            $this->assertSame('validation_error', $e->errorCode);
            $this->assertSame(422, $e->status);
        }
    }

    public function testInvalidApiKey(): void
    {
        try {
            self::client('re_invalid')->domains->list();
            $this->fail('expected invalid_api_key');
        } catch (AgdirEmailException $e) {
            $this->assertSame('invalid_api_key', $e->errorCode);
            $this->assertSame(403, $e->status);
        }
    }

    public function testDomainLifecycle(): void
    {
        $agdir = self::client();
        $name = 'php-' . time() . '-' . bin2hex(random_bytes(3)) . '.test';
        $domain = $agdir->domains->create($name);
        try {
            $this->assertSame($name, $domain->name);
            $this->assertNotEmpty($domain->records);
            $this->assertContains($domain->status, [DomainStatus::NotStarted, DomainStatus::Pending]);

            $got = $agdir->domains->get($domain->id);
            $this->assertSame($domain->id, $got->id);
            $this->assertSame($domain->id, $agdir->domains->verify($domain->id)->id);

            $found = false;
            foreach ($agdir->domains->listAll(limit: 1) as $d) {
                $found = $found || $d->id === $domain->id;
            }
            $this->assertTrue($found, 'listAll(limit: 1) did not reach the new domain');
        } finally {
            $deleted = $agdir->domains->delete($domain->id);
        }
        $this->assertTrue($deleted->deleted);

        try {
            $agdir->domains->get($domain->id);
            $this->fail('expected not_found');
        } catch (AgdirEmailException $e) {
            $this->assertSame('not_found', $e->errorCode);
            $this->assertSame(404, $e->status);
        }
    }

    public function testWebhookLifecycle(): void
    {
        $agdir = self::client();
        $created = $agdir->webhooks->create('https://93.184.216.34/hook', [WebhookEventType::Delivered, 'email.bounced']);
        try {
            $this->assertStringStartsWith('whsec_', $created->signingSecret);

            $webhook = $agdir->webhooks->get($created->id);
            $this->assertSame('https://93.184.216.34/hook', $webhook->endpoint);
            $this->assertSame(WebhookStatus::Enabled, $webhook->status);
            $this->assertSame($created->signingSecret, $webhook->signingSecret);

            $agdir->webhooks->update($created->id, status: WebhookStatus::Disabled);
            $this->assertSame(WebhookStatus::Disabled, $agdir->webhooks->get($created->id)->status);

            $rotated = $agdir->webhooks->rotateSecret($created->id);
            $this->assertNotSame($created->signingSecret, $rotated->signingSecret);

            $deliveries = $agdir->webhooks->deliveries->list($created->id);
            $this->assertSame([], $deliveries->data);
            $this->assertFalse($deliveries->hasMore);
        } finally {
            $this->assertTrue($agdir->webhooks->delete($created->id)->deleted);
        }
    }

    public function testSymfonyMailerTransportSends(): void
    {
        $transport = new AgdirTransport(self::client());
        $email = (new Email())
            ->from(self::$from)
            ->to('delivered@example.com')
            ->subject('PHP SDK Symfony Mailer live test')
            ->html('<p>Hello from the agdir+api transport</p>');
        $email->getHeaders()->addTextHeader('X-Agdir-Tag-sdk', 'php');

        $sent = $transport->send($email);
        $this->assertNotNull($sent);
        $this->assertMatchesRegularExpression('/^[0-9a-f-]{36}$/', $sent->getMessageId());

        $got = self::client()->emails->get($sent->getMessageId());
        $this->assertSame('PHP SDK Symfony Mailer live test', $got->subject);
    }
}
