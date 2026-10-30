<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

/** An accepted email. */
final class SendResult
{
    public function __construct(public readonly string $id)
    {
    }
}
