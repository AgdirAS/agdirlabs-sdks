<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Tests\Unit;

use AgdirLabs\Email\AgdirEmail;
use AgdirLabs\Email\Tests\Support\MockClient;
use PHPUnit\Framework\TestCase;

final class ClientTest extends TestCase
{
    private const VARS = ['AGDIR_EMAIL_API_KEY', 'AGDIR_EMAIL_BASE_URL'];

    protected function tearDown(): void
    {
        foreach (self::VARS as $var) {
            putenv($var);
            unset($_ENV[$var], $_SERVER[$var]);
        }
    }

    public function testUsesApiAgdirEmailByDefault(): void
    {
        $this->assertSame('https://api.agdir.email', (new AgdirEmail('re_x', httpClient: new MockClient()))->baseUrl);
        $this->assertSame('agdirlabs-email-php/0.1.0', AgdirEmail::USER_AGENT);
    }

    public function testReadsKeyAndBaseUrlFromTheEnvironmentAndEmptyValuesCountAsUnset(): void
    {
        putenv('AGDIR_EMAIL_API_KEY=re_env');
        putenv('AGDIR_EMAIL_BASE_URL=https://env.test');
        $mock = new MockClient(MockClient::json(['id' => 'e1']));
        $client = new AgdirEmail(httpClient: $mock, maxRetries: 0);
        $this->assertSame('https://env.test', $client->baseUrl);
        $client->emails->send(['from' => 'a@b.co', 'to' => 'c@d.co', 'subject' => 's', 'text' => 't']);
        $this->assertSame('Bearer re_env', $mock->requests[0]->getHeaderLine('Authorization'));

        putenv('AGDIR_EMAIL_BASE_URL=');
        $this->assertSame('https://api.agdir.email', (new AgdirEmail(httpClient: $mock))->baseUrl);
        $this->assertSame('https://arg.test', (new AgdirEmail(baseUrl: 'https://arg.test', httpClient: $mock))->baseUrl);

        putenv('AGDIR_EMAIL_API_KEY=');
        try {
            new AgdirEmail(httpClient: $mock);
            $this->fail('expected a missing key error');
        } catch (\InvalidArgumentException $e) {
            $this->assertStringContainsString('AGDIR_EMAIL_API_KEY', $e->getMessage());
            $this->assertStringContainsString('https://app.agdir.email', $e->getMessage());
        }
    }

    public function testRejectsInvalidOptions(): void
    {
        $this->expectException(\InvalidArgumentException::class);
        new AgdirEmail('re_x', maxRetries: -1, httpClient: new MockClient());
    }

    public function testDiscoversAnHttpClientWhenNoneIsGiven(): void
    {
        $client = new AgdirEmail('re_x');
        $this->assertInstanceOf(AgdirEmail::class, $client);
    }

    public function testVersionMatchesTheChangelog(): void
    {
        $changelog = (string) file_get_contents(__DIR__ . '/../../CHANGELOG.md');
        $this->assertSame(1, preg_match('/^## \[(\d+\.\d+\.\d+)\]/m', $changelog, $m));
        $this->assertSame($m[1] ?? null, AgdirEmail::VERSION);
    }
}
