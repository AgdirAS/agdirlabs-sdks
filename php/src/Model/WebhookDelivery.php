<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

use AgdirLabs\Email\Internal\Wire;

/**
 * One message sent (or queued) to a webhook endpoint.
 * `nextAttemptAt` and `payload` are only filled by `deliveries->get()`.
 */
final class WebhookDelivery
{
    public function __construct(
        public readonly string $id,
        public readonly WebhookEventType $type,
        public readonly WebhookDeliveryStatus $status,
        public readonly \DateTimeImmutable $createdAt,
        public readonly ?\DateTimeImmutable $nextAttemptAt = null,
        /** Exact JSON body sent to your endpoint, decoded. */
        public readonly mixed $payload = null,
    ) {
    }

    /**
     * @internal
     *
     * @param array<mixed> $e
     */
    public static function fromWire(array $e): self
    {
        return new self(
            id: Wire::string($e['id'] ?? null),
            type: WebhookEventType::from(Wire::string($e['type'] ?? null)),
            status: WebhookDeliveryStatus::from(Wire::string($e['status'] ?? null)),
            createdAt: Wire::date($e['created_at'] ?? null),
            nextAttemptAt: Wire::optionalDate($e['next_attempt_at'] ?? null),
            payload: $e['payload'] ?? null,
        );
    }
}
