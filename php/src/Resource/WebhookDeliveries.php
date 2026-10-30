<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Resource;

use AgdirLabs\Email\Exception\AgdirEmailException;
use AgdirLabs\Email\Internal\Paginator;
use AgdirLabs\Email\Internal\Transport;
use AgdirLabs\Email\Internal\Wire;
use AgdirLabs\Email\Model\Page;
use AgdirLabs\Email\Model\Reference;
use AgdirLabs\Email\Model\WebhookAttempt;
use AgdirLabs\Email\Model\WebhookDelivery;

/** Messages sent (or queued) to one webhook endpoint, with their HTTP attempts. */
final class WebhookDeliveries
{
    /** @internal */
    public function __construct(private readonly Transport $transport)
    {
    }

    /**
     * @return Page<WebhookDelivery>
     *
     * @throws AgdirEmailException
     */
    public function list(string $webhookId, ?int $limit = null, ?string $after = null): Page
    {
        $res = $this->transport->request(
            'GET',
            '/webhooks/' . rawurlencode($webhookId) . '/events',
            retryable: true,
            query: ['limit' => $limit, 'after' => $after],
        );

        return Page::fromWire($res, WebhookDelivery::fromWire(...));
    }

    /**
     * @return \Generator<int, WebhookDelivery, mixed, void>
     *
     * @throws AgdirEmailException
     */
    public function listAll(string $webhookId, ?int $limit = null, ?string $after = null): \Generator
    {
        return Paginator::walk(fn (?string $cursor) => $this->list($webhookId, $limit, $cursor), $after);
    }

    /**
     * One delivery, with `payload` (the exact body sent) and `nextAttemptAt`.
     *
     * @throws AgdirEmailException
     */
    public function get(string $webhookId, string $deliveryId): WebhookDelivery
    {
        return WebhookDelivery::fromWire($this->transport->request(
            'GET',
            '/webhooks/' . rawurlencode($webhookId) . '/events/' . rawurlencode($deliveryId),
            retryable: true,
        ));
    }

    /**
     * HTTP attempts for one delivery, newest first.
     *
     * @return Page<WebhookAttempt>
     *
     * @throws AgdirEmailException
     */
    public function attempts(string $webhookId, string $deliveryId, ?int $limit = null, ?string $after = null): Page
    {
        $res = $this->transport->request(
            'GET',
            '/webhooks/' . rawurlencode($webhookId) . '/events/' . rawurlencode($deliveryId) . '/attempts',
            retryable: true,
            query: ['limit' => $limit, 'after' => $after],
        );

        return Page::fromWire($res, WebhookAttempt::fromWire(...));
    }

    /**
     * Sends the identical message again, once, right now (also to disabled endpoints).
     *
     * @throws AgdirEmailException
     */
    public function replay(string $webhookId, string $deliveryId): Reference
    {
        $res = $this->transport->request(
            'POST',
            '/webhooks/' . rawurlencode($webhookId) . '/events/' . rawurlencode($deliveryId) . '/replay',
            retryable: false,
        );

        return new Reference(Wire::string($res['id'] ?? null));
    }
}
