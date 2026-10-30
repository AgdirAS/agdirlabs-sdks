<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

/** Returned when a webhook is created or its secret rotated. Keep `signingSecret` to verify events. */
final class WebhookSecret
{
    public function __construct(
        public readonly string $id,
        /** `whsec_…` */
        public readonly string $signingSecret,
    ) {
    }
}
