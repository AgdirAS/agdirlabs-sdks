<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

use AgdirLabs\Email\Internal\Wire;

/** One HTTP attempt of a webhook delivery. */
final class WebhookAttempt
{
    public function __construct(
        public readonly string $id,
        /** 0 when no HTTP response was received. */
        public readonly int $httpStatus,
        public readonly string $response,
        public readonly \DateTimeImmutable $sentAt,
    ) {
    }

    /**
     * @internal
     *
     * @param array<mixed> $a
     */
    public static function fromWire(array $a): self
    {
        $status = $a['http_status_code'] ?? 0;

        return new self(
            id: Wire::string($a['id'] ?? null),
            httpStatus: is_numeric($status) ? (int) $status : 0,
            response: Wire::string($a['response'] ?? null),
            sentAt: Wire::date($a['sent_at'] ?? null),
        );
    }
}
