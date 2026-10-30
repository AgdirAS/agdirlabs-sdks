package com.agdirlabs.email;

/** State of one message to a webhook endpoint. {@link #UNKNOWN} stands for values added to the API after this SDK version. */
public enum WebhookDeliveryStatus {
  PENDING("pending"),
  ATTEMPTING("attempting"),
  SUCCESS("success"),
  FAILED("failed"),
  UNKNOWN("unknown");

  private final String value;

  WebhookDeliveryStatus(String value) {
    this.value = value;
  }

  /** The API's wire value, e.g. {@code "pending"}. */
  public String value() {
    return value;
  }

  /** Parses a wire value; unrecognised values map to {@link #UNKNOWN}. */
  public static WebhookDeliveryStatus fromValue(String value) {
    for (WebhookDeliveryStatus v : values()) {
      if (v.value.equals(value)) return v;
    }
    return UNKNOWN;
  }

  @Override
  public String toString() {
    return value;
  }
}
