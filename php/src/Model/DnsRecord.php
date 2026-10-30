<?php

declare(strict_types=1);

namespace AgdirLabs\Email\Model;

use AgdirLabs\Email\Internal\Wire;

/** A DNS record to publish for a sending domain. */
final class DnsRecord
{
    public function __construct(
        /** What the record is for: `DKIM`, `SPF` or `DMARC`. */
        public readonly string $purpose,
        /** Relative to the registrable domain, e.g. `agdir._domainkey` or `send`. */
        public readonly string $name,
        /** `TXT`, `MX` or `CNAME`. */
        public readonly string $type,
        public readonly string $value,
        public readonly string $ttl,
        public readonly DomainStatus $status,
        public readonly ?int $priority = null,
    ) {
    }

    /**
     * @internal
     *
     * @param array<mixed> $r
     */
    public static function fromWire(array $r): self
    {
        return new self(
            purpose: Wire::string($r['record'] ?? null),
            name: Wire::string($r['name'] ?? null),
            type: Wire::string($r['type'] ?? null),
            value: Wire::string($r['value'] ?? null),
            ttl: Wire::string($r['ttl'] ?? null),
            status: DomainStatus::from(Wire::string($r['status'] ?? null)),
            priority: isset($r['priority']) && is_numeric($r['priority']) ? (int) $r['priority'] : null,
        );
    }
}
