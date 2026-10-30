package com.agdirlabs.email;

import static com.agdirlabs.email.Http.seg;

import java.lang.reflect.Array;
import java.nio.charset.StandardCharsets;
import java.security.GeneralSecurityException;
import java.security.MessageDigest;
import java.time.Duration;
import java.time.Instant;
import java.util.Base64;
import java.util.Collection;
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.concurrent.CompletableFuture;
import java.util.function.Function;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;

/**
 * Webhook endpoints ({@code agdir.webhooks()}) and verification of the events they receive
 * ({@link #verify(String, Map, String) Webhooks.verify}).
 */
public final class Webhooks {
  /** Default allowed clock difference for {@link #verify}: 5 minutes. */
  public static final Duration DEFAULT_TOLERANCE = Duration.ofMinutes(5);

  private final Http http;
  private final WebhookDeliveries deliveries;

  Webhooks(Http http) {
    this.http = http;
    this.deliveries = new WebhookDeliveries(http);
  }

  /** Messages sent to an endpoint and their HTTP attempts. */
  public WebhookDeliveries deliveries() {
    return deliveries;
  }

  /** Registers a public {@code https://} endpoint. Keep the returned signing secret to verify incoming events. */
  public WebhookSecret create(String endpoint, List<WebhookEventType> events) {
    return Http.await(createAsync(endpoint, events));
  }

  public CompletableFuture<WebhookSecret> createAsync(String endpoint, List<WebhookEventType> events) {
    Map<String, Object> body = new LinkedHashMap<>();
    body.put("endpoint", Objects.requireNonNull(endpoint, "endpoint"));
    body.put("events", Wire.eventValues(Objects.requireNonNull(events, "events")));
    return http.send(Http.Call.post("/webhooks").body(body), Wire::webhookSecret);
  }

  public WebhookWithSecret get(String id) {
    return Http.await(getAsync(id));
  }

  public CompletableFuture<WebhookWithSecret> getAsync(String id) {
    return http.send(Http.Call.get("/webhooks/" + seg(id)), Wire::webhookWithSecret);
  }

  /** The first page of webhooks. */
  public Page<Webhook> list() {
    return list(ListOptions.defaults());
  }

  public Page<Webhook> list(ListOptions options) {
    return Http.await(listAsync(options));
  }

  public CompletableFuture<Page<Webhook>> listAsync(ListOptions options) {
    ListOptions o = options == null ? ListOptions.defaults() : options;
    Http.Call call = Http.Call.get("/webhooks").query("limit", o.getLimit()).query("after", o.getAfter());
    return http.send(call, Wire.page(Wire::webhook));
  }

  /** Every webhook, fetching further pages as you iterate. */
  public PagedIterable<Webhook> listAll() {
    return listAll(ListOptions.defaults());
  }

  public PagedIterable<Webhook> listAll(ListOptions options) {
    return new PagedIterable<>(this::list, options);
  }

  /** Changes the endpoint, events and/or status. Returns the webhook's id. */
  public String update(String id, UpdateWebhookRequest changes) {
    return Http.await(updateAsync(id, changes));
  }

  public CompletableFuture<String> updateAsync(String id, UpdateWebhookRequest changes) {
    Objects.requireNonNull(changes, "changes");
    return http.send(Http.Call.patch("/webhooks/" + seg(id)).body(changes.toWire()).retryable(true), Wire::id);
  }

  public Deleted delete(String id) {
    return Http.await(deleteAsync(id));
  }

  public CompletableFuture<Deleted> deleteAsync(String id) {
    return http.send(Http.Call.delete("/webhooks/" + seg(id)), body -> new Deleted(Wire.id(body)));
  }

  /** Issues a new signing secret; the old one stops working immediately. */
  public WebhookSecret rotateSecret(String id) {
    return Http.await(rotateSecretAsync(id));
  }

  public CompletableFuture<WebhookSecret> rotateSecretAsync(String id) {
    return http.send(Http.Call.post("/webhooks/" + seg(id) + "/signing-secret/rotate"), Wire::webhookSecret);
  }

  // ---- signature verification ----

  /**
   * Checks that a webhook request came from Agdir Email and returns the parsed event.
   *
   * <p>Messages are signed per the Standard Webhooks spec ({@code webhook-id}, {@code webhook-timestamp},
   * {@code webhook-signature}); messages older or newer than 5 minutes are rejected.
   *
   * @param payload the raw request body, exactly as received (not re-serialized JSON)
   * @param headers request headers; names are matched case-insensitively, values may be strings or lists
   * @param secret the webhook's {@code whsec_…} signing secret
   * @throws WebhookVerificationException if anything doesn't match
   */
  public static WebhookEvent verify(String payload, Map<String, ?> headers, String secret) {
    return verify(payload, mapLookup(headers), secret, DEFAULT_TOLERANCE, Instant.now());
  }

