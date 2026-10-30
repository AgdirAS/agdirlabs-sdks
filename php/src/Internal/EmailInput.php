<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Internal;

/**
 * Turns the SDK's send array into the API's JSON body.
 *
 * @internal
 */
final class EmailInput
{
    private const KEYS = [
        'from' => 'from',
        'to' => 'to',
        'subject' => 'subject',
        'html' => 'html',
        'text' => 'text',
        'cc' => 'cc',
        'bcc' => 'bcc',
        'replyTo' => 'reply_to',
        'reply_to' => 'reply_to',
        'headers' => 'headers',
        'tags' => 'tags',
        'scheduledAt' => 'scheduled_at',
        'scheduled_at' => 'scheduled_at',
    ];

    /**
     * @param array<mixed> $email
     *
     * @return array<string, mixed>
     */
    public static function toWire(array $email): array
    {
        $body = [];
        foreach ($email as $key => $value) {
            if (!is_string($key) || !isset(self::KEYS[$key])) {
                throw new \InvalidArgumentException(sprintf(
                    'Unknown email field "%s". Allowed: from, to, subject, html, text, cc, bcc, replyTo, headers, tags, scheduledAt.',
                    (string) $key,
                ));
            }
            if ($value === null) {
                continue;
            }
            $field = self::KEYS[$key];
            $body[$field] = match ($field) {
                'tags' => self::tags($value),
                'scheduled_at' => $value instanceof \DateTimeInterface ? $value->format('Y-m-d\TH:i:sP') : $value,
                'headers' => self::headers($value),
                default => $value,
            };
        }

        return $body;
    }

    /** @return list<array{name: string, value: string}> */
    private static function tags(mixed $tags): array
    {
        if (!is_array($tags)) {
            throw new \InvalidArgumentException('"tags" must be a list of ["name" => …, "value" => …] or a ["name" => "value"] map.');
        }
        $out = [];
        if (array_is_list($tags)) {
            foreach ($tags as $tag) {
                if (!is_array($tag) || !isset($tag['name'], $tag['value'])) {
                    throw new \InvalidArgumentException('Each tag needs a "name" and a "value".');
                }
                $out[] = ['name' => Wire::string($tag['name']), 'value' => Wire::string($tag['value'])];
            }

            return $out;
        }
        foreach ($tags as $name => $value) {
            $out[] = ['name' => (string) $name, 'value' => Wire::string($value)];
        }

        return $out;
    }

    /** @return array<string, string>|\stdClass */
    private static function headers(mixed $headers): array|\stdClass
    {
        if (!is_array($headers)) {
            throw new \InvalidArgumentException('"headers" must be a ["Name" => "value"] map.');
        }
        $out = [];
        foreach ($headers as $name => $value) {
            $out[(string) $name] = Wire::string($value);
        }

        // An empty PHP array would encode as `[]`, not `{}`.
        return $out === [] ? new \stdClass() : $out;
    }

    public static function uuid4(): string
    {
        $bytes = random_bytes(16);
        $bytes[6] = chr((ord($bytes[6]) & 0x0F) | 0x40);
        $bytes[8] = chr((ord($bytes[8]) & 0x3F) | 0x80);

        return vsprintf('%s%s-%s-%s-%s-%s%s%s', str_split(bin2hex($bytes), 4));
    }
}
