<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Internal;

use AgdirLabs\Email\Model\Page;

/** @internal */
final class Paginator
{
    /**
     * @template T
     *
     * @param callable(?string): Page<T> $fetchPage
     *
     * @return \Generator<int, T, mixed, void>
     */
    public static function walk(callable $fetchPage, ?string $after = null): \Generator
    {
        $index = 0;
        do {
            $page = $fetchPage($after);
            foreach ($page->data as $item) {
                yield $index++ => $item;
            }
            $after = $page->nextCursor;
        } while ($after !== null);
    }
}
