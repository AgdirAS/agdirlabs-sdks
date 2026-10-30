<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Tests\Unit;

use AgdirLabs\Email\AgdirEmail;
use AgdirLabs\Email\Mailer\AgdirTransport;
use AgdirLabs\Email\Mailer\AgdirTransportFactory;
use AgdirLabs\Email\Mailer\UnsupportedAttachmentException;
use AgdirLabs\Email\Tests\Support\MockClient;
use GuzzleHttp\Psr7\Response;
use PHPUnit\Framework\TestCase;
use Symfony\Component\Mailer\Envelope;
use Symfony\Component\Mailer\Exception\IncompleteDsnException;
use Symfony\Component\Mailer\Exception\TransportException;
use Symfony\Component\Mailer\Exception\UnsupportedSchemeException;
use Symfony\Component\Mailer\Header\MetadataHeader;
use Symfony\Component\Mailer\Header\TagHeader;
use Symfony\Component\Mailer\Transport;
use Symfony\Component\Mailer\Transport\Dsn;
use Symfony\Component\Mime\Address;
use Symfony\Component\Mime\Email;

final class MailerTransportTest extends TestCase
{
    private static function email(): Email
    {
        $email = (new Email())
            ->from(new Address('hello@acme.com', 'Acme, Inc.'))
            ->to('a@example.com', new Address('b@example.com', 'Ben'))
            ->cc('c@example.com')
            ->bcc('d@example.com')
            ->replyTo('support@acme.com')
            ->subject('Welcome')
            ->html('<p>Hi</p>')
            ->text('Hi');
        $email->getHeaders()
            ->addTextHeader('X-Entity-Ref-ID', 'inv-1042')
            ->addTextHeader('X-Agdir-Tag-kind', 'welcome')
            ->add(new TagHeader('onboarding'))
            ->add(new MetadataHeader('user_id', '42'));

        return $email;
    }

    public function testMapsASymfonyEmailToTheApiPayload(): void
    {
        $this->assertSame([
            'from' => '"Acme, Inc." <hello@acme.com>',
            'to' => ['a@example.com', 'Ben <b@example.com>'],
            'subject' => 'Welcome',
            'cc' => ['c@example.com'],
            'bcc' => ['d@example.com'],
            'replyTo' => ['support@acme.com'],
            'html' => '<p>Hi</p>',
            'text' => 'Hi',
            'headers' => ['X-Entity-Ref-ID' => 'inv-1042'],
            'tags' => ['kind' => 'welcome', 'onboarding' => 'true', 'user_id' => '42'],
        ], AgdirTransport::payload(self::email()));
    }

    public function testSendsThroughTheApiAndExposesTheEmailId(): void
    {
        $mock = new MockClient(MockClient::json(['id' => 'em_1']));
        $transport = new AgdirTransport($mock->client());
        $email = self::email();
        $sent = $transport->send($email);

        $this->assertNotNull($sent);
        $this->assertSame('em_1', $sent->getMessageId());
        $body = $mock->body(0);
        $this->assertSame(['a@example.com', 'Ben <b@example.com>'], $body['to'] ?? null);
        $this->assertSame(['support@acme.com'], $body['reply_to'] ?? null);
        $this->assertSame([['name' => 'kind', 'value' => 'welcome'], ['name' => 'onboarding', 'value' => 'true'], ['name' => 'user_id', 'value' => '42']], $body['tags'] ?? null);
        $this->assertNotSame('', $mock->requests[0]->getHeaderLine('Idempotency-Key'));
        $this->assertSame('agdir+api://api.test', (string) $transport);
    }

    public function testUsesEnvelopeRecipientsForTo(): void
    {
        $envelope = new Envelope(new Address('bounce@acme.com'), [new Address('dev@acme.com'), new Address('c@example.com')]);
        $payload = AgdirTransport::payload(self::email(), $envelope);
        $this->assertSame(['dev@acme.com'], $payload['to']);
        $this->assertSame(['c@example.com'], $payload['cc']);
    }

    public function testRejectsAttachmentsClearly(): void
    {
        $mock = new MockClient();
        $email = self::email()->attach('data', 'report.csv', 'text/csv');
        try {
            (new AgdirTransport($mock->client()))->send($email);
            $this->fail('expected an attachment error');
        } catch (UnsupportedAttachmentException $e) {
            $this->assertStringContainsString('attachments', $e->getMessage());
        }
        $this->assertSame(0, $mock->count());
    }

    public function testWrapsApiErrorsInATransportException(): void
    {
        $mock = new MockClient(new Response(403, ['Content-Type' => 'application/json'], (string) json_encode([
            'statusCode' => 403, 'name' => 'validation_error', 'message' => 'The acme.com domain is not verified.',
        ])));
        try {
            (new AgdirTransport($mock->client()))->send(self::email());
            $this->fail('expected a transport error');
        } catch (TransportException $e) {
            $this->assertStringContainsString('validation_error', $e->getMessage());
            $this->assertStringContainsString('not verified', $e->getMessage());
            $this->assertSame(403, $e->getCode());
        }
    }

    public function testTheFactoryBuildsTransportsFromDsns(): void
    {
        $factory = new AgdirTransportFactory();
        $this->assertTrue($factory->supports(Dsn::fromString('agdir+api://re_key@default')));
        $this->assertTrue($factory->supports(Dsn::fromString('agdir://re_key@default')));
        $this->assertFalse($factory->supports(Dsn::fromString('smtp://localhost')));

        $this->assertSame('agdir+api://api.agdir.email', (string) $factory->create(Dsn::fromString('agdir+api://re_key@default')));
        $this->assertSame('agdir+api://eu.example.com', (string) $factory->create(Dsn::fromString('agdir+api://re_key@eu.example.com:8443?max_retries=0')));

        $transport = (new Transport([new AgdirTransportFactory()]))->fromString('agdir+api://re_key@default');
        $this->assertSame('agdir+api://api.agdir.email', (string) $transport);

        $this->expectException(UnsupportedSchemeException::class);
        $factory->create(Dsn::fromString('agdir+smtp://re_key@default'));
    }

    public function testTheFactoryNeedsAKey(): void
    {
        $this->expectException(IncompleteDsnException::class);
        (new AgdirTransportFactory())->create(Dsn::fromString('agdir+api://default'));
    }

    public function testDefaultBaseUrlConstant(): void
    {
        $this->assertSame('https://api.agdir.email', AgdirEmail::DEFAULT_BASE_URL);
    }
}
