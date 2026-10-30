package com.agdirlabs.email;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.nio.charset.StandardCharsets;
import java.security.SecureRandom;
import java.time.Duration;
import java.time.Instant;
import java.util.Base64;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import org.junit.jupiter.api.Test;

class WebhookVerifyTest {
  private static final Instant NOW = Instant.parse("2026-10-30T12:00:00Z");
  private static final String SECRET = newSecret();
  private static final String PAYLOAD = "{\"type\":\"email.bounced\",\"created_at\":\"2026-10-30T11:59:58.000Z\","
      + "\"data\":{\"created_at\":\"2026-10-30T11:59:00.000Z\",\"email_id\":\"em_1\",\"message_id\":\"ses-1\","
      + "\"from\":\"Acme <hello@acme.com>\",\"to\":[\"user@example.com\"],\"subject\":\"Hi\","
      + "\"tags\":{\"kind\":\"welcome\"},"
      + "\"bounce\":{\"message\":\"mailbox full\",\"subType\":\"MailboxFull\",\"type\":\"Transient\"}}}";

  private static String newSecret() {
    byte[] key = new byte[32];
    new SecureRandom().nextBytes(key);
    return "whsec_" + Base64.getEncoder().encodeToString(key);
  }

  private static Map<String, String> sign(String body, String secret, Instant at) throws Exception {
    String id = "msg_1";
    String ts = Long.toString(at.getEpochSecond());
    Mac mac = Mac.getInstance("HmacSHA256");
    mac.init(new SecretKeySpec(Base64.getDecoder().decode(secret.substring(6)), "HmacSHA256"));
    String sig = Base64.getEncoder()
        .encodeToString(mac.doFinal((id + "." + ts + "." + body).getBytes(StandardCharsets.UTF_8)));
    Map<String, String> headers = new HashMap<>();
    headers.put("webhook-id", id);
    headers.put("webhook-timestamp", ts);
    headers.put("webhook-signature", "v1," + sig);
    return headers;
  }

  private static WebhookEvent verify(String payload, Map<String, ?> headers, String secret) {
    return Webhooks.verify(payload, headers, secret, Webhooks.DEFAULT_TOLERANCE, NOW);
  }

  @Test
  void acceptsAValidSignatureAndReturnsTheEvent() throws Exception {
    WebhookEvent event = verify(PAYLOAD, sign(PAYLOAD, SECRET, NOW), SECRET);
    assertEquals(WebhookEventType.BOUNCED, event.getType());
    assertEquals("email.bounced", event.getRawType());
    assertEquals(Instant.parse("2026-10-30T11:59:58Z"), event.getCreatedAt());
    WebhookEvent.Data d = event.getData();
    assertEquals("em_1", d.getEmailId());
    assertEquals("ses-1", d.getMessageId());
    assertEquals("Acme <hello@acme.com>", d.getFrom());
    assertEquals(List.of("user@example.com"), d.getTo());
    assertEquals("Hi", d.getSubject());
    assertEquals(Map.of("kind", "welcome"), d.getTags());
    assertEquals(Instant.parse("2026-10-30T11:59:00Z"), d.getCreatedAt());
    assertEquals("Transient", d.getBounce().getType());
    assertEquals("MailboxFull", d.getBounce().getSubType());
    assertEquals("mailbox full", d.getBounce().getMessage());
    assertNull(d.getClick());
    assertNull(d.getFailed());
  }

  @Test
  void acceptsCaseInsensitiveListHeadersAndOneOfSeveralSignatures() throws Exception {
    Map<String, String> signed = sign(PAYLOAD, SECRET, NOW);
    Map<String, List<String>> headers = new HashMap<>();
    headers.put("Webhook-Id", List.of(signed.get("webhook-id")));
    headers.put("Webhook-Timestamp", List.of(signed.get("webhook-timestamp")));
    headers.put("Webhook-Signature", List.of("v1,AAAA " + signed.get("webhook-signature")));
    assertEquals(WebhookEventType.BOUNCED, verify(PAYLOAD, headers, SECRET).getType());
  }

