package com.agdirlabs.email;

import static com.agdirlabs.email.StubServer.Reply.drop;
import static com.agdirlabs.email.StubServer.Reply.json;
import static com.agdirlabs.email.StubServer.Reply.slow;
import static com.agdirlabs.email.StubServer.Reply.status;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.time.Duration;
import java.time.Instant;
import java.time.OffsetDateTime;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ExecutionException;
import java.util.stream.Collectors;
import org.junit.jupiter.api.Test;

class ClientTest {
  private static SendEmailRequest simple() {
    return SendEmailRequest.builder().from("a@acme.com").to("b@example.com").subject("s").text("t").build();
  }

  @Test
  void sendsAnEmailAsApiJsonWithAuthUserAgentAndIdempotencyKey() throws Exception {
    try (StubServer api = new StubServer(json("{\"id\":\"em_1\"}"))) {
      SendResult result = api.client().emails().send(SendEmailRequest.builder()
          .from("Acme <hello@acme.com>")
          .to("user@example.com")
          .subject("Hi")
          .text("Hello")
          .replyTo("support@acme.com")
          .header("X-Entity-Ref-ID", "42")
          .scheduledAt(Instant.parse("2026-11-01T09:00:00Z"))
          .tag("kind", "welcome")
          .build());

      assertEquals("em_1", result.getId());
      StubServer.Recorded req = api.request(0);
      assertEquals("POST", req.method);
      assertEquals("/emails", req.path);
      assertEquals("Bearer re_test", req.header("Authorization"));
      assertEquals("agdirlabs-email-java/" + AgdirEmail.VERSION, req.header("User-Agent"));
      assertEquals("application/json", req.header("Accept"));
      assertEquals("application/json", req.header("Content-Type"));
      assertTrue(req.header("Idempotency-Key").matches("[0-9a-f-]{36}"), req.header("Idempotency-Key"));
      assertEquals(
          "{\"from\":\"Acme <hello@acme.com>\",\"to\":[\"user@example.com\"],\"subject\":\"Hi\",\"text\":\"Hello\","
              + "\"reply_to\":[\"support@acme.com\"],\"headers\":{\"X-Entity-Ref-ID\":\"42\"},"
              + "\"tags\":[{\"name\":\"kind\",\"value\":\"welcome\"}],\"scheduled_at\":\"2026-11-01T09:00:00Z\"}",
          req.body);
    }
  }

  @Test
  void schedulesWithAnOffsetDateTime() throws Exception {
    try (StubServer api = new StubServer(json("{\"id\":\"em_1\"}"))) {
      api.client().emails().send(simple().toBuilder()
          .scheduledAt(OffsetDateTime.of(2026, 11, 1, 10, 0, 0, 0, ZoneOffset.ofHours(1)))
          .build());
      assertEquals("2026-11-01T10:00:00+01:00", Json.object(api.request(0).json()).get("scheduled_at"));
    }
  }

  @Test
  void usesACallerIdempotencyKeyAndNoneWhenRetriesAreOff() throws Exception {
    try (StubServer api = new StubServer(json("{\"id\":\"a\"}"), json("{\"id\":\"b\"}"))) {
      api.client().emails().send(simple(), SendOptions.idempotencyKey("order-42"));
      api.client(0).emails().send(simple());
      assertEquals("order-42", api.request(0).header("Idempotency-Key"));
      assertNull(api.request(1).header("Idempotency-Key"));
    }
  }

  @Test
  void sendsPermissiveBatchesAndReturnsPerIndexErrors() throws Exception {
    try (StubServer api = new StubServer(
        json("{\"data\":[{\"id\":\"em_1\"}],\"errors\":[{\"index\":1,\"message\":\"bad to\"}]}"))) {
      BatchResult result = api.client().emails().sendBatch(
          List.of(simple(), simple().toBuilder().to("nope").build()), SendOptions.permissive());

      assertEquals(List.of("em_1"), result.getData().stream().map(SendResult::getId).collect(Collectors.toList()));
      assertEquals(1, result.getErrors().size());
      assertEquals(1, result.getErrors().get(0).getIndex());
      assertEquals("bad to", result.getErrors().get(0).getMessage());
      assertEquals("/emails/batch", api.request(0).path);
      assertEquals("permissive", api.request(0).header("x-batch-validation"));
      assertEquals(2, Json.array(api.request(0).json()).size());
    }
  }

