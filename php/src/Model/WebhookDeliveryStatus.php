<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

enum WebhookDeliveryStatus: string
{
    case Pending = 'pending';
    case Attempting = 'attempting';
    case Success = 'success';
    case Failed = 'failed';
}
