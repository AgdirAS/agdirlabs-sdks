<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

use AgdirLabs\Email\Internal\Wire;

final class Email
{
    /**
     * @param list<string> $to
     * @param list<string> $cc
     * @param list<string> $bcc
     * @param list<string> $replyTo
     */
    public function __construct(
        public readonly string $id,
        public readonly string $from,
        public readonly array $to,
        public readonly array $cc,
        public readonly array $bcc,
        public readonly array $replyTo,
        public readonly string $subject,
        public readonly ?string $html,
        public readonly ?string $text,
        public readonly \DateTimeImmutable $createdAt,
        public readonly ?\DateTimeImmutable $scheduledAt,
        /** Most advanced delivery state reached so far. */
        public readonly EmailStatus $status,
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
            from: Wire::string($e['from'] ?? null),
            to: Wire::stringList($e['to'] ?? null),
            cc: Wire::stringList($e['cc'] ?? null),
            bcc: Wire::stringList($e['bcc'] ?? null),
            replyTo: Wire::stringList($e['reply_to'] ?? null),
            subject: Wire::string($e['subject'] ?? null),
            html: Wire::nullableString($e['html'] ?? null),
            text: Wire::nullableString($e['text'] ?? null),
            createdAt: Wire::date($e['created_at'] ?? null),
            scheduledAt: Wire::optionalDate($e['scheduled_at'] ?? null),
            status: EmailStatus::from(Wire::string($e['last_event'] ?? null)),
        );
    }
}
