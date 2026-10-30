<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Tests\Unit;

use AgdirLabs\Email\Model\DomainStatus;
use AgdirLabs\Email\Tests\Support\MockClient;
use PHPUnit\Framework\TestCase;

final class DomainsTest extends TestCase
{
    /** @return array<string, mixed> */
    public static function domain(string $id = 'd_1', string $name = 'acme.com'): array
    {
        return [
            'object' => 'domain',
            'id' => $id,
            'name' => $name,
            'status' => 'pending',
            'created_at' => '2026-10-30T09:00:00Z',
            'region' => 'eu-north-1',
            'capabilities' => ['sending' => 'enabled', 'receiving' => 'disabled'],
            'open_tracking' => false,
            'click_tracking' => false,
            'records' => [
                ['record' => 'DKIM', 'name' => 'agdir._domainkey', 'type' => 'TXT', 'ttl' => 'Auto', 'status' => 'pending', 'value' => 'p=MIG'],
                ['record' => 'SPF', 'name' => 'send', 'type' => 'MX', 'ttl' => 'Auto', 'status' => 'pending', 'value' => 'feedback-smtp.eu-north-1.amazonses.com', 'priority' => 10],
            ],
        ];
    }

    public function testCreatesAndMapsDomainsWithDnsRecords(): void
    {
        $mock = new MockClient(MockClient::json(self::domain()));
        $domain = $mock->client()->domains->create('acme.com');

        $this->assertSame(['name' => 'acme.com'], $mock->body(0));
        $this->assertSame('POST', $mock->requests[0]->getMethod());
        $this->assertFalse($mock->requests[0]->hasHeader('Idempotency-Key'));
        $this->assertSame('d_1', $domain->id);
        $this->assertSame(DomainStatus::Pending, $domain->status);
        $this->assertSame('eu-north-1', $domain->region);
        $this->assertFalse($domain->openTracking);
        $this->assertCount(2, $domain->records);
        $this->assertSame('DKIM', $domain->records[0]->purpose);
        $this->assertSame('agdir._domainkey', $domain->records[0]->name);
        $this->assertNull($domain->records[0]->priority);
        $this->assertSame('MX', $domain->records[1]->type);
        $this->assertSame(10, $domain->records[1]->priority);
        $this->assertSame(DomainStatus::Pending, $domain->records[1]->status);
    }

    public function testGetVerifyDelete(): void
    {
        $mock = new MockClient(
            MockClient::json(self::domain()),
            MockClient::json(['object' => 'domain', 'id' => 'd_1']),
            MockClient::json(['object' => 'domain', 'id' => 'd_1', 'deleted' => true]),
        );
        $client = $mock->client();
        $this->assertSame('acme.com', $client->domains->get('d_1')->name);
        $this->assertSame('d_1', $client->domains->verify('d_1')->id);
        $deleted = $client->domains->delete('d_1');
        $this->assertTrue($deleted->deleted);
        $this->assertSame('d_1', $deleted->id);

        $this->assertSame(['GET', 'POST', 'DELETE'], array_map(fn ($r) => $r->getMethod(), $mock->requests));
        $this->assertSame('/domains/d_1/verify', $mock->requests[1]->getUri()->getPath());
    }

    public function testReturnsPagesWithACursorAndWalksAllOfThemWithListAll(): void
    {
        $mock = new MockClient(
            MockClient::json(['object' => 'list', 'has_more' => true, 'data' => [self::domain('d_1'), self::domain('d_2', 'b.com')]]),
            MockClient::json(['object' => 'list', 'has_more' => true, 'data' => [self::domain('d_1'), self::domain('d_2', 'b.com')]]),
            MockClient::json(['object' => 'list', 'has_more' => false, 'data' => [self::domain('d_3', 'c.com')]]),
        );
        $client = $mock->client();

        $page = $client->domains->list(limit: 2);
        $this->assertTrue($page->hasMore);
        $this->assertSame('d_2', $page->nextCursor);
        $this->assertSame('https://api.test/domains?limit=2', (string) $mock->requests[0]->getUri());

        $names = [];
        foreach ($client->domains->listAll(limit: 2) as $domain) {
            $names[] = $domain->name;
        }
        $this->assertSame(['acme.com', 'b.com', 'c.com'], $names);
        $this->assertSame('limit=2', $mock->requests[1]->getUri()->getQuery());
        $this->assertSame('limit=2&after=d_2', $mock->requests[2]->getUri()->getQuery());
    }

    public function testTheLastPageHasNoCursor(): void
    {
        $mock = new MockClient(MockClient::json(['object' => 'list', 'has_more' => false, 'data' => []]));
        $page = $mock->client()->domains->list();
        $this->assertSame([], $page->data);
        $this->assertNull($page->nextCursor);
        $this->assertSame('', $mock->requests[0]->getUri()->getQuery());
    }

    public function testListAllIsLazy(): void
    {
        $mock = new MockClient();
        $mock->client()->domains->listAll();
        $this->assertSame(0, $mock->count());
    }
}
