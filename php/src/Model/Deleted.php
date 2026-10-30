<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

final class Deleted
{
    public function __construct(
        public readonly string $id,
        public readonly bool $deleted = true,
    ) {
    }
}
