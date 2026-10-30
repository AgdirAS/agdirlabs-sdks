<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

/** The id of the object an action was applied to (cancel, verify, update, replay). */
final class Reference
{
    public function __construct(public readonly string $id)
    {
    }
}
