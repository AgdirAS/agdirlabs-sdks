<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

use AgdirLabs\Email\Internal\Wire;

/** A webhook endpoint. `signingSecret` is only returned by `get()` (null on list pages). */
final class Webhook
{
    /**
     * @param list<WebhookEventType> $events
     */
    public function __construct(
        public readonly string $id,
        public readonly string $endpoint,
        public readonly WebhookStatus $status,
        public readonly array $events,
        public readonly \DateTimeImmutable $createdAt,
        /** `whsec_…` — pass it to `Webhook::verify()`. */
        public readonly ?string $signingSecret = null,
    ) {
    }

    /**
     * @internal
     *
     * @param array<mixed> $w
     */
    public static function fromWire(array $w): self
    {
        return new self(
            id: Wire::string($w['id'] ?? null),
            endpoint: Wire::string($w['endpoint'] ?? null),
            status: WebhookStatus::from(Wire::string($w['status'] ?? null)),
            events: array_values(array_map(
                static fn (string $e) => WebhookEventType::from($e),
                Wire::stringList($w['events'] ?? null),
            )),
            createdAt: Wire::date($w['created_at'] ?? null),
            signingSecret: Wire::nullableString($w['signing_secret'] ?? null),
        );
    }
}
