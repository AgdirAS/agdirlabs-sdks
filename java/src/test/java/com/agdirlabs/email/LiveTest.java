package com.agdirlabs.email;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.List;
import java.util.UUID;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.condition.EnabledIfEnvironmentVariable;

/**
 * Runs against a real Agdir Email API. Opt-in: set {@code AGDIR_EMAIL_LIVE_BASE_URL}, {@code AGDIR_EMAIL_LIVE_API_KEY}
 * and optionally {@code AGDIR_EMAIL_LIVE_FROM} (an address on a verified domain). Never point it at production.
 */
@EnabledIfEnvironmentVariable(named = "AGDIR_EMAIL_LIVE_BASE_URL", matches = ".+")
@EnabledIfEnvironmentVariable(named = "AGDIR_EMAIL_LIVE_API_KEY", matches = ".+")
class LiveTest {
  private static AgdirEmail agdir;
  private static String baseUrl;
  private static String from;

  @BeforeAll
  static void setUp() {
    baseUrl = System.getenv("AGDIR_EMAIL_LIVE_BASE_URL");
    String key = System.getenv("AGDIR_EMAIL_LIVE_API_KEY");
    String f = System.getenv("AGDIR_EMAIL_LIVE_FROM");
    from = f == null || f.isEmpty() ? "hi@java.sdk-test.example.com" : f;
    agdir = AgdirEmail.builder().apiKey(key).baseUrl(baseUrl).maxRetries(5).build();
  }

  private static SendEmailRequest.Builder email() {
    return SendEmailRequest.builder()
        .from("Java SDK <" + from + ">")
        .to("delivered@example.com")
        .subject("Java SDK live test")
        .text("Hello from the Java SDK");
  }

  @Test
  void scheduledSendIdempotentReplayAndCancel() {
    SendEmailRequest scheduled = email().scheduledAt(Instant.now().plus(1, ChronoUnit.HOURS)).tag("sdk", "java").build();
    String key = "java-live-" + UUID.randomUUID();

    String id = agdir.emails().send(scheduled, SendOptions.idempotencyKey(key)).getId();
    assertEquals(id, agdir.emails().send(scheduled, SendOptions.idempotencyKey(key)).getId());

    Email got = agdir.emails().get(id);
    assertEquals(EmailStatus.SCHEDULED, got.getStatus());
    assertEquals(List.of("delivered@example.com"), got.getTo());
    assertTrue(got.getScheduledAt().isAfter(Instant.now()));

    assertEquals(id, agdir.emails().cancel(id));
    assertEquals(EmailStatus.CANCELED, agdir.emails().get(id).getStatus());
  }

  @Test
  void permissiveBatchReportsTheBadRecipient() {
    BatchResult result = agdir.emails().sendBatch(
        List.of(email().scheduledAt(Instant.now().plus(1, ChronoUnit.HOURS)).build(),
            email().to("not-an-address").build()),
        SendOptions.permissive());
    assertEquals(1, result.getData().size());
    assertEquals(1, result.getErrors().size());
    assertEquals(1, result.getErrors().get(0).getIndex());
    agdir.emails().cancel(result.getData().get(0).getId());
  }

  @Test
  void validationErrorOnABadRecipient() {
    AgdirEmailException e =
        assertThrows(AgdirEmailException.class, () -> agdir.emails().send(email().to("nope").build()));
    assertEquals(AgdirEmailException.VALIDATION_ERROR, e.getCode());
    assertEquals(422, e.getStatus());
  }

  @Test
  void invalidApiKey() {
    AgdirEmail bad = AgdirEmail.builder().apiKey("re_invalid").baseUrl(baseUrl).build();
    AgdirEmailException e = assertThrows(AgdirEmailException.class, () -> bad.domains().list());
    assertEquals(AgdirEmailException.INVALID_API_KEY, e.getCode());
    assertEquals(403, e.getStatus());
  }

  @Test
  void domainLifecycle() {
    String name = "java-" + System.currentTimeMillis() + ".test";
    DomainWithRecords created = agdir.domains().create(name);
    try {
      assertEquals(name, created.getName());
      assertFalse(created.getRecords().isEmpty());
      assertEquals(created.getId(), agdir.domains().get(created.getId()).getId());
      assertEquals(created.getId(), agdir.domains().verify(created.getId()));
      assertTrue(agdir.domains().listAll(ListOptions.limit(1)).stream().anyMatch(d -> d.getId().equals(created.getId())));
    } finally {
      assertTrue(agdir.domains().delete(created.getId()).isDeleted());
    }
    AgdirEmailException e = assertThrows(AgdirEmailException.class, () -> agdir.domains().get(created.getId()));
    assertEquals(AgdirEmailException.NOT_FOUND, e.getCode());
    assertEquals(404, e.getStatus());
  }

  @Test
  void webhookLifecycle() throws Exception {
    WebhookSecret created = agdir.webhooks()
        .createAsync("https://93.184.216.34/hook", List.of(WebhookEventType.SENT, WebhookEventType.DELIVERED))
        .get();
    try {
      assertTrue(created.getSigningSecret().startsWith("whsec_"));
      WebhookWithSecret hook = agdir.webhooks().get(created.getId());
      assertEquals("https://93.184.216.34/hook", hook.getEndpoint());
      assertEquals(WebhookStatus.ENABLED, hook.getStatus());
      assertEquals(created.getSigningSecret(), hook.getSigningSecret());

      agdir.webhooks().update(created.getId(), UpdateWebhookRequest.builder().status(WebhookStatus.DISABLED).build());
      assertEquals(WebhookStatus.DISABLED, agdir.webhooks().get(created.getId()).getStatus());

      WebhookSecret rotated = agdir.webhooks().rotateSecret(created.getId());
      assertNotEquals(created.getSigningSecret(), rotated.getSigningSecret());

      Page<WebhookDelivery> deliveries = agdir.webhooks().deliveries().list(created.getId());
      assertTrue(deliveries.getData().isEmpty());
      assertFalse(deliveries.hasMore());
    } finally {
      assertTrue(agdir.webhooks().delete(created.getId()).isDeleted());
    }
  }
}
