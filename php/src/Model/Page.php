<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

use AgdirLabs\Email\Internal\Wire;

/**
 * One page of a list endpoint.
 *
 * @template T
 */
final class Page
{
    /**
     * @param list<T> $data
     */
    public function __construct(
        public readonly array $data,
        public readonly bool $hasMore,
        /** Pass as `after` to fetch the next page; null on the last page. */
        public readonly ?string $nextCursor,
    ) {
    }

    /**
     * @internal
     *
     * @template U
     *
     * @param array<mixed>                $body
     * @param callable(array<mixed>): U   $map
     *
     * @return self<U>
     */
    public static function fromWire(array $body, callable $map): self
    {
        $items = array_values(Wire::array($body['data'] ?? null));
        $data = [];
        foreach ($items as $item) {
            $data[] = $map(Wire::array($item));
        }
        $hasMore = (bool) ($body['has_more'] ?? false);
        $last = $items === [] ? null : Wire::array($items[count($items) - 1]);

        return new self($data, $hasMore, $hasMore && $last !== null ? Wire::string($last['id'] ?? null) : null);
    }
}
