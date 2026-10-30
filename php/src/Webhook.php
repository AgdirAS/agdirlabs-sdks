<?php

declare(strict_types=1);

namespace AgdirLabs\Email;

use AgdirLabs\Email\Exception\WebhookVerificationException;
use AgdirLabs\Email\Model\WebhookEvent;

/**
 * Verifies webhook requests sent by Agdir Email.
 *
 * Messages are signed per the Standard Webhooks spec (`webhook-id`, `webhook-timestamp`,
 * `webhook-signature` headers, HMAC-SHA256 with the `whsec_…` secret).
 */
final class Webhook
{
    public const DEFAULT_TOLERANCE = 300;

    /**
     * Checks that a webhook request came from Agdir Email and returns the parsed event.
     *
     * @param string                                         $payload   the raw request body, exactly as received — not re-encoded JSON
     * @param array<string, string|list<string>|null>|object $headers   request headers: a `['name' => value]` array (any case,
     *                                                                  values may be lists) or an object with `get(string $name)`
     *                                                                  (Laravel/Symfony `HeaderBag`, PSR-7 via `getHeaders()`)
     * @param string                                         $secret    the webhook's `whsec_…` signing secret
     * @param int                                            $tolerance reject messages older or newer than this many seconds
     *
     * @throws WebhookVerificationException
     */
    public static function verify(
        string $payload,
        array|object $headers,
        string $secret,
        int $tolerance = self::DEFAULT_TOLERANCE,
        ?\DateTimeInterface $now = null,
    ): WebhookEvent {
        $id = self::header($headers, 'webhook-id');
        $timestamp = self::header($headers, 'webhook-timestamp');
        $signatures = self::header($headers, 'webhook-signature');
        if ($id === null || $timestamp === null || $signatures === null) {
            throw new WebhookVerificationException('Missing webhook signature headers');
        }

        $nowTs = ($now ?? new \DateTimeImmutable())->getTimestamp();
        if (!ctype_digit($timestamp) || abs($nowTs - (int) $timestamp) > $tolerance) {
            throw new WebhookVerificationException('Webhook timestamp is outside the allowed tolerance');
        }

        $encoded = str_starts_with($secret, 'whsec_') ? substr($secret, strlen('whsec_')) : $secret;
        $key = base64_decode($encoded, true);
        if ($key === false || $key === '') {
            throw new WebhookVerificationException('Signing secret is not valid base64');
        }
        $expected = base64_encode(hash_hmac('sha256', $id . '.' . $timestamp . '.' . $payload, $key, true));

        // The header may hold several space-separated `v1,<sig>` entries (e.g. during secret rotation).
        $valid = false;
        foreach (preg_split('/\s+/', trim($signatures)) ?: [] as $entry) {
            if (str_starts_with($entry, 'v1,') && hash_equals($expected, substr($entry, 3))) {
                $valid = true;
                break;
            }
        }
        if (!$valid) {
            throw new WebhookVerificationException('Webhook signature does not match');
        }

        try {
            $body = json_decode($payload, true, 512, JSON_THROW_ON_ERROR);
        } catch (\JsonException $e) {
            throw new WebhookVerificationException('Webhook payload is not valid JSON', 0, $e);
        }
        if (!is_array($body)) {
            throw new WebhookVerificationException('Webhook payload is not a JSON object');
        }
        try {
            return WebhookEvent::fromWire($body);
        } catch (\ValueError|\Exception $e) {
            throw new WebhookVerificationException('Webhook payload is not a valid Agdir Email event: ' . $e->getMessage(), 0, $e);
        }
    }

    /** @param array<string, string|list<string>|null>|object $headers */
    private static function header(array|object $headers, string $name): ?string
    {
        if (is_object($headers)) {
            if (method_exists($headers, 'get')) {
                $value = $headers->get($name);
            } elseif (method_exists($headers, 'getHeaderLine')) {
                $value = $headers->getHeaderLine($name);
            } elseif (method_exists($headers, 'getHeaders')) {
                return self::header((array) $headers->getHeaders(), $name);
            } else {
                return null;
            }
        } else {
            $value = null;
            foreach ($headers as $key => $v) {
                if (strcasecmp((string) $key, $name) === 0 || strcasecmp((string) $key, 'HTTP_' . str_replace('-', '_', $name)) === 0) {
                    $value = $v;
                    break;
                }
            }
        }
        if (is_array($value)) {
            $value = $value[0] ?? null;
        }

        return is_string($value) && $value !== '' ? $value : null;
    }
}
