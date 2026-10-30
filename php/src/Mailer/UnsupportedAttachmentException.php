<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Mailer;

use Symfony\Component\Mailer\Exception\LogicException;

/** The Agdir Email API does not accept attachments yet. */
final class UnsupportedAttachmentException extends LogicException
{
}
