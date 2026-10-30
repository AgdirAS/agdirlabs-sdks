<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Resource;

use AgdirLabs\Email\Exception\AgdirEmailException;
use AgdirLabs\Email\Internal\EmailInput;
use AgdirLabs\Email\Internal\Transport;
use AgdirLabs\Email\Internal\Wire;
use AgdirLabs\Email\Model\BatchError;
use AgdirLabs\Email\Model\Email;
use AgdirLabs\Email\Model\Reference;
use AgdirLabs\Email\Model\SendBatchResult;
use AgdirLabs\Email\Model\SendResult;

/**
 * An email to send is an array:
 *
 * ```php
 * [
 *     'from' => 'Acme <hello@acme.com>',           // required
 *     'to' => 'user@example.com',                  // required; string or list
 *     'subject' => 'Welcome',                      // required
 *     'html' => '<p>Hi</p>', 'text' => 'Hi',       // at least one
 *     'cc' => [...], 'bcc' => [...], 'replyTo' => ..., // string or list
 *     'headers' => ['X-Entity-Ref-ID' => '123'],
 *     'tags' => [['name' => 'kind', 'value' => 'welcome']], // or ['kind' => 'welcome']
 *     'scheduledAt' => new DateTimeImmutable('+1 hour'),   // or an ISO 8601 string with offset
 * ]
 * ```
 *
 * `reply_to` and `scheduled_at` are accepted as aliases.
 *
 * @phpstan-type SendEmail array<string, mixed>
 */
final class Emails
{
    public const MAX_BATCH_SIZE = 100;

    /** @internal */
    public function __construct(private readonly Transport $transport)
    {
    }

    /**
     * Accepts one email for delivery (or for `scheduledAt`).
     *
     * @param array<string, mixed> $email
     * @param string|null          $idempotencyKey makes the send safe to retry: the same key with the same body
     *                                             returns the original result. Generated when retries are enabled.
     *
     * @throws AgdirEmailException
     */
    public function send(array $email, ?string $idempotencyKey = null): SendResult
    {
        $key = $this->idempotencyKey($idempotencyKey);
        $res = $this->transport->request(
            'POST',
            '/emails',
            retryable: $key !== null,
            body: EmailInput::toWire($email),
            headers: ['Idempotency-Key' => $key],
        );

        return new SendResult(Wire::string($res['id'] ?? null));
    }

    /**
     * Accepts up to 100 emails in one request.
     *
     * @param list<array<string, mixed>> $emails
     * @param string                     $validation `strict` (default): one invalid email rejects the whole batch.
     *                                               `permissive`: valid emails are accepted, invalid ones are
     *                                               reported in `errors`.
     *
     * @throws AgdirEmailException
     */
    public function sendBatch(array $emails, ?string $idempotencyKey = null, string $validation = 'strict'): SendBatchResult
    {
        if ($validation !== 'strict' && $validation !== 'permissive') {
            throw new \InvalidArgumentException('$validation must be "strict" or "permissive".');
        }
        $key = $this->idempotencyKey($idempotencyKey);
        $res = $this->transport->request(
            'POST',
            '/emails/batch',
            retryable: $key !== null,
            body: array_map(EmailInput::toWire(...), array_values($emails)),
            headers: [
                'Idempotency-Key' => $key,
                'x-batch-validation' => $validation === 'permissive' ? 'permissive' : null,
            ],
        );

        $data = [];
        foreach (Wire::array($res['data'] ?? null) as $item) {
            $data[] = new SendResult(Wire::string(Wire::array($item)['id'] ?? null));
        }
        $errors = [];
        foreach (Wire::array($res['errors'] ?? null) as $error) {
            $error = Wire::array($error);
            $index = $error['index'] ?? 0;
            $errors[] = new BatchError(is_numeric($index) ? (int) $index : 0, Wire::string($error['message'] ?? null));
        }

        return new SendBatchResult($data, $errors);
    }

    /**
     * Reads an email and its current delivery status. Requires a full-access key.
     *
     * @throws AgdirEmailException
     */
    public function get(string $id): Email
    {
        return Email::fromWire($this->transport->request('GET', '/emails/' . rawurlencode($id), retryable: true));
    }

    /**
     * Cancels a scheduled email that has not been sent yet.
     *
     * @throws AgdirEmailException
     */
    public function cancel(string $id): Reference
    {
        $res = $this->transport->request('POST', '/emails/' . rawurlencode($id) . '/cancel', retryable: true);

        return new Reference(Wire::string($res['id'] ?? null));
    }

    private function idempotencyKey(?string $key): ?string
    {
        if ($key !== null && $key !== '') {
            return $key;
        }

        // Without a key a retried send could be delivered twice, so only generate one when we may retry.
        return $this->transport->maxRetries > 0 ? EmailInput::uuid4() : null;
    }
}
