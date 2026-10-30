<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

final class Bounce
{
    public function __construct(
        /** e.g. `Permanent` or `Transient`. */
        public readonly string $type,
        public readonly string $subType,
        public readonly string $message,
    ) {
    }
}
