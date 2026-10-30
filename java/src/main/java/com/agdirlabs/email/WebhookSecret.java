package com.agdirlabs.email;

/** A webhook's id and (new) signing secret, returned by {@link Webhooks#create} and {@link Webhooks#rotateSecret}. */
public final class WebhookSecret {
  private final String id;
  private final String signingSecret;

  WebhookSecret(String id, String signingSecret) {
    this.id = id;
    this.signingSecret = signingSecret;
  }

  public String getId() {
    return id;
  }

  /** {@code whsec_…}: keep it to verify incoming events with {@link Webhooks#verify}. */
  public String getSigningSecret() {
    return signingSecret;
  }

  @Override
  public String toString() {
    return "WebhookSecret{id=" + id + ", signingSecret=whsec_***}";
  }
}
