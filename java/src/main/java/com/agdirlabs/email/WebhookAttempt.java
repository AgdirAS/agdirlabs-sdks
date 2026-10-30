package com.agdirlabs.email;

import java.time.Instant;

/** One HTTP attempt to deliver a webhook message. */
public final class WebhookAttempt {
  private final String id;
  private final int httpStatus;
  private final String response;
  private final Instant sentAt;

  WebhookAttempt(String id, int httpStatus, String response, Instant sentAt) {
    this.id = id;
    this.httpStatus = httpStatus;
    this.response = response;
    this.sentAt = sentAt;
  }

  public String getId() {
    return id;
  }

  /** 0 when no HTTP response was received. */
  public int getHttpStatus() {
    return httpStatus;
  }

  /** The (truncated) response body or error. */
  public String getResponse() {
    return response;
  }

  public Instant getSentAt() {
    return sentAt;
  }

  @Override
  public String toString() {
    return "WebhookAttempt{id=" + id + ", httpStatus=" + httpStatus + ", sentAt=" + sentAt + "}";
  }
}