  @Test
  void strictBatchesSendNoValidationHeaderAndHaveNoErrors() throws Exception {
    try (StubServer api = new StubServer(json("{\"data\":[{\"id\":\"em_1\"}]}"))) {
      BatchResult result = api.client().emails().sendBatch(List.of(simple()));
      assertNull(api.request(0).header("x-batch-validation"));
      assertTrue(result.getErrors().isEmpty());
    }
  }

  @Test
  void mapsAnEmailWithInstantsAndEmptyLists() throws Exception {
    try (StubServer api = new StubServer(json("{\"object\":\"email\",\"id\":\"em_1\",\"from\":\"a@acme.com\","
        + "\"to\":[\"b@example.com\"],\"cc\":null,\"bcc\":null,\"reply_to\":null,\"subject\":\"s\",\"html\":null,"
        + "\"text\":\"t\",\"created_at\":\"2026-10-30T10:00:00.000Z\",\"scheduled_at\":null,"
        + "\"last_event\":\"delivered\"}"))) {
      Email email = api.client().emails().get("em_1");
      assertEquals("GET", api.request(0).method);
      assertEquals("/emails/em_1", api.request(0).path);
      assertEquals("em_1", email.getId());
      assertEquals(List.of("b@example.com"), email.getTo());
      assertEquals(List.of(), email.getCc());
      assertEquals(List.of(), email.getBcc());
      assertEquals(List.of(), email.getReplyTo());
      assertNull(email.getHtml());
      assertEquals("t", email.getText());
      assertEquals(Instant.parse("2026-10-30T10:00:00Z"), email.getCreatedAt());
      assertNull(email.getScheduledAt());
      assertEquals(EmailStatus.DELIVERED, email.getStatus());
    }
  }

  @Test
  void unknownEnumValuesMapToUnknown() throws Exception {
    try (StubServer api = new StubServer(json("{\"id\":\"em_1\",\"to\":\"b@example.com\","
        + "\"created_at\":\"2026-10-30T10:00:00+02:00\",\"last_event\":\"teleported\"}"))) {
      Email email = api.client().emails().get("em_1");
      assertEquals(EmailStatus.UNKNOWN, email.getStatus());
      assertEquals(List.of("b@example.com"), email.getTo());
      assertEquals(Instant.parse("2026-10-30T08:00:00Z"), email.getCreatedAt());
    }
  }

  @Test
  void mapsDomainsWithDnsRecords() throws Exception {
    try (StubServer api = new StubServer(json("{\"object\":\"domain\",\"id\":\"d_1\",\"name\":\"acme.com\","
        + "\"status\":\"pending\",\"region\":\"eu-north-1\",\"created_at\":\"2026-10-30T10:00:00.000Z\","
        + "\"capabilities\":{\"sending\":\"enabled\",\"receiving\":\"disabled\"},"
        + "\"open_tracking\":false,\"click_tracking\":false,\"records\":[{\"record\":\"SPF\",\"name\":\"send\","
        + "\"type\":\"MX\",\"ttl\":\"Auto\",\"status\":\"pending\","
        + "\"value\":\"feedback-smtp.eu-north-1.amazonses.com\",\"priority\":10}]}"))) {
      DomainWithRecords domain = api.client().domains().create("acme.com");
      assertEquals("{\"name\":\"acme.com\"}", api.request(0).body);
      assertEquals(DomainStatus.PENDING, domain.getStatus());
      assertEquals("eu-north-1", domain.getRegion());
      assertEquals(Instant.parse("2026-10-30T10:00:00Z"), domain.getCreatedAt());
      assertFalse(domain.isOpenTracking());
      DnsRecord r = domain.getRecords().get(0);
      assertEquals("SPF", r.getPurpose());
      assertEquals("send", r.getName());
      assertEquals("MX", r.getType());
      assertEquals("Auto", r.getTtl());
      assertEquals(10, r.getPriority());
      assertEquals(DomainStatus.PENDING, r.getStatus());
      assertEquals("feedback-smtp.eu-north-1.amazonses.com", r.getValue());
    }
  }

  @Test
  void domainVerifyAndDelete() throws Exception {
    try (StubServer api = new StubServer(json("{\"object\":\"domain\",\"id\":\"d_1\"}"),
        json("{\"object\":\"domain\",\"id\":\"d_1\",\"deleted\":true}"))) {
      assertEquals("d_1", api.client().domains().verify("d_1"));
      Deleted deleted = api.client().domains().delete("d_1");
      assertEquals("d_1", deleted.getId());
      assertTrue(deleted.isDeleted());
      assertEquals("POST /domains/d_1/verify", api.request(0).method + " " + api.request(0).path);
      assertEquals("DELETE /domains/d_1", api.request(1).method + " " + api.request(1).path);
    }
  }

