package com.agdirlabs.email;

/** Whether a webhook endpoint receives events. {@link #UNKNOWN} stands for values added to the API after this SDK version. */
public enum WebhookStatus {
  ENABLED("enabled"),
  DISABLED("disabled"),
  UNKNOWN("unknown");

  private final String value;

  WebhookStatus(String value) {
    this.value = value;
  }

  /** The API's wire value, e.g. {@code "enabled"}. */
  public String value() {
    return value;
  }

  /** Parses a wire value; unrecognised values map to {@link #UNKNOWN}. */
  public static WebhookStatus fromValue(String value) {
    for (WebhookStatus v : values()) {
      if (v.value.equals(value)) return v;
    }
    return UNKNOWN;
  }

  @Override
  public String toString() {
    return value;
  }
}
