package com.agdirlabs.email;

import static com.agdirlabs.email.Http.seg;

import java.util.concurrent.CompletableFuture;

/** Messages sent (or queued) to one webhook endpoint, with their HTTP attempts: {@code agdir.webhooks().deliveries()}. */
public final class WebhookDeliveries {
  private final Http http;

  WebhookDeliveries(Http http) {
    this.http = http;
  }

  /** The first page of deliveries, newest first. */
  public Page<WebhookDelivery> list(String webhookId) {
    return list(webhookId, ListOptions.defaults());
  }

  public Page<WebhookDelivery> list(String webhookId, ListOptions options) {
    return Http.await(listAsync(webhookId, options));
  }

  public CompletableFuture<Page<WebhookDelivery>> listAsync(String webhookId, ListOptions options) {
    ListOptions o = options == null ? ListOptions.defaults() : options;
    Http.Call call =
        Http.Call.get("/webhooks/" + seg(webhookId) + "/events")
            .query("limit", o.getLimit())
            .query("after", o.getAfter());
    return http.send(call, Wire.page(Wire::delivery));
  }

  public PagedIterable<WebhookDelivery> listAll(String webhookId) {
    return listAll(webhookId, ListOptions.defaults());
  }

  public PagedIterable<WebhookDelivery> listAll(String webhookId, ListOptions options) {
    return new PagedIterable<>(o -> list(webhookId, o), options);
  }

  public WebhookDeliveryDetail get(String webhookId, String deliveryId) {
    return Http.await(getAsync(webhookId, deliveryId));
  }

  public CompletableFuture<WebhookDeliveryDetail> getAsync(String webhookId, String deliveryId) {
    return http.send(
        Http.Call.get("/webhooks/" + seg(webhookId) + "/events/" + seg(deliveryId)), Wire::deliveryDetail);
  }

  /** HTTP attempts for one delivery, newest first. */
  public Page<WebhookAttempt> attempts(String webhookId, String deliveryId) {
    return attempts(webhookId, deliveryId, ListOptions.defaults());
  }

  public Page<WebhookAttempt> attempts(String webhookId, String deliveryId, ListOptions options) {
    return Http.await(attemptsAsync(webhookId, deliveryId, options));
  }

  public CompletableFuture<Page<WebhookAttempt>> attemptsAsync(
      String webhookId, String deliveryId, ListOptions options) {
    ListOptions o = options == null ? ListOptions.defaults() : options;
    Http.Call call =
        Http.Call.get("/webhooks/" + seg(webhookId) + "/events/" + seg(deliveryId) + "/attempts")
            .query("limit", o.getLimit())
            .query("after", o.getAfter());
    return http.send(call, Wire.page(Wire::attempt));
  }

  /** Sends the identical message again, once, right now (also to disabled endpoints). Returns the delivery id. */
  public String replay(String webhookId, String deliveryId) {
    return Http.await(replayAsync(webhookId, deliveryId));
  }

  public CompletableFuture<String> replayAsync(String webhookId, String deliveryId) {
    return http.send(
        Http.Call.post("/webhooks/" + seg(webhookId) + "/events/" + seg(deliveryId) + "/replay"), Wire::id);
  }
}