  @Test
  void mapsWebhookCreateAndRotateToSigningSecret() throws Exception {
    try (StubServer api = new StubServer(json("{\"object\":\"webhook\",\"id\":\"wh_1\",\"signing_secret\":\"whsec_a\"}"),
        json("{\"object\":\"webhook\",\"id\":\"wh_1\",\"signing_secret\":\"whsec_b\"}"))) {
      AgdirEmail agdir = api.client();
      WebhookSecret created =
          agdir.webhooks().create("https://acme.com/hook", List.of(WebhookEventType.DELIVERED, WebhookEventType.BOUNCED));
      assertEquals("wh_1", created.getId());
      assertEquals("whsec_a", created.getSigningSecret());
      assertEquals("{\"endpoint\":\"https://acme.com/hook\",\"events\":[\"email.delivered\",\"email.bounced\"]}",
          api.request(0).body);
      assertEquals("whsec_b", agdir.webhooks().rotateSecret("wh_1").getSigningSecret());
      assertEquals("/webhooks/wh_1/signing-secret/rotate", api.request(1).path);
    }
  }

  @Test
  void updatesAWebhookWithOnlyTheGivenFields() throws Exception {
    try (StubServer api = new StubServer(json("{\"object\":\"webhook\",\"id\":\"wh_1\"}"))) {
      String id = api.client().webhooks()
          .update("wh_1", UpdateWebhookRequest.builder().status(WebhookStatus.DISABLED).build());
      assertEquals("wh_1", id);
      assertEquals("PATCH", api.request(0).method);
      assertEquals("{\"status\":\"disabled\"}", api.request(0).body);
    }
  }

  @Test
  void mapsWebhookGetDeliveriesAndAttempts() throws Exception {
    try (StubServer api = new StubServer(
        json("{\"object\":\"webhook\",\"id\":\"wh_1\",\"endpoint\":\"https://acme.com/h\",\"status\":\"enabled\","
            + "\"events\":[\"email.sent\"],\"created_at\":\"2026-10-30T10:00:00.000Z\",\"signing_secret\":\"whsec_x\"}"),
        json("{\"object\":\"list\",\"has_more\":false,\"data\":[{\"id\":\"ev_1\",\"type\":\"email.sent\","
            + "\"created_at\":\"2026-10-30T10:00:00.000Z\",\"status\":\"failed\"}]}"),
        json("{\"id\":\"ev_1\",\"type\":\"email.sent\",\"created_at\":\"2026-10-30T10:00:00.000Z\","
            + "\"status\":\"attempting\",\"next_attempt_at\":\"2026-10-30T10:05:00.000Z\","
            + "\"payload\":{\"type\":\"email.sent\",\"data\":{\"email_id\":\"em_1\"}}}"),
        json("{\"object\":\"list\",\"has_more\":false,\"data\":[{\"id\":\"at_1\",\"http_status_code\":0,"
            + "\"response\":\"timeout\",\"sent_at\":\"2026-10-30T10:00:01.000Z\"}]}"),
        json("{\"object\":\"webhook_event\",\"id\":\"ev_1\"}"))) {
      AgdirEmail agdir = api.client();
      WebhookWithSecret hook = agdir.webhooks().get("wh_1");
      assertEquals(WebhookStatus.ENABLED, hook.getStatus());
      assertEquals(List.of(WebhookEventType.SENT), hook.getEvents());
      assertEquals("whsec_x", hook.getSigningSecret());

      Page<WebhookDelivery> deliveries = agdir.webhooks().deliveries().list("wh_1", ListOptions.limit(5));
      assertEquals("limit=5", api.request(1).query);
      assertEquals(WebhookDeliveryStatus.FAILED, deliveries.getData().get(0).getStatus());
      assertFalse(deliveries.hasMore());
      assertNull(deliveries.getNextCursor());

      WebhookDeliveryDetail detail = agdir.webhooks().deliveries().get("wh_1", "ev_1");
      assertEquals(Instant.parse("2026-10-30T10:05:00Z"), detail.getNextAttemptAt());
      assertEquals("email.sent", detail.getPayload().get("type"));
      assertEquals("{\"type\":\"email.sent\",\"data\":{\"email_id\":\"em_1\"}}", detail.getPayloadJson());

      WebhookAttempt attempt = agdir.webhooks().deliveries().attempts("wh_1", "ev_1").getData().get(0);
      assertEquals(0, attempt.getHttpStatus());
      assertEquals("timeout", attempt.getResponse());
      assertEquals(Instant.parse("2026-10-30T10:00:01Z"), attempt.getSentAt());
      assertEquals("/webhooks/wh_1/events/ev_1/attempts", api.request(3).path);

      assertEquals("ev_1", agdir.webhooks().deliveries().replay("wh_1", "ev_1"));
      assertEquals("POST /webhooks/wh_1/events/ev_1/replay", api.request(4).method + " " + api.request(4).path);
    }
  }

