<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

final class SendBatchResult
{
    /**
     * @param list<SendResult> $data   accepted emails, in order
     * @param list<BatchError> $errors only with permissive validation: emails that were not accepted
     */
    public function __construct(
        public readonly array $data,
        public readonly array $errors = [],
    ) {
    }
}
