package com.agdirlabs.email;

/** Thrown by {@link Webhooks#verify} when a payload's signature, timestamp or body is invalid. */
public class WebhookVerificationException extends RuntimeException {
  private static final long serialVersionUID = 1L;

  public WebhookVerificationException(String message) {
    super(message);
  }

  public WebhookVerificationException(String message, Throwable cause) {
    super(message, cause);
  }
}
