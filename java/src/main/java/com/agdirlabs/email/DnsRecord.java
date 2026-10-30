package com.agdirlabs.email;

/** A DNS record to publish for a sending domain. */
public final class DnsRecord {
  private final String purpose;
  private final String name;
  private final String type;
  private final String value;
  private final String ttl;
  private final Integer priority;
  private final DomainStatus status;

  DnsRecord(String purpose, String name, String type, String value, String ttl, Integer priority,
      DomainStatus status) {
    this.purpose = purpose;
    this.name = name;
    this.type = type;
    this.value = value;
    this.ttl = ttl;
    this.priority = priority;
    this.status = status;
  }

  /** What the record is for: {@code DKIM}, {@code SPF} or {@code DMARC}. */
  public String getPurpose() {
    return purpose;
  }

  /** Relative to the registrable domain, e.g. {@code agdir._domainkey} or {@code send}. */
  public String getName() {
    return name;
  }

  /** {@code TXT}, {@code MX} or {@code CNAME}. */
  public String getType() {
    return type;
  }

  public String getValue() {
    return value;
  }

  /** E.g. {@code Auto}. */
  public String getTtl() {
    return ttl;
  }

  /** MX priority, or {@code null} for other record types. */
  public Integer getPriority() {
    return priority;
  }

  public DomainStatus getStatus() {
    return status;
  }

  @Override
  public String toString() {
    return "DnsRecord{" + purpose + " " + type + " " + name + " " + value + (priority != null ? " " + priority : "")
        + ", status=" + status + "}";
  }
}
