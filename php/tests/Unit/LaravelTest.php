<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Tests\Unit;

use AgdirLabs\Email\AgdirEmail;
use AgdirLabs\Email\Laravel\AgdirEmailServiceProvider;
use AgdirLabs\Email\Laravel\Facades\AgdirEmail as AgdirEmailFacade;
use AgdirLabs\Email\Mailer\AgdirTransport;
use AgdirLabs\Email\Resource\Emails;
use AgdirLabs\Email\Tests\Support\MockClient;
use Illuminate\Mail\Mailable;
use Illuminate\Mail\Mailables\Content;
use Illuminate\Mail\Mailables\Envelope;
use Illuminate\Support\Facades\Mail;
use Orchestra\Testbench\TestCase;

final class LaravelTest extends TestCase
{
    /** @return list<class-string> */
    protected function getPackageProviders($app): array
    {
        return [AgdirEmailServiceProvider::class];
    }

    /** @return array<string, class-string> */
    protected function getPackageAliases($app): array
    {
        return ['AgdirEmail' => AgdirEmailFacade::class];
    }

    protected function defineEnvironment($app): void
    {
        $app['config']->set('agdir-email.api_key', 're_laravel');
        $app['config']->set('agdir-email.base_url', 'https://laravel.test');
        $app['config']->set('mail.default', 'agdir');
        $app['config']->set('mail.from', ['address' => 'hello@acme.com', 'name' => 'Acme']);
    }

    private static function transport(string $mailer): \Symfony\Component\Mailer\Transport\TransportInterface
    {
        $instance = Mail::mailer($mailer);
        self::assertInstanceOf(\Illuminate\Mail\Mailer::class, $instance);

        return $instance->getSymfonyTransport();
    }

    public function testRegistersTheClientFromConfigAndTheFacade(): void
    {
        $client = app(AgdirEmail::class);
        $this->assertSame('https://laravel.test', $client->baseUrl);
        $this->assertSame($client, app('agdir-email'));
        $this->assertSame($client, AgdirEmailFacade::getFacadeRoot());
        $this->assertInstanceOf(Emails::class, AgdirEmailFacade::emails());
        $this->assertSame($client->domains, AgdirEmailFacade::domains());
    }

    public function testMergesDefaultConfigAndAddsTheAgdirMailer(): void
    {
        $this->assertSame(['transport' => 'agdir'], config('mail.mailers.agdir'));
        $this->assertSame(2, config('agdir-email.max_retries'));
        $this->assertEquals(30, config('agdir-email.timeout'));
    }

    public function testTheAgdirMailerSendsMailablesThroughTheApi(): void
    {
        $mock = new MockClient(MockClient::json(['id' => 'em_laravel']));
        app()->instance(AgdirEmail::class, $mock->client());

        $this->assertInstanceOf(AgdirTransport::class, self::transport('agdir'));

        $sent = Mail::to('user@example.com')->send(new class () extends Mailable {
            public function envelope(): Envelope
            {
                return new Envelope(subject: 'Welcome', tags: ['onboarding'], metadata: ['user_id' => '42']);
            }

            public function content(): Content
            {
                return new Content(htmlString: '<p>Welcome aboard</p>');
            }
        });

        $this->assertNotNull($sent);
        $this->assertSame('em_laravel', $sent->getMessageId());
        $body = $mock->body(0);
        $this->assertSame('Acme <hello@acme.com>', $body['from'] ?? null);
        $this->assertSame(['user@example.com'], $body['to'] ?? null);
        $this->assertSame('Welcome', $body['subject'] ?? null);
        $this->assertSame('<p>Welcome aboard</p>', $body['html'] ?? null);
        $this->assertSame([['name' => 'onboarding', 'value' => 'true'], ['name' => 'user_id', 'value' => '42']], $body['tags'] ?? null);
    }

    public function testAMailerCanUseItsOwnKey(): void
    {
        config()->set('mail.mailers.other', ['transport' => 'agdir', 'key' => 're_other', 'base_url' => 'https://other.test']);
        $transport = self::transport('other');
        $this->assertSame('agdir+api://other.test', (string) $transport);
    }
}
