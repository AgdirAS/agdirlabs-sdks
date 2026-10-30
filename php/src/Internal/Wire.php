<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Internal;

/**
 * Helpers for reading the API's snake_case JSON.
 *
 * @internal
 */
final class Wire
{
    public static function date(mixed $value): \DateTimeImmutable
    {
        return new \DateTimeImmutable(is_string($value) ? $value : '@0');
    }

    public static function optionalDate(mixed $value): ?\DateTimeImmutable
    {
        return is_string($value) && $value !== '' ? new \DateTimeImmutable($value) : null;
    }

    /**
     * `null` → `[]`, `"a"` → `["a"]`.
     *
     * @return list<string>
     */
    public static function stringList(mixed $value): array
    {
        if ($value === null) {
            return [];
        }
        if (!is_array($value)) {
            return [(string) (is_scalar($value) ? $value : '')];
        }
        $out = [];
        foreach ($value as $item) {
            $out[] = is_scalar($item) ? (string) $item : '';
        }

        return $out;
    }

    public static function string(mixed $value): string
    {
        return is_scalar($value) ? (string) $value : '';
    }

    public static function nullableString(mixed $value): ?string
    {
        return $value === null ? null : self::string($value);
    }

    /** @return array<mixed> */
    public static function array(mixed $value): array
    {
        return is_array($value) ? $value : [];
    }
}