  @Test
  void acceptsAHeaderLookupFunction() throws Exception {
    Map<String, String> signed = sign(PAYLOAD, SECRET, NOW);
    WebhookEvent event = Webhooks.verify(PAYLOAD, signed::get, SECRET, Duration.ofMinutes(5), NOW);
    assertEquals("em_1", event.getData().getEmailId());
  }

  @Test
  void rejectsAModifiedBody() throws Exception {
    Map<String, String> headers = sign(PAYLOAD, SECRET, NOW);
    WebhookVerificationException e = assertThrows(WebhookVerificationException.class,
        () -> verify(PAYLOAD.replace("Hi", "Ho"), headers, SECRET));
    assertTrue(e.getMessage().contains("does not match"));
  }

  @Test
  void rejectsAWrongSecret() throws Exception {
    Map<String, String> headers = sign(PAYLOAD, SECRET, NOW);
    WebhookVerificationException e =
        assertThrows(WebhookVerificationException.class, () -> verify(PAYLOAD, headers, newSecret()));
    assertTrue(e.getMessage().contains("does not match"));
  }

  @Test
  void rejectsMissingHeaders() {
    WebhookVerificationException e =
        assertThrows(WebhookVerificationException.class, () -> verify(PAYLOAD, Map.of(), SECRET));
    assertTrue(e.getMessage().contains("Missing"));
  }

  @Test
  void rejectsStaleAndFutureTimestamps() throws Exception {
    Map<String, String> stale = sign(PAYLOAD, SECRET, NOW.minusSeconds(301));
    assertThrows(WebhookVerificationException.class, () -> verify(PAYLOAD, stale, SECRET));
    Map<String, String> future = sign(PAYLOAD, SECRET, NOW.plusSeconds(301));
    assertThrows(WebhookVerificationException.class, () -> verify(PAYLOAD, future, SECRET));
    // Within a custom tolerance it passes.
    Webhooks.verify(PAYLOAD, stale, SECRET, Duration.ofMinutes(10), NOW);
  }

  @Test
  void rejectsSignaturesWithoutTheV1Prefix() throws Exception {
    Map<String, String> headers = sign(PAYLOAD, SECRET, NOW);
    headers.put("webhook-signature", headers.get("webhook-signature").substring(3));
    assertThrows(WebhookVerificationException.class, () -> verify(PAYLOAD, headers, SECRET));
  }

  @Test
  void rejectsAnInvalidSecret() throws Exception {
    Map<String, String> headers = sign(PAYLOAD, SECRET, NOW);
    assertThrows(WebhookVerificationException.class, () -> verify(PAYLOAD, headers, "whsec_***"));
  }

  @Test
  void rejectsASignedPayloadThatIsNotJson() throws Exception {
    String body = "not json";
    WebhookVerificationException e = assertThrows(WebhookVerificationException.class,
        () -> verify(body, sign(body, SECRET, NOW), SECRET));
    assertTrue(e.getMessage().contains("not valid JSON"));
  }

  @Test
  void mapsClickAndFailureDetails() throws Exception {
    String body = "{\"type\":\"email.clicked\",\"created_at\":\"2026-10-30T11:59:58.000Z\",\"data\":{"
        + "\"created_at\":\"2026-10-30T11:59:00.000Z\",\"email_id\":\"em_2\",\"from\":\"a@acme.com\","
        + "\"to\":[\"u@example.com\"],\"subject\":\"S\",\"click\":{\"ipAddress\":\"1.2.3.4\","
        + "\"link\":\"https://acme.com\",\"timestamp\":\"2026-10-30T11:59:30.000Z\",\"userAgent\":\"UA\"},"
        + "\"failed\":{\"reason\":\"r\"}}}";
    WebhookEvent event = verify(body, sign(body, SECRET, NOW), SECRET);
    assertEquals(WebhookEventType.CLICKED, event.getType());
    assertNull(event.getData().getMessageId());
    assertEquals(Map.of(), event.getData().getTags());
    assertEquals("https://acme.com", event.getData().getClick().getLink());
    assertEquals("1.2.3.4", event.getData().getClick().getIpAddress());
    assertEquals("UA", event.getData().getClick().getUserAgent());
    assertEquals(Instant.parse("2026-10-30T11:59:30Z"), event.getData().getClick().getTimestamp());
    assertEquals("r", event.getData().getFailed().getReason());
  }
}
