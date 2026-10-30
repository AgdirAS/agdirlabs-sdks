<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

enum WebhookEventType: string
{
    case Sent = 'email.sent';
    case Scheduled = 'email.scheduled';
    case Delivered = 'email.delivered';
    case DeliveryDelayed = 'email.delivery_delayed';
    case Complained = 'email.complained';
    case Bounced = 'email.bounced';
    case Opened = 'email.opened';
    case Clicked = 'email.clicked';
    case Failed = 'email.failed';
}
