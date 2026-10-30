package com.agdirlabs.email;

/** A webhook endpoint with its signing secret, as returned by {@link Webhooks#get}. */
public final class WebhookWithSecret extends Webhook {
  private final String signingSecret;

  WebhookWithSecret(Webhook w, String signingSecret) {
    super(w.getId(), w.getEndpoint(), w.getStatus(), w.getEvents(), w.getCreatedAt());
    this.signingSecret = signingSecret;
  }

  /** {@code whsec_…}: pass it to {@link Webhooks#verify}. */
  public String getSigningSecret() {
    return signingSecret;
  }
}
