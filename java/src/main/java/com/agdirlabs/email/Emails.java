package com.agdirlabs.email;

import static com.agdirlabs.email.Http.seg;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;

/** Send, read and cancel emails: {@code agdir.emails()}. */
public final class Emails {
  /** Most emails accepted by one {@link #sendBatch} call. */
  public static final int MAX_BATCH_SIZE = 100;

  private final Http http;

  Emails(Http http) {
    this.http = http;
  }

  /** Accepts one email for delivery (or for its scheduled time). */
  public SendResult send(SendEmailRequest email) {
    return Http.await(sendAsync(email, SendOptions.defaults()));
  }

  /** Accepts one email for delivery, with an idempotency key. */
  public SendResult send(SendEmailRequest email, SendOptions options) {
    return Http.await(sendAsync(email, options));
  }

  public CompletableFuture<SendResult> sendAsync(SendEmailRequest email) {
    return sendAsync(email, SendOptions.defaults());
  }

  public CompletableFuture<SendResult> sendAsync(SendEmailRequest email, SendOptions options) {
    if (email == null) throw new IllegalArgumentException("email must not be null");
    String key = idempotencyKey(options);
    Http.Call call =
        Http.Call.post("/emails").body(email.toWire()).header("Idempotency-Key", key).retryable(key != null);
    return http.send(call, body -> new SendResult(Wire.id(body)));
  }

  /** Accepts up to 100 emails in one request. Strict validation (the default) is all-or-nothing. */
  public BatchResult sendBatch(List<SendEmailRequest> emails) {
    return Http.await(sendBatchAsync(emails, SendOptions.defaults()));
  }

  /** Accepts up to 100 emails in one request; {@link SendOptions#permissive()} accepts the valid ones. */
  public BatchResult sendBatch(List<SendEmailRequest> emails, SendOptions options) {
    return Http.await(sendBatchAsync(emails, options));
  }

  public CompletableFuture<BatchResult> sendBatchAsync(List<SendEmailRequest> emails) {
    return sendBatchAsync(emails, SendOptions.defaults());
  }

  public CompletableFuture<BatchResult> sendBatchAsync(List<SendEmailRequest> emails, SendOptions options) {
    if (emails == null) throw new IllegalArgumentException("emails must not be null");
    List<Map<String, Object>> body = new ArrayList<>();
    for (SendEmailRequest e : emails) body.add(e.toWire());
    String key = idempotencyKey(options);
    boolean permissive = options != null && options.getValidation() == SendOptions.BatchValidation.PERMISSIVE;
    Http.Call call =
        Http.Call.post("/emails/batch")
            .body(body)
            .header("Idempotency-Key", key)
            .header("x-batch-validation", permissive ? "permissive" : null)
            .retryable(key != null);
    return http.send(call, Wire::batch);
  }

  /** Reads an email and its current delivery status. Requires a full-access key. */
  public Email get(String id) {
    return Http.await(getAsync(id));
  }

  public CompletableFuture<Email> getAsync(String id) {
    return http.send(Http.Call.get("/emails/" + seg(id)), Wire::email);
  }

  /** Cancels a scheduled email that has not been sent yet. Returns the email's id. */
  public String cancel(String id) {
    return Http.await(cancelAsync(id));
  }

  public CompletableFuture<String> cancelAsync(String id) {
    return http.send(Http.Call.post("/emails/" + seg(id) + "/cancel").retryable(true), Wire::id);
  }

  private String idempotencyKey(SendOptions options) {
    if (options != null && options.getIdempotencyKey() != null && !options.getIdempotencyKey().isEmpty()) {
      return options.getIdempotencyKey();
    }
    // Without a key a retried send could be delivered twice, so only generate one when we may retry.
    return http.maxRetries() > 0 ? UUID.randomUUID().toString() : null;
  }
}
