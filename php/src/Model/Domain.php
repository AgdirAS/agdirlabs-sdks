<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

use AgdirLabs\Email\Internal\Wire;

/** A sending domain. `records` is only filled by `create()` and `get()`; list pages leave it empty. */
final class Domain
{
    /**
     * @param list<DnsRecord> $records
     */
    public function __construct(
        public readonly string $id,
        public readonly string $name,
        public readonly DomainStatus $status,
        public readonly string $region,
        public readonly \DateTimeImmutable $createdAt,
        public readonly bool $openTracking,
        public readonly bool $clickTracking,
        public readonly array $records = [],
    ) {
    }

    /**
     * @internal
     *
     * @param array<mixed> $d
     */
    public static function fromWire(array $d): self
    {
        $records = [];
        foreach (Wire::array($d['records'] ?? null) as $r) {
            $records[] = DnsRecord::fromWire(Wire::array($r));
        }

        return new self(
            id: Wire::string($d['id'] ?? null),
            name: Wire::string($d['name'] ?? null),
            status: DomainStatus::from(Wire::string($d['status'] ?? null)),
            region: Wire::string($d['region'] ?? null),
            createdAt: Wire::date($d['created_at'] ?? null),
            openTracking: (bool) ($d['open_tracking'] ?? false),
            clickTracking: (bool) ($d['click_tracking'] ?? false),
            records: $records,
        );
    }
}
