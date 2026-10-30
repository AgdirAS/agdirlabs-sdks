<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Tests\Unit;

use AgdirLabs\Email\Model\EmailStatus;
use AgdirLabs\Email\Tests\Support\MockClient;
use PHPUnit\Framework\TestCase;

final class EmailsTest extends TestCase
{
    public function testSendsAnEmailAsApiJsonWithAuthUserAgentAndAnIdempotencyKey(): void
    {
        $mock = new MockClient(MockClient::json(['id' => 'em_1']));
        $result = $mock->client()->emails->send([
            'from' => 'Acme <hello@acme.com>',
            'to' => ['a@example.com'],
            'cc' => 'c@example.com',
            'replyTo' => 'support@acme.com',
            'subject' => 'Hi',
            'html' => '<p>Hi</p>',
            'text' => 'Hi',
            'headers' => ['X-Entity-Ref-ID' => '42'],
            'tags' => [['name' => 'kind', 'value' => 'welcome']],
            'scheduledAt' => new \DateTimeImmutable('2026-11-01T10:00:00+01:00'),
        ]);

        $this->assertSame('em_1', $result->id);
        $request = $mock->requests[0];
        $this->assertSame('POST', $request->getMethod());
        $this->assertSame('https://api.test/emails', (string) $request->getUri());
        $this->assertSame('Bearer re_test', $request->getHeaderLine('Authorization'));
        $this->assertSame('agdirlabs-email-php/0.1.0', $request->getHeaderLine('User-Agent'));
        $this->assertSame('application/json', $request->getHeaderLine('Accept'));
        $this->assertSame('application/json', $request->getHeaderLine('Content-Type'));
        $this->assertMatchesRegularExpression(
            '/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/',
            $request->getHeaderLine('Idempotency-Key'),
        );
        $this->assertSame([
            'from' => 'Acme <hello@acme.com>',
            'to' => ['a@example.com'],
            'cc' => 'c@example.com',
            'reply_to' => 'support@acme.com',
            'subject' => 'Hi',
            'html' => '<p>Hi</p>',
            'text' => 'Hi',
            'headers' => ['X-Entity-Ref-ID' => '42'],
            'tags' => [['name' => 'kind', 'value' => 'welcome']],
            'scheduled_at' => '2026-11-01T10:00:00+01:00',
        ], $mock->body(0));
    }

    public function testAcceptsSnakeCaseAliasesAndATagMapAndRejectsUnknownFields(): void
    {
        $mock = new MockClient(MockClient::json(['id' => 'em_1']));
        $mock->client()->emails->send([
            'from' => 'a@b.co', 'to' => 'c@d.co', 'subject' => 's', 'text' => 't',
            'reply_to' => ['r@b.co'], 'scheduled_at' => '2026-11-01T10:00:00Z', 'tags' => ['kind' => 'welcome'],
            'html' => null,
        ]);
        $this->assertSame([
            'from' => 'a@b.co', 'to' => 'c@d.co', 'subject' => 's', 'text' => 't',
            'reply_to' => ['r@b.co'], 'scheduled_at' => '2026-11-01T10:00:00Z',
            'tags' => [['name' => 'kind', 'value' => 'welcome']],
        ], $mock->body(0));

        $this->expectException(\InvalidArgumentException::class);
        $this->expectExceptionMessage('Unknown email field "reply"');
        $mock->client()->emails->send(['from' => 'a@b.co', 'reply' => 'x']);
    }

    public function testUsesTheCallersIdempotencyKeyAndNoneWhenRetriesAreOff(): void
    {
        $mock = new MockClient(MockClient::json(['id' => 'a']), MockClient::json(['id' => 'b']));
        $email = ['from' => 'a@b.co', 'to' => 'c@d.co', 'subject' => 's', 'text' => 't'];
        $mock->client()->emails->send($email, idempotencyKey: 'welcome/42');
        $mock->client(maxRetries: 0)->emails->send($email);
        $this->assertSame('welcome/42', $mock->requests[0]->getHeaderLine('Idempotency-Key'));
        $this->assertFalse($mock->requests[1]->hasHeader('Idempotency-Key'));
    }

    public function testSendsPermissiveBatchesAndReturnsPerIndexErrors(): void
    {
        $mock = new MockClient(MockClient::json([
            'data' => [['id' => 'a']],
            'errors' => [['index' => 1, 'message' => 'Invalid `to` field']],
        ]));
        $email = ['from' => 'a@b.co', 'to' => 'c@d.co', 'subject' => 's', 'text' => 't'];
        $result = $mock->client()->emails->sendBatch([$email, ['to' => 'bad'] + $email], validation: 'permissive');

        $this->assertSame('https://api.test/emails/batch', (string) $mock->requests[0]->getUri());
        $this->assertSame('permissive', $mock->requests[0]->getHeaderLine('x-batch-validation'));
        $this->assertNotSame('', $mock->requests[0]->getHeaderLine('Idempotency-Key'));
        $this->assertCount(2, (array) $mock->body(0));
        $this->assertSame('a', $result->data[0]->id);
        $this->assertSame(1, $result->errors[0]->index);
        $this->assertSame('Invalid `to` field', $result->errors[0]->message);
    }

    public function testStrictBatchesSendNoValidationHeader(): void
    {
        $mock = new MockClient(MockClient::json(['data' => [['id' => 'a']]]));
        $result = $mock->client()->emails->sendBatch([['from' => 'a@b.co', 'to' => 'c@d.co', 'subject' => 's', 'text' => 't']]);
        $this->assertFalse($mock->requests[0]->hasHeader('x-batch-validation'));
        $this->assertSame([], $result->errors);
    }

    public function testMapsAnEmailWithDatesStatusAndEmptyLists(): void
    {
        $mock = new MockClient(MockClient::json([
            'object' => 'email',
            'id' => 'em_1',
            'from' => 'Acme <hello@acme.com>',
            'to' => ['a@example.com'],
            'cc' => null,
            'bcc' => null,
            'reply_to' => null,
            'subject' => 'Hi',
            'html' => '<p>Hi</p>',
            'text' => null,
            'created_at' => '2026-10-30T09:00:00.123Z',
            'scheduled_at' => null,
            'last_event' => 'delivered',
        ]));
        $email = $mock->client()->emails->get('em_1');

        $this->assertSame('GET', $mock->requests[0]->getMethod());
        $this->assertSame('https://api.test/emails/em_1', (string) $mock->requests[0]->getUri());
        $this->assertSame(['a@example.com'], $email->to);
        $this->assertSame([], $email->cc);
        $this->assertSame([], $email->bcc);
        $this->assertSame([], $email->replyTo);
        $this->assertNull($email->text);
        $this->assertSame('<p>Hi</p>', $email->html);
        $this->assertSame(EmailStatus::Delivered, $email->status);
        $this->assertSame('2026-10-30T09:00:00.123+00:00', $email->createdAt->format('Y-m-d\TH:i:s.vP'));
        $this->assertNull($email->scheduledAt);
    }

    public function testCancelsAndEscapesIdsInPaths(): void
    {
        $mock = new MockClient(MockClient::json(['object' => 'email', 'id' => 'a/b']));
        $this->assertSame('a/b', $mock->client()->emails->cancel('a/b')->id);
        $this->assertSame('POST', $mock->requests[0]->getMethod());
        $this->assertSame('https://api.test/emails/a%2Fb/cancel', (string) $mock->requests[0]->getUri());
    }
}
