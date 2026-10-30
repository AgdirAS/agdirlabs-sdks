package com.agdirlabs.email;

/** Verification state of a sending domain or one of its DNS records. {@link #UNKNOWN} stands for values added to the API after this SDK version. */
public enum DomainStatus {
  NOT_STARTED("not_started"),
  PENDING("pending"),
  VERIFIED("verified"),
  FAILED("failed"),
  TEMPORARY_FAILURE("temporary_failure"),
  UNKNOWN("unknown");

  private final String value;

  DomainStatus(String value) {
    this.value = value;
  }

  /** The API's wire value, e.g. {@code "not_started"}. */
  public String value() {
    return value;
  }

  /** Parses a wire value; unrecognised values map to {@link #UNKNOWN}. */
  public static DomainStatus fromValue(String value) {
    for (DomainStatus v : values()) {
      if (v.value.equals(value)) return v;
    }
    return UNKNOWN;
  }

  @Override
  public String toString() {
    return value;
  }
}
