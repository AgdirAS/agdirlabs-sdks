<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

final class Click
{
    public function __construct(
        public readonly string $link,
        public readonly string $ipAddress,
        public readonly string $userAgent,
        public readonly \DateTimeImmutable $timestamp,
    ) {
    }
}
