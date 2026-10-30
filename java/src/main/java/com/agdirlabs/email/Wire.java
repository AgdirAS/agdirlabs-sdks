package com.agdirlabs.email;

import java.time.Instant;
import java.time.OffsetDateTime;
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.Collection;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.function.Function;

/** Mapping between the SDK's objects and the API's snake_case JSON. */
final class Wire {
  private Wire() {}

  static Instant date(Object value) {
    if (value == null) return null;
    String s = String.valueOf(value);
    if (s.isEmpty()) return null;
    return OffsetDateTime.parse(s, DateTimeFormatter.ISO_OFFSET_DATE_TIME).toInstant();
  }

  /** {@code null}, a string or a list → a list. */
  static List<String> list(Object value) {
    List<String> out = new ArrayList<>();
    if (value instanceof List) {
      for (Object o : (List<?>) value) {
        if (o != null) out.add(String.valueOf(o));
      }
    } else if (value != null) {
      out.add(String.valueOf(value));
    }
    return out;
  }

  static List<String> eventValues(Collection<WebhookEventType> events) {
    List<String> out = new ArrayList<>();
    for (WebhookEventType e : events) {
      if (e == null || e == WebhookEventType.UNKNOWN) throw new IllegalArgumentException("invalid event type: " + e);
      out.add(e.value());
    }
    return out;
  }

  static String id(Object body) {
    return Json.string(Json.object(body), "id");
  }

  static Email email(Object body) {
    Map<String, Object> e = Json.object(body);
    return new Email(
        Json.string(e, "id"),
        Json.string(e, "from"),
        list(e.get("to")),
        list(e.get("cc")),
        list(e.get("bcc")),
        list(e.get("reply_to")),
        Json.string(e, "subject"),
        Json.string(e, "html"),
        Json.string(e, "text"),
        date(e.get("created_at")),
        date(e.get("scheduled_at")),
        EmailStatus.fromValue(Json.string(e, "last_event")));
  }

  static BatchResult batch(Object body) {
    Map<String, Object> b = Json.object(body);
    List<SendResult> data = new ArrayList<>();
    for (Object d : Json.array(b.get("data"))) data.add(new SendResult(id(d)));
    List<BatchResult.BatchError> errors = new ArrayList<>();
    for (Object o : Json.array(b.get("errors"))) {
      Map<String, Object> e = Json.object(o);
      Integer index = Json.integer(e, "index");
      errors.add(new BatchResult.BatchError(index == null ? -1 : index, Json.string(e, "message")));
    }
    return new BatchResult(data, errors);
  }

  static DnsRecord record(Object value) {
    Map<String, Object> r = Json.object(value);
    return new DnsRecord(
        Json.string(r, "record"),
        Json.string(r, "name"),
        Json.string(r, "type"),
        Json.string(r, "value"),
        Json.string(r, "ttl"),
        Json.integer(r, "priority"),
        DomainStatus.fromValue(Json.string(r, "status")));
  }

  static Domain domain(Object body) {
    Map<String, Object> d = Json.object(body);
    return new Domain(
        Json.string(d, "id"),
        Json.string(d, "name"),
        DomainStatus.fromValue(Json.string(d, "status")),
        Json.string(d, "region"),
        date(d.get("created_at")),
        Json.bool(d, "open_tracking"),
        Json.bool(d, "click_tracking"));
  }

  static DomainWithRecords domainWithRecords(Object body) {
    List<DnsRecord> records = new ArrayList<>();
    for (Object r : Json.array(Json.object(body).get("records"))) records.add(record(r));
    return new DomainWithRecords(domain(body), records);
  }

  static Webhook webhook(Object body) {
    Map<String, Object> w = Json.object(body);
    List<WebhookEventType> events = new ArrayList<>();
    for (String e : list(w.get("events"))) events.add(WebhookEventType.fromValue(e));
    return new Webhook(
        Json.string(w, "id"),
        Json.string(w, "endpoint"),
        WebhookStatus.fromValue(Json.string(w, "status")),
        events,
        date(w.get("created_at")));
  }

  static WebhookWithSecret webhookWithSecret(Object body) {
    return new WebhookWithSecret(webhook(body), Json.string(Json.object(body), "signing_secret"));
  }

  static WebhookSecret webhookSecret(Object body) {
    Map<String, Object> w = Json.object(body);
    return new WebhookSecret(Json.string(w, "id"), Json.string(w, "signing_secret"));
  }

  static WebhookDelivery delivery(Object body) {
    Map<String, Object> e = Json.object(body);
    return new WebhookDelivery(
        Json.string(e, "id"),
        WebhookEventType.fromValue(Json.string(e, "type")),
        WebhookDeliveryStatus.fromValue(Json.string(e, "status")),
        date(e.get("created_at")));
  }

  static WebhookDeliveryDetail deliveryDetail(Object body) {
    Map<String, Object> e = Json.object(body);
    return new WebhookDeliveryDetail(delivery(body), date(e.get("next_attempt_at")), Json.object(e.get("payload")));
  }

  static WebhookAttempt attempt(Object body) {
    Map<String, Object> a = Json.object(body);
    Integer status = Json.integer(a, "http_status_code");
    return new WebhookAttempt(
        Json.string(a, "id"), status == null ? 0 : status, Json.string(a, "response"), date(a.get("sent_at")));
  }

  static <T> Function<Object, Page<T>> page(Function<Object, T> map) {
    return body -> {
      Map<String, Object> b = Json.object(body);
      List<Object> items = Json.array(b.get("data"));
      List<T> data = new ArrayList<>();
      for (Object item : items) data.add(map.apply(item));
      boolean hasMore = Json.bool(b, "has_more");
      String cursor = hasMore && !items.isEmpty() ? id(items.get(items.size() - 1)) : null;
      return new Page<>(data, hasMore, cursor);
    };
  }

  static WebhookEvent webhookEvent(Object body) {
    Map<String, Object> b = Json.object(body);
    Map<String, Object> d = Json.object(b.get("data"));
    Map<String, String> tags = new LinkedHashMap<>();
    Json.object(d.get("tags")).forEach((k, v) -> {
      if (v != null) tags.put(k, String.valueOf(v));
    });
    WebhookEvent.Bounce bounce = null;
    if (d.get("bounce") instanceof Map) {
      Map<String, Object> x = Json.object(d.get("bounce"));
      bounce = new WebhookEvent.Bounce(Json.string(x, "type"), Json.string(x, "subType"), Json.string(x, "message"));
    }
    WebhookEvent.Click click = null;
    if (d.get("click") instanceof Map) {
      Map<String, Object> x = Json.object(d.get("click"));
      click = new WebhookEvent.Click(
          Json.string(x, "link"), Json.string(x, "ipAddress"), Json.string(x, "userAgent"), date(x.get("timestamp")));
    }
    WebhookEvent.Failure failed = null;
    if (d.get("failed") instanceof Map) failed = new WebhookEvent.Failure(Json.string(Json.object(d.get("failed")), "reason"));
    WebhookEvent.Data data = new WebhookEvent.Data(
        Json.string(d, "email_id"),
        Json.string(d, "message_id"),
        Json.string(d, "from"),
        list(d.get("to")),
        Json.string(d, "subject"),
        tags,
        date(d.get("created_at")),
        bounce,
        click,
        failed);
    return new WebhookEvent(Json.string(b, "type"), date(b.get("created_at")), data);
  }
}
