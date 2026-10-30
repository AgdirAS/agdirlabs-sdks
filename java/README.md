# Agdir Email for Java

[![Maven Central](https://img.shields.io/maven-central/v/com.agdirlabs/agdir-email.svg)](https://central.sonatype.com/artifact/com.agdirlabs/agdir-email)
[![CI](https://github.com/AgdirAS/agdirlabs-sdks/actions/workflows/java.yml/badge.svg)](https://github.com/AgdirAS/agdirlabs-sdks/actions/workflows/java.yml)

The official Java SDK for [Agdir Email](https://agdir.email): programmatic SMTP for developers, hosted in the EU. Built by [Agdir Labs](https://agdirlabs.com) at [Agdir](https://agdir.no).

- Java 11+, works from Kotlin, Scala and other JVM languages
- **No runtime dependencies**: uses `java.net.http.HttpClient` and a small built-in JSON codec
- Immutable request builders and response objects, `java.time.Instant` timestamps
- Blocking calls plus `CompletableFuture` variants (`sendAsync`, `getAsync`, …)
- Safe automatic retries with idempotency keys
- Lazy cursor pagination as an `Iterable` / `Stream`
- Webhook signature verification built in

## Install

Maven:

```xml
<dependency>
  <groupId>com.agdirlabs</groupId>
  <artifactId>agdir-email</artifactId>
  <version>0.1.0</version>
</dependency>
```

Gradle (Kotlin DSL):

```kotlin
implementation("com.agdirlabs:agdir-email:0.1.0")
```

## Quick start

Create an API key at [app.agdir.email](https://app.agdir.email) and verify a sending domain, then:

```java
import com.agdirlabs.email.*;

AgdirEmail agdir = AgdirEmail.builder().apiKey("re_…").build();

SendResult sent = agdir.emails().send(SendEmailRequest.builder()
    .from("Acme <hello@acme.com>")
    .to("user@example.com")
    .subject("Welcome to Acme")
    .html("<p>Thanks for signing up!</p>")
    .build());

System.out.println(sent.getId());
```

`AgdirEmail.fromEnv()` reads `AGDIR_EMAIL_API_KEY` (and `AGDIR_EMAIL_BASE_URL`, if set) from the environment. The client is thread-safe: create one and share it.

Kotlin:

```kotlin
val agdir = AgdirEmail.fromEnv()
val id = agdir.emails().send(
    SendEmailRequest.builder()
        .from("Acme <hello@acme.com>")
        .to("user@example.com")
        .subject("Welcome to Acme")
        .text("Thanks for signing up!")
        .build()
).id
```

## Emails

```java
// Plain text, several recipients, reply-to, tags, custom headers
agdir.emails().send(SendEmailRequest.builder()
    .from("Acme <billing@acme.com>")
    .to("ana@example.com", "ben@example.com")
    .cc("finance@acme.com")
    .replyTo("support@acme.com")
    .subject("Your invoice")
    .text("Invoice #1042 is attached in your dashboard.")
    .tag("kind", "invoice")
    .header("X-Entity-Ref-ID", "inv-1042")
    .build());

// Schedule for later (Instant, OffsetDateTime or an ISO 8601 string), then cancel if plans change
String id = agdir.emails().send(email.toBuilder()
    .scheduledAt(Instant.now().plus(1, ChronoUnit.HOURS))
    .build()).getId();
agdir.emails().cancel(id);

// Delivery status
Email e = agdir.emails().get(id);
e.getStatus(); // EmailStatus.QUEUED, SCHEDULED, SENT, DELIVERED, BOUNCED, COMPLAINED, CANCELED, …

// Up to 100 emails per request (all-or-nothing by default)
BatchResult batch = agdir.emails().sendBatch(List.of(email1, email2));

// Accept the valid ones, report the rest
BatchResult result = agdir.emails().sendBatch(emails, SendOptions.permissive());
for (BatchResult.BatchError err : result.getErrors()) {
  System.out.println(err.getIndex() + ": " + err.getMessage());
}
```

### Async

Every call has a `CompletableFuture` variant. Failures complete the future exceptionally with an `AgdirEmailException`; cancelling the future stops any pending retries.

```java
agdir.emails().sendAsync(email)
    .thenAccept(r -> log.info("accepted {}", r.getId()))
    .exceptionally(err -> { log.error("send failed", err); return null; });
```

## Domains

```java
DomainWithRecords domain = agdir.domains().create("acme.com");
for (DnsRecord r : domain.getRecords()) {
  System.out.println(r.getType() + " " + r.getName() + " " + r.getValue()); // add these to your DNS
}

agdir.domains().verify(domain.getId()); // check DNS now instead of waiting
agdir.domains().get(domain.getId()).getStatus(); // PENDING → VERIFIED

// Every domain, fetching pages lazily
for (Domain d : agdir.domains().listAll()) System.out.println(d.getName() + " " + d.getStatus());
agdir.domains().listAll().stream().filter(d -> d.getStatus() == DomainStatus.FAILED).forEach(...);

// Or one page at a time
Page<Domain> page = agdir.domains().list(ListOptions.limit(20));
if (page.hasMore()) page = agdir.domains().list(ListOptions.limit(20).after(page.getNextCursor()));

agdir.domains().delete(domain.getId());
```

## Webhooks

```java
WebhookSecret hook = agdir.webhooks().create("https://acme.com/webhooks/email",
    List.of(WebhookEventType.DELIVERED, WebhookEventType.BOUNCED, WebhookEventType.COMPLAINED));
hook.getSigningSecret(); // whsec_… — store it to verify incoming events

agdir.webhooks().update(hook.getId(), UpdateWebhookRequest.builder().status(WebhookStatus.DISABLED).build());
agdir.webhooks().rotateSecret(hook.getId());

// What was sent to your endpoint, and each HTTP attempt
Page<WebhookDelivery> deliveries = agdir.webhooks().deliveries().list(hook.getId());
String deliveryId = deliveries.getData().get(0).getId();
Page<WebhookAttempt> attempts = agdir.webhooks().deliveries().attempts(hook.getId(), deliveryId);
agdir.webhooks().deliveries().replay(hook.getId(), deliveryId);
```

### Verifying incoming events

Every request to your endpoint is signed. Verify it against the **raw** body before trusting it. `Webhooks.verify` takes the body, the headers (a `Map` with string or list values, or a lookup function) and the signing secret, and returns the parsed `WebhookEvent` or throws `WebhookVerificationException`.

Spring Boot:

```java
@PostMapping("/webhooks/email")
ResponseEntity<Void> email(@RequestBody String payload, @RequestHeader HttpHeaders headers) {
  WebhookEvent event;
  try {
    event = Webhooks.verify(payload, headers::getFirst, webhookSecret);
  } catch (WebhookVerificationException e) {
    return ResponseEntity.badRequest().build();
  }
  if (event.getType() == WebhookEventType.BOUNCED) {
    markBounced(event.getData().getTo(), event.getData().getBounce());
  }
  return ResponseEntity.noContent().build();
}
```

Jakarta Servlet:

```java
String payload = new String(request.getInputStream().readAllBytes(), StandardCharsets.UTF_8);
WebhookEvent event = Webhooks.verify(payload, request::getHeader, secret);
```

Messages older than 5 minutes are rejected (pass a `Duration` tolerance to change it). Failed deliveries are retried for about 18 hours; use `event.getData().getEmailId()` to deduplicate.

## Errors

Every failed call throws an unchecked `AgdirEmailException`:

```java
try {
  agdir.emails().send(email);
} catch (AgdirEmailException e) {
  e.getCode();       // "validation_error", "daily_quota_exceeded", "rate_limit_exceeded", "network_error", "timeout", …
  e.getStatus();     // HTTP status, 0 if no response
  e.getMessage();    // human-readable explanation
  e.getRetryAfter(); // Duration from the retry-after header, or null
}
```

The codes are available as constants, e.g. `AgdirEmailException.VALIDATION_ERROR`. Response enums (`EmailStatus`, `DomainStatus`, …) map values added to the API after this SDK version to `UNKNOWN` instead of failing.

## Retries and idempotency

Rate limits (`429 rate_limit_exceeded`) are always retried, honouring `retry-after`. Server errors (408/500/502/503/504) and network failures are retried for requests that are safe to repeat: reads, sends that carry an idempotency key, webhook updates, domain verification and email cancellation. The SDK retries up to `maxRetries` times (default 2) with exponential backoff.

Sends are only retried with an idempotency key, so a retry can never deliver an email twice: the SDK generates one per call, or you can pass your own to make sends safe across process restarts too:

```java
agdir.emails().send(email, SendOptions.idempotencyKey("welcome/" + user.getId()));
```

Quota errors (`daily_quota_exceeded`, `monthly_quota_exceeded`) are not retried.

## Configuration

```java
AgdirEmail agdir = AgdirEmail.builder()
    .apiKey("re_…")                        // default: AGDIR_EMAIL_API_KEY
    .baseUrl("https://api.agdir.email")    // default: AGDIR_EMAIL_BASE_URL, else this
    .maxRetries(2)                         // 0 disables retries
    .timeout(Duration.ofSeconds(30))       // per attempt
    .httpClient(HttpClient.newBuilder()    // e.g. for a proxy or custom executor
        .proxy(ProxySelector.of(new InetSocketAddress("proxy.internal", 3128)))
        .build())
    .build();
```

Blocking calls are interruptible: interrupting the waiting thread cancels the request and throws `CancellationException` with the interrupt flag set.

## Why no dependencies?

The SDK needs a JSON codec for a handful of small, fixed-shape objects. Bundling Jackson would add ~2 MB and, more importantly, pull a specific Jackson version into your dependency tree, where it can clash with the one your framework already uses. A ~300-line internal parser/writer covers the API's needs and keeps the SDK conflict-free. Responses are mapped to typed classes, so you never touch the internal JSON. Webhook delivery payloads are exposed both as a `Map` and as a JSON string (`getPayloadJson()`) for your own JSON library.

## Development

No local Java? Run the tests in Docker:

```bash
cd java
docker run --rm -v "$PWD":/src -w /src -v agdir-m2:/root/.m2 maven:3.9-eclipse-temurin-21 ./mvnw -B verify
```

The opt-in live test runs against a real (non-production) API when these are set:

```bash
docker run --rm -v "$PWD":/src -w /src -v agdir-m2:/root/.m2 \
  -e AGDIR_EMAIL_LIVE_BASE_URL=http://host.docker.internal:<port>/<api path> \
  -e AGDIR_EMAIL_LIVE_API_KEY=re_… \
  -e AGDIR_EMAIL_LIVE_FROM=hi@your-verified-domain.example \
  maven:3.9-eclipse-temurin-21 ./mvnw -B test -Dtest=LiveTest
```

### Future work

- Spring Boot auto-configuration (`agdir-email-spring-boot-starter`): an `AgdirEmail` bean from `agdir.email.*` properties and a webhook verification helper.

## Releasing

Releases go to Maven Central through the [Sonatype Central Portal](https://central.sonatype.com) from `.github/workflows/java-release.yml`. Maven Central has no OIDC trusted publishing, so the workflow uses a Portal token and a GPG key stored as repository secrets.

**One-time setup (owner):**

1. Sign in at [central.sonatype.com](https://central.sonatype.com) with the Agdir account and add the namespace `com.agdirlabs` (*Namespaces → Add Namespace*). The Portal shows a verification key: publish it as a DNS TXT record on the apex of `agdirlabs.com`, e.g. `agdirlabs.com. TXT "<verification key>"`, then click *Verify*. The record can be removed once the namespace is verified.
2. Create a user token (*Account → Generate User Token*) and store its two parts as repository secrets `CENTRAL_TOKEN_USERNAME` and `CENTRAL_TOKEN_PASSWORD`.
3. Create a signing key and publish its public half:
   ```bash
   gpg --quick-generate-key "Agdir Labs <hi@agdirlabs.com>" ed25519 sign 3y
   gpg --keyserver keys.openpgp.org --send-keys <KEY_ID>
   gpg --keyserver keyserver.ubuntu.com --send-keys <KEY_ID>
   gpg --armor --export-secret-keys <KEY_ID>   # → secret GPG_PRIVATE_KEY
   ```
   Store the armored private key as `GPG_PRIVATE_KEY` and its passphrase as `GPG_PASSPHRASE`.

**Each release:**

1. Set the version in `pom.xml` and `AgdirEmail.VERSION` (a unit test checks they match), and move the `CHANGELOG.md` entries under the new version.
2. Merge to `main`, then tag and push: `git tag java/v0.1.0 && git push origin java/v0.1.0`.
3. The workflow checks the tag against `pom.xml`, runs the tests, signs the jar, sources and javadoc, and publishes with `./mvnw -P release deploy` (it waits until the release is live on Central).

## Contributing

Issues and pull requests are welcome; see [CONTRIBUTING.md](../CONTRIBUTING.md). Changes are listed in [CHANGELOG.md](CHANGELOG.md).

## License

[MIT](LICENSE) © [Agdir AS](https://agdir.no)
