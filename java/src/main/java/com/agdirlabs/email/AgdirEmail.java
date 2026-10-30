package com.agdirlabs.email;

import java.net.http.HttpClient;
import java.time.Duration;
import java.util.Objects;
import java.util.function.Function;

/**
 * Agdir Email API client. Thread-safe; create one and share it.
 *
 * <pre>{@code
 * AgdirEmail agdir = AgdirEmail.builder().apiKey("re_…").build();   // or AgdirEmail.fromEnv()
 * SendResult sent = agdir.emails().send(SendEmailRequest.builder()
 *     .from("Acme <hello@acme.com>")
 *     .to("user@example.com")
 *     .subject("Welcome")
 *     .html("<p>Thanks for signing up!</p>")
 *     .build());
 * }</pre>
 */
public final class AgdirEmail {
  /** This SDK's version. */
  public static final String VERSION = "0.1.0";
  /** Sent as the {@code User-Agent} header. */
  public static final String USER_AGENT = "agdirlabs-email-java/" + VERSION;
  public static final String DEFAULT_BASE_URL = "https://api.agdir.email";
  public static final String API_KEY_ENV = "AGDIR_EMAIL_API_KEY";
  public static final String BASE_URL_ENV = "AGDIR_EMAIL_BASE_URL";

  private final String baseUrl;
  private final Emails emails;
  private final Domains domains;
  private final Webhooks webhooks;

  private AgdirEmail(Builder b) {
    String apiKey = firstNonEmpty(b.apiKey, b.env.apply(API_KEY_ENV));
    if (apiKey == null) {
      throw new IllegalStateException(
          "Missing API key: call apiKey(...) or set " + API_KEY_ENV + ". Create one at https://app.agdir.email.");
    }
    String url = firstNonEmpty(b.baseUrl, b.env.apply(BASE_URL_ENV));
    this.baseUrl = url != null ? url : DEFAULT_BASE_URL;
    HttpClient client =
        b.httpClient != null
            ? b.httpClient
            : HttpClient.newBuilder()
                .connectTimeout(b.timeout.compareTo(Duration.ofSeconds(10)) < 0 ? b.timeout : Duration.ofSeconds(10))
                .followRedirects(HttpClient.Redirect.NEVER)
                .build();
    Http http = new Http(apiKey, baseUrl, client, b.maxRetries, b.timeout);
    this.emails = new Emails(http);
    this.domains = new Domains(http);
    this.webhooks = new Webhooks(http);
  }

  public static Builder builder() {
    return new Builder();
  }

  /** A client configured from {@code AGDIR_EMAIL_API_KEY} and (optionally) {@code AGDIR_EMAIL_BASE_URL}. */
  public static AgdirEmail fromEnv() {
    return builder().build();
  }

  public Emails emails() {
    return emails;
  }

  public Domains domains() {
    return domains;
  }

  public Webhooks webhooks() {
    return webhooks;
  }

  public String baseUrl() {
    return baseUrl;
  }

  private static String firstNonEmpty(String a, String b) {
    // Empty values (e.g. `AGDIR_EMAIL_BASE_URL=` in a .env file) count as unset.
    if (a != null && !a.isEmpty()) return a;
    if (b != null && !b.isEmpty()) return b;
    return null;
  }

  /** Configures an {@link AgdirEmail} client. */
  public static final class Builder {
    private String apiKey;
    private String baseUrl;
    private int maxRetries = 2;
    private Duration timeout = Duration.ofSeconds(30);
    private HttpClient httpClient;
    private Function<String, String> env = System::getenv;

    private Builder() {}

    /** Defaults to the {@code AGDIR_EMAIL_API_KEY} environment variable. */
    public Builder apiKey(String apiKey) {
      this.apiKey = apiKey;
      return this;
    }

    /** Defaults to {@code AGDIR_EMAIL_BASE_URL}, else {@code https://api.agdir.email}. */
    public Builder baseUrl(String baseUrl) {
      this.baseUrl = baseUrl;
      return this;
    }

    /** Retries for rate limits, server errors and network failures. Default 2; 0 disables. */
    public Builder maxRetries(int maxRetries) {
      if (maxRetries < 0) throw new IllegalArgumentException("maxRetries must be a non-negative integer.");
      this.maxRetries = maxRetries;
      return this;
    }

    /** Per-attempt timeout. Default 30 seconds. */
    public Builder timeout(Duration timeout) {
      Objects.requireNonNull(timeout, "timeout");
      if (timeout.isNegative() || timeout.isZero()) throw new IllegalArgumentException("timeout must be positive");
      this.timeout = timeout;
      return this;
    }

    /** A custom {@link HttpClient} (proxy, SSL context, executor). The client's own timeouts still apply. */
    public Builder httpClient(HttpClient httpClient) {
      this.httpClient = httpClient;
      return this;
    }

    /** Environment lookup, for tests. */
    Builder env(Function<String, String> env) {
      this.env = env;
      return this;
    }

    /** @throws IllegalStateException if no API key was given or found in the environment */
    public AgdirEmail build() {
      return new AgdirEmail(this);
    }
  }
}
