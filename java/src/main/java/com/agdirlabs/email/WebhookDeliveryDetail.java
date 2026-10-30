package com.agdirlabs.email;

import java.util.Map;
import java.time.Instant;

/** A webhook delivery with its payload and next retry time. */
public final class WebhookDeliveryDetail extends WebhookDelivery {
  private final Instant nextAttemptAt;
  private final Map<String, Object> payload;
  private final String payloadJson;

  WebhookDeliveryDetail(WebhookDelivery d, Instant nextAttemptAt, Map<String, Object> payload) {
    super(d.getId(), d.getType(), d.getStatus(), d.getCreatedAt());
    this.nextAttemptAt = nextAttemptAt;
    this.payload = java.util.Collections.unmodifiableMap(payload);
    this.payloadJson = Json.write(payload);
  }

  /** {@code null} when no further attempt is scheduled. */
  public Instant getNextAttemptAt() {
    return nextAttemptAt;
  }

  /** The JSON body sent to your endpoint, parsed: nested {@code Map}s, {@code List}s, strings, numbers, booleans. */
  public Map<String, Object> getPayload() {
    return payload;
  }

  /** The JSON body sent to your endpoint, as a string (to feed your own JSON library). */
  public String getPayloadJson() {
    return payloadJson;
  }
}
