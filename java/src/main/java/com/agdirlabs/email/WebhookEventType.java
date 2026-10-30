package com.agdirlabs.email;

/** Event types a webhook can subscribe to. {@link #UNKNOWN} stands for values added to the API after this SDK version. */
public enum WebhookEventType {
  SENT("email.sent"),
  SCHEDULED("email.scheduled"),
  DELIVERED("email.delivered"),
  DELIVERY_DELAYED("email.delivery_delayed"),
  COMPLAINED("email.complained"),
  BOUNCED("email.bounced"),
  OPENED("email.opened"),
  CLICKED("email.clicked"),
  FAILED("email.failed"),
  UNKNOWN("unknown");

  private final String value;

  WebhookEventType(String value) {
    this.value = value;
  }

  /** The API's wire value, e.g. {@code "email.sent"}. */
  public String value() {
    return value;
  }

  /** Parses a wire value; unrecognised values map to {@link #UNKNOWN}. */
  public static WebhookEventType fromValue(String value) {
    for (WebhookEventType v : values()) {
      if (v.value.equals(value)) return v;
    }
    return UNKNOWN;
  }

  @Override
  public String toString() {
    return value;
  }
}
