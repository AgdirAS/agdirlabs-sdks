<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

/** An email of a permissive batch that was not accepted. */
final class BatchError
{
    public function __construct(
        /** Position of the email in the batch you sent (0-based). */
        public readonly int $index,
        public readonly string $message,
    ) {
    }
}
