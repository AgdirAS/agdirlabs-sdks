package com.agdirlabs.email;

import java.util.List;

/** A domain with the DNS records to publish before it can be verified. */
public final class DomainWithRecords extends Domain {
  private final List<DnsRecord> records;

  DomainWithRecords(Domain d, List<DnsRecord> records) {
    super(d.getId(), d.getName(), d.getStatus(), d.getRegion(), d.getCreatedAt(), d.isOpenTracking(),
        d.isClickTracking());
    this.records = List.copyOf(records);
  }

  public List<DnsRecord> getRecords() {
    return records;
  }
}