  @Test
  void escapesIdsInPaths() throws Exception {
    try (StubServer api = new StubServer(json("{\"id\":\"x\"}"))) {
      api.client().emails().cancel("a/b?c d");
      assertEquals("/emails/a%2Fb%3Fc%20d/cancel", api.request(0).path);
    }
  }

  @Test
  void returnsPagesWithACursorAndWalksAllOfThemLazily() throws Exception {
    String hook = "{\"id\":\"%s\",\"endpoint\":\"https://acme.com/h\",\"status\":\"enabled\","
        + "\"events\":[\"email.sent\"],\"created_at\":\"2026-10-30T10:00:00.000Z\"}";
    try (StubServer api = new StubServer(
        json("{\"object\":\"list\",\"has_more\":true,\"data\":[" + String.format(hook, "w1") + ","
            + String.format(hook, "w2") + "]}"),
        json("{\"object\":\"list\",\"has_more\":false,\"data\":[" + String.format(hook, "w3") + "]}"))) {
      PagedIterable<Webhook> all = api.client().webhooks().listAll(ListOptions.limit(2));
      assertTrue(api.requests.isEmpty(), "listAll must not fetch before iteration");

      List<String> ids = new ArrayList<>();
      for (Webhook w : all) ids.add(w.getId());

      assertEquals(List.of("w1", "w2", "w3"), ids);
      assertEquals("limit=2", api.request(0).query);
      assertEquals("limit=2&after=w2", api.request(1).query);
    }
  }

  @Test
  void streamStopsFetchingOnceItHasEnough() throws Exception {
    String domain = "{\"id\":\"%s\",\"name\":\"%s.com\",\"status\":\"verified\",\"region\":\"eu-north-1\","
        + "\"created_at\":\"2026-10-30T10:00:00.000Z\"}";
    try (StubServer api = new StubServer(json("{\"object\":\"list\",\"has_more\":true,\"data\":["
        + String.format(domain, "d1", "a") + "," + String.format(domain, "d2", "b") + "]}"))) {
      List<String> names = api.client().domains().listAll().stream().limit(1)
          .map(Domain::getName).collect(Collectors.toList());
      assertEquals(List.of("a.com"), names);
      assertEquals(1, api.requests.size());
      assertNull(api.request(0).query);
    }
  }

  @Test
  void throwsAgdirEmailExceptionWithTheApiCodeStatusAndMessage() throws Exception {
    try (StubServer api = new StubServer(
        status(422, "{\"statusCode\":422,\"name\":\"validation_error\",\"message\":\"to: Invalid email address\"}"))) {
      AgdirEmailException e = assertThrows(AgdirEmailException.class, () -> api.client().emails().send(simple()));
      assertEquals("validation_error", e.getCode());
      assertEquals(422, e.getStatus());
      assertEquals("to: Invalid email address", e.getMessage());
      assertNull(e.getRetryAfter());
    }
  }

  @Test
  void nonJsonErrorBodiesFallBackToTheStatus() throws Exception {
    try (StubServer api = new StubServer(status(502, "<html>Bad gateway</html>"))) {
      AgdirEmailException e = assertThrows(AgdirEmailException.class, () -> api.client(0).domains().get("d_1"));
      assertEquals("Request failed with HTTP 502", e.getMessage());
      assertEquals("internal_server_error", e.getCode());
      assertEquals(502, e.getStatus());
    }
  }

  @Test
  void retriesRateLimitsAfterRetryAfterReusingTheIdempotencyKey() throws Exception {
    try (StubServer api = new StubServer(
        status(429, "{\"name\":\"rate_limit_exceeded\",\"message\":\"slow down\"}", Map.of("retry-after", "0")),
        json("{\"id\":\"em_1\"}"))) {
      assertEquals("em_1", api.client().emails().send(simple()).getId());
      assertEquals(2, api.requests.size());
      assertEquals(api.request(0).header("Idempotency-Key"), api.request(1).header("Idempotency-Key"));
    }
  }

