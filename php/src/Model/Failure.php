<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

final class Failure
{
    public function __construct(public readonly string $reason)
    {
    }
}
