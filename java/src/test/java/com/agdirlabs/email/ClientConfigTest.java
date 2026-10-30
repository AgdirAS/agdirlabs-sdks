package com.agdirlabs.email;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.io.InputStream;
import java.util.Map;
import java.util.Properties;
import org.junit.jupiter.api.Test;

class ClientConfigTest {
  @Test
  void usesApiAgdirEmailByDefault() {
    AgdirEmail agdir = AgdirEmail.builder().apiKey("re_test").env(Map.of("AGDIR_EMAIL_BASE_URL", "")::get).build();
    assertEquals("https://api.agdir.email", agdir.baseUrl());
    assertEquals(AgdirEmail.DEFAULT_BASE_URL, agdir.baseUrl());
  }

  @Test
  void readsApiKeyAndBaseUrlFromTheEnvironmentAndTreatsEmptyValuesAsUnset() {
    Map<String, String> env = Map.of("AGDIR_EMAIL_API_KEY", "re_env", "AGDIR_EMAIL_BASE_URL", "http://localhost:8080");
    assertEquals("http://localhost:8080", AgdirEmail.builder().env(env::get).build().baseUrl());

    Map<String, String> empty = Map.of("AGDIR_EMAIL_API_KEY", "");
    IllegalStateException e =
        assertThrows(IllegalStateException.class, () -> AgdirEmail.builder().env(empty::get).build());
    assertTrue(e.getMessage().contains("AGDIR_EMAIL_API_KEY"), e.getMessage());
    assertTrue(e.getMessage().contains("https://app.agdir.email"), e.getMessage());
  }

  @Test
  void explicitOptionsWinOverTheEnvironment() {
    Map<String, String> env = Map.of("AGDIR_EMAIL_API_KEY", "re_env", "AGDIR_EMAIL_BASE_URL", "http://env");
    assertEquals("http://explicit", AgdirEmail.builder().baseUrl("http://explicit").env(env::get).build().baseUrl());
  }

  @Test
  void rejectsInvalidOptions() {
    assertThrows(IllegalArgumentException.class, () -> AgdirEmail.builder().maxRetries(-1));
    assertThrows(IllegalArgumentException.class, () -> AgdirEmail.builder().timeout(java.time.Duration.ZERO));
  }

  @Test
  void versionMatchesThePom() throws Exception {
    Properties p = new Properties();
    try (InputStream in = getClass().getResourceAsStream("/agdir-email-test.properties")) {
      p.load(in);
    }
    assertEquals(p.getProperty("version"), AgdirEmail.VERSION);
    assertEquals("agdirlabs-email-java/" + AgdirEmail.VERSION, AgdirEmail.USER_AGENT);
  }

  @Test
  void sendRequestBuilderValidatesRequiredFields() {
    assertThrows(NullPointerException.class, () -> SendEmailRequest.builder().to("a@b.c").subject("s").text("t").build());
    assertThrows(IllegalArgumentException.class,
        () -> SendEmailRequest.builder().from("a@b.c").subject("s").text("t").build());
    assertThrows(IllegalArgumentException.class,
        () -> SendEmailRequest.builder().from("a@b.c").to("x@y.z").subject("s").build());
    assertThrows(IllegalArgumentException.class, () -> ListOptions.limit(0));
    assertThrows(IllegalArgumentException.class, () -> ListOptions.limit(101));
  }
}
