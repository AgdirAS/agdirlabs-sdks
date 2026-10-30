package com.agdirlabs.email;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Changes to a webhook; unset fields are left as they are. Immutable; create it with {@link #builder()}.
 *
 * <pre>{@code
 * agdir.webhooks().update(id, UpdateWebhookRequest.builder().status(WebhookStatus.DISABLED).build());
 * }</pre>
 */
public final class UpdateWebhookRequest {
  private final String endpoint;
  private final List<WebhookEventType> events;
  private final WebhookStatus status;

  private UpdateWebhookRequest(Builder b) {
    this.endpoint = b.endpoint;
    this.events = b.events == null ? null : List.copyOf(b.events);
    this.status = b.status;
  }

  public static Builder builder() {
    return new Builder();
  }

  /** May be {@code null} (unchanged). */
  public String getEndpoint() {
    return endpoint;
  }

  /** May be {@code null} (unchanged). */
  public List<WebhookEventType> getEvents() {
    return events;
  }

  /** May be {@code null} (unchanged). */
  public WebhookStatus getStatus() {
    return status;
  }

  Map<String, Object> toWire() {
    Map<String, Object> m = new LinkedHashMap<>();
    if (endpoint != null) m.put("endpoint", endpoint);
    if (events != null) m.put("events", Wire.eventValues(events));
    if (status != null) m.put("status", status.value());
    return m;
  }

  public static final class Builder {
    private String endpoint;
    private List<WebhookEventType> events;
    private WebhookStatus status;

    private Builder() {}

    /** Public {@code https://} URL. */
    public Builder endpoint(String endpoint) {
      this.endpoint = endpoint;
      return this;
    }

    public Builder events(WebhookEventType... events) {
      return events(Arrays.asList(events));
    }

    public Builder events(Collection<WebhookEventType> events) {
      this.events = new ArrayList<>(events);
      return this;
    }

    public Builder status(WebhookStatus status) {
      if (status == WebhookStatus.UNKNOWN) throw new IllegalArgumentException("status must be ENABLED or DISABLED");
      this.status = status;
      return this;
    }

    public UpdateWebhookRequest build() {
      return new UpdateWebhookRequest(this);
    }
  }
}
