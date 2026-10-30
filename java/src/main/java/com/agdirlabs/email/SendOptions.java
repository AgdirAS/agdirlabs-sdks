package com.agdirlabs.email;

import java.util.Objects;

/**
 * Per-call options for {@link Emails#send} and {@link Emails#sendBatch}.
 *
 * <pre>{@code
 * agdir.emails().send(email, SendOptions.idempotencyKey("welcome/" + user.getId()));
 * agdir.emails().sendBatch(emails, SendOptions.permissive());
 * }</pre>
 */
public final class SendOptions {
  private static final SendOptions DEFAULTS = new SendOptions(null, BatchValidation.STRICT);

  private final String idempotencyKey;
  private final BatchValidation validation;

  private SendOptions(String idempotencyKey, BatchValidation validation) {
    this.idempotencyKey = idempotencyKey;
    this.validation = Objects.requireNonNull(validation, "validation");
  }

  /** No idempotency key (the client generates one when retries are enabled) and strict batch validation. */
  public static SendOptions defaults() {
    return DEFAULTS;
  }

  /**
   * Makes a send safe to retry: the same key with the same body returns the original result. The client
   * generates one per call automatically when retries are enabled; pass your own to stay safe across process
   * restarts too.
   */
  public static SendOptions idempotencyKey(String key) {
    return DEFAULTS.withIdempotencyKey(key);
  }

  /** Batch only: accept the valid emails and report the invalid ones in {@link BatchResult#getErrors()}. */
  public static SendOptions permissive() {
    return DEFAULTS.withValidation(BatchValidation.PERMISSIVE);
  }

  public SendOptions withIdempotencyKey(String key) {
    return new SendOptions(key, validation);
  }

  public SendOptions withValidation(BatchValidation validation) {
    return new SendOptions(idempotencyKey, validation);
  }

  /** May be {@code null}. */
  public String getIdempotencyKey() {
    return idempotencyKey;
  }

  public BatchValidation getValidation() {
    return validation;
  }

  /** How {@link Emails#sendBatch} treats invalid emails. */
  public enum BatchValidation {
    /** One invalid email rejects the whole batch (default). */
    STRICT,
    /** Valid emails are accepted, invalid ones are reported by index. */
    PERMISSIVE
  }
}
