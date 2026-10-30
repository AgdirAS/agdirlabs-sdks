package com.agdirlabs.email;

/** Most advanced delivery state an email has reached. {@link #UNKNOWN} stands for values added to the API after this SDK version. */
public enum EmailStatus {
  QUEUED("queued"),
  SCHEDULED("scheduled"),
  SENT("sent"),
  DELIVERY_DELAYED("delivery_delayed"),
  DELIVERED("delivered"),
  OPENED("opened"),
  CLICKED("clicked"),
  BOUNCED("bounced"),
  FAILED("failed"),
  COMPLAINED("complained"),
  CANCELED("canceled"),
  UNKNOWN("unknown");

  private final String value;

  EmailStatus(String value) {
    this.value = value;
  }

  /** The API's wire value, e.g. {@code "queued"}. */
  public String value() {
    return value;
  }

  /** Parses a wire value; unrecognised values map to {@link #UNKNOWN}. */
  public static EmailStatus fromValue(String value) {
    for (EmailStatus v : values()) {
      if (v.value.equals(value)) return v;
    }
    return UNKNOWN;
  }

  @Override
  public String toString() {
    return value;
  }
}
