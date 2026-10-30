<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Exception;

/** Thrown by `Webhook::verify()` when a request's signature, timestamp or payload is invalid. */
final class WebhookVerificationException extends \RuntimeException
{
}
