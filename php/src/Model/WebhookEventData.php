<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

final class WebhookEventData
{
    /**
     * @param list<string>          $to
     * @param array<string, string> $tags the tags the email was sent with
     */
    public function __construct(
        public readonly string $emailId,
        public readonly string $from,
        public readonly array $to,
        public readonly string $subject,
        public readonly array $tags,
        public readonly \DateTimeImmutable $createdAt,
        public readonly ?string $messageId = null,
        /** Only on `email.bounced`. */
        public readonly ?Bounce $bounce = null,
        /** Only on `email.clicked`. */
        public readonly ?Click $click = null,
        /** Only on `email.failed`. */
        public readonly ?Failure $failed = null,
    ) {
    }
}
