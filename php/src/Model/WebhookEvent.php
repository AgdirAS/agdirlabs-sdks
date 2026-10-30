<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

use AgdirLabs\Email\Internal\Wire;

/** An event received on your webhook endpoint, as returned by `Webhook::verify()`. */
final class WebhookEvent
{
    public function __construct(
        public readonly WebhookEventType $type,
        public readonly \DateTimeImmutable $createdAt,
        public readonly WebhookEventData $data,
    ) {
    }

    /**
     * @internal
     *
     * @param array<mixed> $body
     */
    public static function fromWire(array $body): self
    {
        $d = Wire::array($body['data'] ?? null);
        $bounce = isset($d['bounce']) && is_array($d['bounce']) ? $d['bounce'] : null;
        $click = isset($d['click']) && is_array($d['click']) ? $d['click'] : null;
        $failed = isset($d['failed']) && is_array($d['failed']) ? $d['failed'] : null;
        $tags = [];
        foreach (Wire::array($d['tags'] ?? null) as $name => $value) {
            $tags[(string) $name] = Wire::string($value);
        }

        return new self(
            type: WebhookEventType::from(Wire::string($body['type'] ?? null)),
            createdAt: Wire::date($body['created_at'] ?? null),
            data: new WebhookEventData(
                emailId: Wire::string($d['email_id'] ?? null),
                from: Wire::string($d['from'] ?? null),
                to: Wire::stringList($d['to'] ?? null),
                subject: Wire::string($d['subject'] ?? null),
                tags: $tags,
                createdAt: Wire::date($d['created_at'] ?? null),
                messageId: isset($d['message_id']) && $d['message_id'] !== '' ? Wire::string($d['message_id']) : null,
                bounce: $bounce === null ? null : new Bounce(
                    type: Wire::string($bounce['type'] ?? null),
                    subType: Wire::string($bounce['subType'] ?? null),
                    message: Wire::string($bounce['message'] ?? null),
                ),
                click: $click === null ? null : new Click(
                    link: Wire::string($click['link'] ?? null),
                    ipAddress: Wire::string($click['ipAddress'] ?? null),
                    userAgent: Wire::string($click['userAgent'] ?? null),
                    timestamp: Wire::date($click['timestamp'] ?? null),
                ),
                failed: $failed === null ? null : new Failure(reason: Wire::string($failed['reason'] ?? null)),
            ),
        );
    }
}
