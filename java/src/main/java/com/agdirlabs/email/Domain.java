package com.agdirlabs.email;

import java.time.Instant;

/** A sending domain, as listed by {@link Domains#list}. {@link DomainWithRecords} adds its DNS records. */
public class Domain {
  private final String id;
  private final String name;
  private final DomainStatus status;
  private final String region;
  private final Instant createdAt;
  private final boolean openTracking;
  private final boolean clickTracking;

  Domain(String id, String name, DomainStatus status, String region, Instant createdAt, boolean openTracking,
      boolean clickTracking) {
    this.id = id;
    this.name = name;
    this.status = status;
    this.region = region;
    this.createdAt = createdAt;
    this.openTracking = openTracking;
    this.clickTracking = clickTracking;
  }

  public String getId() {
    return id;
  }

  public String getName() {
    return name;
  }

  public DomainStatus getStatus() {
    return status;
  }

  /** E.g. {@code eu-north-1}. */
  public String getRegion() {
    return region;
  }

  public Instant getCreatedAt() {
    return createdAt;
  }

  public boolean isOpenTracking() {
    return openTracking;
  }

  public boolean isClickTracking() {
    return clickTracking;
  }

  @Override
  public String toString() {
    return getClass().getSimpleName() + "{id=" + id + ", name=" + name + ", status=" + status + "}";
  }
}
