package com.agdirlabs.email;

import java.time.Instant;
import java.util.List;

/** A webhook endpoint, as listed by {@link Webhooks#list}. {@link WebhookWithSecret} adds its signing secret. */
public class Webhook {
  private final String id;
  private final String endpoint;
  private final WebhookStatus status;
  private final List<WebhookEventType> events;
  private final Instant createdAt;

  Webhook(String id, String endpoint, WebhookStatus status, List<WebhookEventType> events, Instant createdAt) {
    this.id = id;
    this.endpoint = endpoint;
    this.status = status;
    this.events = List.copyOf(events);
    this.createdAt = createdAt;
  }

  public String getId() {
    return id;
  }

  public String getEndpoint() {
    return endpoint;
  }

  public WebhookStatus getStatus() {
    return status;
  }

  public List<WebhookEventType> getEvents() {
    return events;
  }

  public Instant getCreatedAt() {
    return createdAt;
  }

  @Override
  public String toString() {
    return getClass().getSimpleName() + "{id=" + id + ", endpoint=" + endpoint + ", status=" + status + ", events="
        + events + "}";
  }
}
