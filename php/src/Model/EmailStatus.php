<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

/** The most advanced delivery state an email has reached. */
enum EmailStatus: string
{
    case Queued = 'queued';
    case Scheduled = 'scheduled';
    case Sent = 'sent';
    case DeliveryDelayed = 'delivery_delayed';
    case Delivered = 'delivered';
    case Opened = 'opened';
    case Clicked = 'clicked';
    case Bounced = 'bounced';
    case Failed = 'failed';
    case Complained = 'complained';
    case Canceled = 'canceled';
}
