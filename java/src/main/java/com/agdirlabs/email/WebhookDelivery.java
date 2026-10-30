package com.agdirlabs.email;

import java.time.Instant;

/** One message sent (or queued) to a webhook endpoint. */
public class WebhookDelivery {
  private final String id;
  private final WebhookEventType type;
  private final WebhookDeliveryStatus status;
  private final Instant createdAt;

  WebhookDelivery(String id, WebhookEventType type, WebhookDeliveryStatus status, Instant createdAt) {
    this.id = id;
    this.type = type;
    this.status = status;
    this.createdAt = createdAt;
  }

  public String getId() {
    return id;
  }

  public WebhookEventType getType() {
    return type;
  }

  public WebhookDeliveryStatus getStatus() {
    return status;
  }

  public Instant getCreatedAt() {
    return createdAt;
  }

  @Override
  public String toString() {
    return getClass().getSimpleName() + "{id=" + id + ", type=" + type + ", status=" + status + "}";
  }
}
