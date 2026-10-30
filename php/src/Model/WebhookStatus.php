<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

enum WebhookStatus: string
{
    case Enabled = 'enabled';
    case Disabled = 'disabled';
}
