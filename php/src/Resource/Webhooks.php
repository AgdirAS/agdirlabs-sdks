<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Resource;

use AgdirLabs\Email\Exception\AgdirEmailException;
use AgdirLabs\Email\Internal\Paginator;
use AgdirLabs\Email\Internal\Transport;
use AgdirLabs\Email\Internal\Wire;
use AgdirLabs\Email\Model\Deleted;
use AgdirLabs\Email\Model\Page;
use AgdirLabs\Email\Model\Reference;
use AgdirLabs\Email\Model\Webhook;
use AgdirLabs\Email\Model\WebhookEventType;
use AgdirLabs\Email\Model\WebhookSecret;
use AgdirLabs\Email\Model\WebhookStatus;

final class Webhooks
{
    /** Messages sent (or queued) to an endpoint, with their HTTP attempts. */
    public readonly WebhookDeliveries $deliveries;

    /** @internal */
    public function __construct(private readonly Transport $transport)
    {
        $this->deliveries = new WebhookDeliveries($transport);
    }

    /**
     * Registers an endpoint. Keep the returned `signingSecret` to verify incoming events.
     *
     * @param string                                $endpoint public `https://` URL
     * @param list<WebhookEventType|string> $events e.g. `[WebhookEventType::Delivered, 'email.bounced']`
     *
     * @throws AgdirEmailException
     */
    public function create(string $endpoint, array $events): WebhookSecret
    {
        $res = $this->transport->request('POST', '/webhooks', retryable: false, body: [
            'endpoint' => $endpoint,
            'events' => self::events($events),
        ]);

        return new WebhookSecret(Wire::string($res['id'] ?? null), Wire::string($res['signing_secret'] ?? null));
    }

    /** The webhook, including its `signingSecret`. @throws AgdirEmailException */
    public function get(string $id): Webhook
    {
        return Webhook::fromWire($this->transport->request('GET', '/webhooks/' . rawurlencode($id), retryable: true));
    }

    /**
     * @return Page<Webhook>
     *
     * @throws AgdirEmailException
     */
    public function list(?int $limit = null, ?string $after = null): Page
    {
        $res = $this->transport->request('GET', '/webhooks', retryable: true, query: ['limit' => $limit, 'after' => $after]);

        return Page::fromWire($res, Webhook::fromWire(...));
    }

    /**
     * Every webhook, fetching further pages as you iterate.
     *
     * @return \Generator<int, Webhook, mixed, void>
     *
     * @throws AgdirEmailException
     */
    public function listAll(?int $limit = null, ?string $after = null): \Generator
    {
        return Paginator::walk(fn (?string $cursor) => $this->list($limit, $cursor), $after);
    }

    /**
     * Changes the endpoint, the subscribed events or the status. Omitted arguments stay as they are.
     *
     * @param list<WebhookEventType|string>|null $events
     *
     * @throws AgdirEmailException
     */
    public function update(
        string $id,
        ?string $endpoint = null,
        ?array $events = null,
        WebhookStatus|string|null $status = null,
    ): Reference {
        $body = [];
        if ($endpoint !== null) {
            $body['endpoint'] = $endpoint;
        }
        if ($events !== null) {
            $body['events'] = self::events($events);
        }
        if ($status !== null) {
            $body['status'] = $status instanceof WebhookStatus ? $status->value : $status;
        }
        $res = $this->transport->request(
            'PATCH',
            '/webhooks/' . rawurlencode($id),
            retryable: true,
            body: $body === [] ? new \stdClass() : $body,
        );

        return new Reference(Wire::string($res['id'] ?? null));
    }

    /** @throws AgdirEmailException */
    public function delete(string $id): Deleted
    {
        $res = $this->transport->request('DELETE', '/webhooks/' . rawurlencode($id), retryable: false);

        return new Deleted(Wire::string($res['id'] ?? null));
    }

    /**
     * Issues a new signing secret; the old one stops working immediately.
     *
     * @throws AgdirEmailException
     */
    public function rotateSecret(string $id): WebhookSecret
    {
        $res = $this->transport->request('POST', '/webhooks/' . rawurlencode($id) . '/signing-secret/rotate', retryable: false);

        return new WebhookSecret(Wire::string($res['id'] ?? null), Wire::string($res['signing_secret'] ?? null));
    }

    /**
     * @param list<WebhookEventType|string> $events
     *
     * @return list<string>
     */
    private static function events(array $events): array
    {
        return array_values(array_map(
            static fn (WebhookEventType|string $e) => $e instanceof WebhookEventType ? $e->value : $e,
            $events,
        ));
    }
}
