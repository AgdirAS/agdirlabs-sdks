<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Tests\Unit;

use AgdirLabs\Email\Model\WebhookDeliveryStatus;
use AgdirLabs\Email\Model\WebhookEventType;
use AgdirLabs\Email\Model\WebhookStatus;
use AgdirLabs\Email\Tests\Support\MockClient;
use PHPUnit\Framework\TestCase;

final class WebhooksTest extends TestCase
{
    public function testCreateAndRotateReturnTheSigningSecret(): void
    {
        $mock = new MockClient(
            MockClient::json(['object' => 'webhook', 'id' => 'wh_1', 'signing_secret' => 'whsec_a']),
            MockClient::json(['object' => 'webhook', 'id' => 'wh_1', 'signing_secret' => 'whsec_b']),
        );
        $client = $mock->client();
        $created = $client->webhooks->create('https://acme.com/hook', [WebhookEventType::Delivered, 'email.bounced']);
        $this->assertSame('wh_1', $created->id);
        $this->assertSame('whsec_a', $created->signingSecret);
        $this->assertSame(['endpoint' => 'https://acme.com/hook', 'events' => ['email.delivered', 'email.bounced']], $mock->body(0));

        $this->assertSame('whsec_b', $client->webhooks->rotateSecret('wh_1')->signingSecret);
        $this->assertSame('/webhooks/wh_1/signing-secret/rotate', $mock->requests[1]->getUri()->getPath());
    }

    public function testGetListUpdateDelete(): void
    {
        $webhook = [
            'object' => 'webhook', 'id' => 'wh_1', 'endpoint' => 'https://acme.com/hook', 'created_at' => '2026-10-30T09:00:00Z',
            'status' => 'enabled', 'events' => ['email.sent'], 'signing_secret' => 'whsec_a',
        ];
        $listed = $webhook;
        unset($listed['signing_secret']);
        $mock = new MockClient(
            MockClient::json($webhook),
            MockClient::json(['object' => 'list', 'has_more' => false, 'data' => [$listed]]),
            MockClient::json(['object' => 'webhook', 'id' => 'wh_1']),
            MockClient::json(['object' => 'webhook', 'id' => 'wh_1', 'deleted' => true]),
        );
        $client = $mock->client();

        $got = $client->webhooks->get('wh_1');
        $this->assertSame('whsec_a', $got->signingSecret);
        $this->assertSame(WebhookStatus::Enabled, $got->status);
        $this->assertSame([WebhookEventType::Sent], $got->events);

        $page = $client->webhooks->list();
        $this->assertNull($page->data[0]->signingSecret);

        $this->assertSame('wh_1', $client->webhooks->update('wh_1', status: WebhookStatus::Disabled)->id);
        $this->assertSame('PATCH', $mock->requests[2]->getMethod());
        $this->assertSame(['status' => 'disabled'], $mock->body(2));

        $this->assertTrue($client->webhooks->delete('wh_1')->deleted);
    }

    public function testDeliveriesAndAttempts(): void
    {
        $mock = new MockClient(
            MockClient::json(['object' => 'list', 'has_more' => false, 'data' => [
                ['id' => 'ev_1', 'type' => 'email.delivered', 'created_at' => '2026-10-30T09:00:00Z', 'status' => 'success'],
            ]]),
            MockClient::json([
                'id' => 'ev_1', 'type' => 'email.delivered', 'created_at' => '2026-10-30T09:00:00Z', 'status' => 'attempting',
                'next_attempt_at' => '2026-10-30T09:05:00Z', 'payload' => ['type' => 'email.delivered'],
            ]),
            MockClient::json(['object' => 'list', 'has_more' => false, 'data' => [
                ['id' => 'at_1', 'http_status_code' => 500, 'response' => 'oops', 'sent_at' => '2026-10-30T09:00:01Z'],
            ]]),
            MockClient::json(['object' => 'webhook_event', 'id' => 'ev_2']),
        );
        $deliveries = $mock->client()->webhooks->deliveries;

        $list = $deliveries->list('wh_1', limit: 10);
        $this->assertSame(WebhookDeliveryStatus::Success, $list->data[0]->status);
        $this->assertSame('/webhooks/wh_1/events', $mock->requests[0]->getUri()->getPath());

        $detail = $deliveries->get('wh_1', 'ev_1');
        $this->assertSame(WebhookEventType::Delivered, $detail->type);
        $this->assertSame('2026-10-30T09:05:00+00:00', $detail->nextAttemptAt?->format(DATE_ATOM));
        $this->assertSame(['type' => 'email.delivered'], $detail->payload);

        $attempts = $deliveries->attempts('wh_1', 'ev_1');
        $this->assertSame(500, $attempts->data[0]->httpStatus);
        $this->assertSame('oops', $attempts->data[0]->response);
        $this->assertSame('/webhooks/wh_1/events/ev_1/attempts', $mock->requests[2]->getUri()->getPath());

        $this->assertSame('ev_2', $deliveries->replay('wh_1', 'ev_1')->id);
        $this->assertSame('/webhooks/wh_1/events/ev_1/replay', $mock->requests[3]->getUri()->getPath());
    }
}