  @Test
  void exposesRetryAfterWhenRetriesRunOut() throws Exception {
    try (StubServer api = new StubServer(
        status(429, "{\"name\":\"rate_limit_exceeded\",\"message\":\"slow down\"}", Map.of("retry-after", "2")))) {
      AgdirEmailException e = assertThrows(AgdirEmailException.class, () -> api.client(0).domains().list());
      assertEquals(AgdirEmailException.RATE_LIMIT_EXCEEDED, e.getCode());
      assertEquals(Duration.ofSeconds(2), e.getRetryAfter());
    }
  }

  @Test
  void doesNotRetryQuotaErrors() throws Exception {
    try (StubServer api = new StubServer(status(429, "{\"name\":\"daily_quota_exceeded\",\"message\":\"quota\"}"))) {
      AgdirEmailException e = assertThrows(AgdirEmailException.class, () -> api.client().emails().send(simple()));
      assertEquals("daily_quota_exceeded", e.getCode());
      assertEquals(1, api.requests.size());
    }
  }

  @Test
  void retriesGetServerErrorsThenGivesUpAfterMaxRetries() throws Exception {
    StubServer.Reply unavailable = status(503, "{}", Map.of("retry-after", "0"));
    try (StubServer api = new StubServer(unavailable, unavailable, unavailable)) {
      AgdirEmailException e = assertThrows(AgdirEmailException.class, () -> api.client().domains().get("d_1"));
      assertEquals(3, api.requests.size());
      assertEquals(503, e.getStatus());
      assertEquals("internal_server_error", e.getCode());
    }
  }

  @Test
  void retriesGetNetworkFailures() throws Exception {
    try (StubServer api = new StubServer(drop(), json("{\"object\":\"domain\",\"id\":\"d_1\",\"records\":[]}"))) {
      assertEquals("d_1", api.client().domains().get("d_1").getId());
      assertEquals(2, api.requests.size());
    }
  }

  @Test
  void doesNotRetryANonIdempotentPostAfterANetworkFailure() throws Exception {
    try (StubServer api = new StubServer(drop())) {
      AgdirEmailException e = assertThrows(AgdirEmailException.class,
          () -> api.client().webhooks().create("https://acme.com/h", List.of(WebhookEventType.SENT)));
      assertEquals(1, api.requests.size());
      assertEquals("network_error", e.getCode());
      assertEquals(0, e.getStatus());
      assertNotNull(e.getCause());
    }
  }

  @Test
  void doesNotRetryServerErrorsOnNonIdempotentPosts() throws Exception {
    try (StubServer api = new StubServer(status(500, "{\"name\":\"internal_server_error\",\"message\":\"x\"}"))) {
      assertThrows(AgdirEmailException.class, () -> api.client().domains().create("acme.com"));
      assertEquals(1, api.requests.size());
    }
  }

  @Test
  void reportsTimeouts() throws Exception {
    try (StubServer api = new StubServer(slow(1500, "{\"id\":\"x\"}"))) {
      AgdirEmail agdir = AgdirEmail.builder().apiKey("re_test").baseUrl(api.url()).maxRetries(0)
          .timeout(Duration.ofMillis(200)).env(n -> null).build();
      AgdirEmailException e = assertThrows(AgdirEmailException.class, () -> agdir.emails().send(simple()));
      assertEquals("timeout", e.getCode());
      assertEquals(0, e.getStatus());
    }
  }

  @Test
  void asyncVariantsCompleteWithResultsAndFailWithAgdirEmailException() throws Exception {
    try (StubServer api = new StubServer(json("{\"id\":\"em_1\"}"),
        status(404, "{\"name\":\"not_found\",\"message\":\"Email not found\"}"))) {
      AgdirEmail agdir = api.client();
      assertEquals("em_1", agdir.emails().sendAsync(simple()).get().getId());

      CompletableFuture<Email> missing = agdir.emails().getAsync("em_x");
      ExecutionException e = assertThrows(ExecutionException.class, missing::get);
      AgdirEmailException cause = assertInstanceOf(AgdirEmailException.class, e.getCause());
      assertEquals("not_found", cause.getCode());
      assertEquals(404, cause.getStatus());
    }
  }

  @Test
  void cancellingAnAsyncCallStopsRetries() throws Exception {
    try (StubServer api = new StubServer(
        status(503, "{}", Map.of("retry-after", "1")), json("{\"object\":\"domain\",\"id\":\"d_1\"}"))) {
      CompletableFuture<DomainWithRecords> f = api.client().domains().getAsync("d_1");
      Thread.sleep(300); // first attempt answered, waiting 1 s to retry
      assertTrue(f.cancel(true));
      Thread.sleep(1200);
      assertEquals(1, api.requests.size());
    }
  }
}
