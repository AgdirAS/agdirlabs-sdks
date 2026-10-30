<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

enum DomainStatus: string
{
    case NotStarted = 'not_started';
    case Pending = 'pending';
    case Verified = 'verified';
    case Failed = 'failed';
    case TemporaryFailure = 'temporary_failure';
}