  /**
   * Like {@link #verify(String, Map, String)}, reading headers through a lookup function such as Servlet's
   * {@code request::getHeader} or Spring's {@code httpHeaders::getFirst}.
   */
  public static WebhookEvent verify(String payload, Function<String, String> header, String secret) {
    return verify(payload, header, secret, DEFAULT_TOLERANCE, Instant.now());
  }

  /** Like {@link #verify(String, Map, String)} with a custom tolerance and clock (for tests). */
  public static WebhookEvent verify(
      String payload, Map<String, ?> headers, String secret, Duration tolerance, Instant now) {
    return verify(payload, mapLookup(headers), secret, tolerance, now);
  }

  /** Like {@link #verify(String, Function, String)} with a custom tolerance and clock (for tests). */
  public static WebhookEvent verify(
      String payload, Function<String, String> header, String secret, Duration tolerance, Instant now) {
    Objects.requireNonNull(payload, "payload");
    Objects.requireNonNull(header, "header");
    Objects.requireNonNull(secret, "secret");
    String id = header.apply("webhook-id");
    String timestamp = header.apply("webhook-timestamp");
    String signatures = header.apply("webhook-signature");
    if (isBlank(id) || isBlank(timestamp) || isBlank(signatures)) {
      throw new WebhookVerificationException("Missing webhook signature headers");
    }

    long sentAt;
    try {
      sentAt = Long.parseLong(timestamp.trim());
    } catch (NumberFormatException e) {
      throw new WebhookVerificationException("Webhook timestamp is outside the allowed tolerance");
    }
    long nowSeconds = (now == null ? Instant.now() : now).getEpochSecond();
    long toleranceSeconds = (tolerance == null ? DEFAULT_TOLERANCE : tolerance).getSeconds();
    if (Math.abs(nowSeconds - sentAt) > toleranceSeconds) {
      throw new WebhookVerificationException("Webhook timestamp is outside the allowed tolerance");
    }

    String encodedKey = secret.startsWith("whsec_") ? secret.substring("whsec_".length()) : secret;
    byte[] key;
    try {
      key = Base64.getDecoder().decode(encodedKey);
    } catch (IllegalArgumentException e) {
      throw new WebhookVerificationException("Signing secret is not valid base64", e);
    }
    if (key.length == 0) throw new WebhookVerificationException("Signing secret is empty");

    byte[] expected;
    try {
      Mac mac = Mac.getInstance("HmacSHA256");
      mac.init(new SecretKeySpec(key, "HmacSHA256"));
      byte[] signed = mac.doFinal((id + "." + timestamp + "." + payload).getBytes(StandardCharsets.UTF_8));
      expected = Base64.getEncoder().encodeToString(signed).getBytes(StandardCharsets.US_ASCII);
    } catch (GeneralSecurityException e) {
      throw new IllegalStateException("HmacSHA256 is not available", e);
    }

    // The header may hold several space-separated `v1,<sig>` entries (e.g. during secret rotation).
    boolean valid = false;
    for (String entry : signatures.trim().split("\\s+")) {
      if (entry.startsWith("v1,")
          && MessageDigest.isEqual(entry.substring(3).getBytes(StandardCharsets.US_ASCII), expected)) {
        valid = true;
      }
    }
    if (!valid) throw new WebhookVerificationException("Webhook signature does not match");

    Object json;
    try {
      json = Json.parse(payload);
    } catch (IllegalArgumentException e) {
      throw new WebhookVerificationException("Webhook payload is not valid JSON", e);
    }
    try {
      return Wire.webhookEvent(json);
    } catch (RuntimeException e) {
      throw new WebhookVerificationException("Webhook payload is not a valid event", e);
    }
  }

  private static boolean isBlank(String s) {
    return s == null || s.trim().isEmpty();
  }

  private static Function<String, String> mapLookup(Map<String, ?> headers) {
    Objects.requireNonNull(headers, "headers");
    return name -> {
      for (Map.Entry<String, ?> e : headers.entrySet()) {
        if (e.getKey() != null && e.getKey().equalsIgnoreCase(name)) return first(e.getValue());
      }
      return null;
    };
  }

  private static String first(Object value) {
    if (value == null) return null;
    if (value instanceof Collection) {
      Iterator<?> it = ((Collection<?>) value).iterator();
      return it.hasNext() ? first(it.next()) : null;
    }
    if (value.getClass().isArray()) return Array.getLength(value) > 0 ? first(Array.get(value, 0)) : null;
    return String.valueOf(value);
  }
}
