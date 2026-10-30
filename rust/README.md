# agdirlabs-email

[![crates.io](https://img.shields.io/crates/v/agdirlabs-email.svg)](https://crates.io/crates/agdirlabs-email)
[![docs.rs](https://img.shields.io/docsrs/agdirlabs-email)](https://docs.rs/agdirlabs-email)
[![CI](https://github.com/AgdirAS/agdirlabs-sdks/actions/workflows/rust.yml/badge.svg)](https://github.com/AgdirAS/agdirlabs-sdks/actions/workflows/rust.yml)

The official Rust SDK for [Agdir Email](https://agdir.email) — programmatic SMTP for developers, hosted in the EU. Built by [Agdir Labs](https://agdirlabs.com) at [Agdir](https://agdir.no).

- Async (Tokio + `reqwest`, rustls by default — no OpenSSL needed), with an optional blocking client
- Builder-style requests and fully typed responses
- Safe automatic retries with idempotency keys
- Cursor pagination as a `Stream`
- Webhook signature verification built in
- Rust 1.75+

## Install

```bash
cargo add agdirlabs-email
cargo add tokio --features macros,rt-multi-thread
```

or in `Cargo.toml`:

```toml
[dependencies]
agdirlabs-email = "0.1"
tokio = { version = "1", features = ["macros", "rt-multi-thread"] }
```

## Quick start

Create an API key at [app.agdir.email](https://app.agdir.email) and verify a sending domain, then:

```rust,no_run
use agdirlabs_email::{AgdirEmail, SendEmail};

#[tokio::main]
async fn main() -> Result<(), agdirlabs_email::Error> {
    // Reads AGDIR_EMAIL_API_KEY from the environment. Or: AgdirEmail::new("re_...")?
    let agdir = AgdirEmail::from_env()?;

    let sent = agdir
        .emails()
        .send(
            SendEmail::new("Acme <hello@acme.com>", "user@example.com", "Welcome to Acme")
                .html("<p>Thanks for signing up!</p>"),
        )
        .await?;

    println!("Sent email {}", sent.id);
    Ok(())
}
```

```bash
AGDIR_EMAIL_API_KEY=re_... cargo run
```

## Emails

```rust,no_run
use agdirlabs_email::time::{Duration, OffsetDateTime};
use agdirlabs_email::{AgdirEmail, BatchOptions, EmailStatus, SendEmail, SendOptions};

async fn emails(agdir: &AgdirEmail) -> agdirlabs_email::Result<()> {
    // Plain text, several recipients, reply-to, tags, custom headers
    agdir
        .emails()
        .send(
            SendEmail::new("Acme <billing@acme.com>", ["ana@example.com", "ben@example.com"], "Your invoice")
                .text("Invoice #1042 is attached in your dashboard.")
                .cc("finance@acme.com")
                .reply_to("support@acme.com")
                .tag("kind", "invoice")
                .header("X-Entity-Ref-ID", "inv-1042"),
        )
        .await?;

    // Schedule for later, then cancel if plans change
    let email = SendEmail::new("hello@acme.com", "user@example.com", "Reminder")
        .text("Your trial ends tomorrow.")
        .scheduled_at(OffsetDateTime::now_utc() + Duration::hours(1));
    let scheduled = agdir.emails().send(email).await?;
    agdir.emails().cancel(&scheduled.id).await?;

    // Delivery status
    let sent = agdir.emails().get(&scheduled.id).await?;
    if sent.status == EmailStatus::Bounced {
        println!("{} bounced", sent.to.join(", "));
    }

    // Your own idempotency key: safe to retry, even across process restarts
    let welcome = SendEmail::new("hello@acme.com", "user@example.com", "Welcome").text("Hi!");
    agdir
        .emails()
        .send_with(welcome, SendOptions::new().idempotency_key("welcome/user-42"))
        .await?;

    // Up to 100 emails per request; one invalid email rejects the whole batch...
    let batch = vec![
        SendEmail::new("hello@acme.com", "ana@example.com", "Hi Ana").text("Hello!"),
        SendEmail::new("hello@acme.com", "ben@example.com", "Hi Ben").text("Hello!"),
    ];
    let result = agdir.emails().send_batch(batch.clone()).await?;
    println!("{} accepted", result.data.len());

    // ...or accept the valid ones and report the rest
    let result = agdir
        .emails()
        .send_batch_with(batch, BatchOptions::new().permissive())
        .await?;
    for error in result.errors {
        println!("email #{} rejected: {}", error.index, error.message);
    }
    Ok(())
}
```

## Domains

```rust,no_run
use agdirlabs_email::stream::TryStreamExt;
use agdirlabs_email::{AgdirEmail, ListOptions};

async fn domains(agdir: &AgdirEmail) -> agdirlabs_email::Result<()> {
    let domain = agdir.domains().create("acme.com").await?;
    for record in &domain.records {
        // Add these to your DNS
        println!("{} {} {}", record.record_type, record.name, record.value);
    }

    agdir.domains().verify(&domain.id).await?; // check DNS now instead of waiting
    let status = agdir.domains().get(&domain.id).await?.status; // Pending → Verified

    // One page at a time...
    let page = agdir.domains().list(ListOptions::new().limit(20)).await?;
    println!("{} domains, more: {}", page.data.len(), page.has_more);

    // ...or every domain, fetching pages as needed
    let mut all = agdir.domains().list_all(ListOptions::default());
    while let Some(d) = all.try_next().await? {
        println!("{} {:?}", d.name, d.status);
    }

    agdir.domains().delete(&domain.id).await?;
    println!("was {status:?}");
    Ok(())
}
```

`list_all` returns a `futures` `Stream<Item = Result<T>>`. `agdirlabs_email::stream` re-exports `StreamExt`/`TryStreamExt`, so `.try_next()`, `.try_collect::<Vec<_>>()` etc. work without adding `futures` yourself.

## Webhooks

```rust,no_run
use agdirlabs_email::{AgdirEmail, CreateWebhook, ListOptions, UpdateWebhook, WebhookEventType, WebhookStatus};

async fn webhooks(agdir: &AgdirEmail) -> agdirlabs_email::Result<()> {
    let created = agdir
        .webhooks()
        .create(CreateWebhook::new(
            "https://acme.com/webhooks/email",
            [WebhookEventType::Delivered, WebhookEventType::Bounced, WebhookEventType::Complained],
        ))
        .await?;
    // Store created.signing_secret (whsec_...) to verify incoming events

    agdir
        .webhooks()
        .update(&created.id, UpdateWebhook::new().status(WebhookStatus::Disabled))
        .await?;
    agdir.webhooks().rotate_secret(&created.id).await?;

    // What was sent to your endpoint, and each HTTP attempt
    let deliveries = agdir.webhooks().deliveries().list(&created.id, ListOptions::default()).await?;
    if let Some(delivery) = deliveries.data.first() {
        let attempts = agdir
            .webhooks()
            .deliveries()
            .attempts(&created.id, &delivery.id, ListOptions::default())
            .await?;
        println!("{} attempts", attempts.data.len());
        agdir.webhooks().deliveries().replay(&created.id, &delivery.id).await?;
    }

    agdir.webhooks().delete(&created.id).await?;
    Ok(())
}
```

### Verifying incoming events

Every request to your endpoint is signed ([Standard Webhooks](https://www.standardwebhooks.com)). Verify it against the **raw** body before trusting it. `verify_webhook` accepts an `http::HeaderMap` (axum, hyper, warp…), a `HashMap`/`BTreeMap` of headers or a list of `(name, value)` pairs.

With [axum](https://docs.rs/axum):

```rust,ignore
use axum::{body::Bytes, http::{HeaderMap, StatusCode}, routing::post, Router};
use agdirlabs_email::{verify_webhook, WebhookEventType};

async fn email_webhook(headers: HeaderMap, body: Bytes) -> StatusCode {
    let secret = std::env::var("AGDIR_EMAIL_WEBHOOK_SECRET").expect("secret configured");
    // `Bytes`, not `Json<…>`: the signature covers the exact bytes received.
    let event = match verify_webhook(&body, &headers, &secret) {
        Ok(event) => event,
        Err(_) => return StatusCode::BAD_REQUEST,
    };
    if event.event_type == WebhookEventType::Bounced {
        println!("bounced: {:?} {:?}", event.data.to, event.data.bounce);
    }
    StatusCode::NO_CONTENT
}

let app: Router = Router::new().route("/webhooks/email", post(email_webhook));
```

With [actix-web](https://docs.rs/actix-web) (which uses its own header type), pass the headers as pairs:

```rust,ignore
use actix_web::{post, web, HttpRequest, HttpResponse};
use agdirlabs_email::verify_webhook;

#[post("/webhooks/email")]
async fn email_webhook(req: HttpRequest, body: web::Bytes) -> HttpResponse {
    let headers: Vec<(&str, &str)> = req
        .headers()
        .iter()
        .filter_map(|(name, value)| Some((name.as_str(), value.to_str().ok()?)))
        .collect();
    let secret = std::env::var("AGDIR_EMAIL_WEBHOOK_SECRET").unwrap();
    match verify_webhook(&body, &headers, &secret) {
        Ok(event) => {
            println!("{:?} for {}", event.event_type, event.data.email_id);
            HttpResponse::NoContent().finish()
        }
        Err(_) => HttpResponse::BadRequest().finish(),
    }
}
```

Messages older (or newer) than 5 minutes are rejected; use `verify_webhook_with(payload, headers, secret, VerifyOptions { tolerance, .. })` to change that. Failed deliveries are retried for about 18 hours; use `event.data.email_id` to deduplicate. `sign_webhook` builds valid signatures for your own tests.

## Errors

Every failed call returns an `agdirlabs_email::Error`:

```rust,no_run
use agdirlabs_email::{AgdirEmail, Error, ErrorCode, SendEmail};

async fn send(agdir: &AgdirEmail, email: SendEmail) {
    match agdir.emails().send(email).await {
        Ok(sent) => println!("sent {}", sent.id),
        Err(Error::Api { code: ErrorCode::ValidationError, message, .. }) => eprintln!("invalid email: {message}"),
        Err(Error::Api { code: ErrorCode::DailyQuotaExceeded | ErrorCode::MonthlyQuotaExceeded, .. }) => {
            eprintln!("quota used up")
        }
        Err(err) => {
            // Uniform accessors work on every variant
            eprintln!("failed: {err} (code {:?}, HTTP {})", err.code(), err.status());
        }
    }
}
```

| Variant | When | `code()` | `status()` |
| --- | --- | --- | --- |
| `Error::Api { code, status, message, retry_after }` | The API returned an error | the API's code, e.g. `ValidationError`, `NotFound`, `RateLimitExceeded` | HTTP status |
| `Error::Network { source }` | DNS, connection or TLS failure | `NetworkError` | 0 |
| `Error::Timeout { timeout }` | No response within the per-attempt timeout | `Timeout` | 0 |
| `Error::Decode { status, source }` | A 2xx response that isn't the expected JSON | `None` | HTTP status |
| `Error::Config(message)` | Missing API key or invalid base URL | `None` | 0 |

`ErrorCode` is `#[non_exhaustive]`; codes added to the API later arrive as `ErrorCode::Other(name)`.

## Retries and idempotency

Rate limits (`429 rate_limit_exceeded`), server errors (408/500/502/503/504) and network failures are retried up to `max_retries` times (default 2), honouring `retry-after`, else with exponential backoff (about 0.5 s, 1 s, 2 s…, capped at 8 s). Sends are only retried with an idempotency key, so a retry can never deliver an email twice: the SDK generates one per call, or pass your own with `send_with(email, SendOptions::new().idempotency_key(...))` to be safe across process restarts too. Quota errors are never retried.

## Configuration

```rust,no_run
use std::time::Duration;
use agdirlabs_email::AgdirEmail;

fn client() -> agdirlabs_email::Result<AgdirEmail> {
    AgdirEmail::builder()
        .api_key("re_...")                    // default: AGDIR_EMAIL_API_KEY
        .base_url("https://api.agdir.email")  // default: AGDIR_EMAIL_BASE_URL, else this
        .max_retries(2)
        .timeout(Duration::from_secs(30))     // per attempt
        .http_client(agdirlabs_email::reqwest::Client::new()) // optional: proxies, custom TLS, shared pool
        .build()
}
```

`AgdirEmail` is cheap to clone (an `Arc` inside) — create one at startup and share it, e.g. in your web framework's state. To cancel a call, drop its future (for example with `tokio::time::timeout` or `tokio::select!`).

### Cargo features

| Feature | Default | |
| --- | --- | --- |
| `rustls-tls` | yes | TLS with rustls and the Mozilla root certificates |
| `native-tls` | no | TLS with the platform library (OpenSSL / Secure Transport / SChannel); use with `default-features = false` |
| `blocking` | no | A synchronous client in `agdirlabs_email::blocking` |

### Blocking client

```toml
[dependencies]
agdirlabs-email = { version = "0.1", features = ["blocking"] }
```

```rust,ignore
use agdirlabs_email::blocking::AgdirEmail;
use agdirlabs_email::SendEmail;

fn main() -> agdirlabs_email::Result<()> {
    let agdir = AgdirEmail::from_env()?;
    let sent = agdir
        .emails()
        .send(SendEmail::new("hello@acme.com", "user@example.com", "Hi").text("Hello!"))?;
    println!("sent {}", sent.id);

    for domain in agdir.domains().list_all(Default::default()) {
        println!("{}", domain?.name);
    }
    Ok(())
}
```

The blocking client runs its own single-threaded Tokio runtime: don't call it from async code.

## Development

```bash
cargo fmt --check
cargo clippy --all-features --all-targets -- -D warnings
cargo test --all-features
```

The live integration test runs only when these are set, against a real Agdir Email API (never production in CI):

```bash
AGDIR_EMAIL_LIVE_BASE_URL=http://127.0.0.1:5180/<api base path> \
AGDIR_EMAIL_LIVE_API_KEY=re_... \
AGDIR_EMAIL_LIVE_FROM=hi@rust.sdk-test.example.com \
cargo test --all-features --test live -- --nocapture
```

## Releasing

Releases are published to crates.io from GitHub Actions with [Trusted Publishing](https://crates.io/docs/trusted-publishing) (`.github/workflows/rust-release.yml`) — no API token is stored anywhere.

1. Bump `version` in `Cargo.toml`, update `CHANGELOG.md`, merge to `main`.
2. Tag and push: `git tag rust/v0.1.0 && git push origin rust/v0.1.0`. The workflow checks the tag matches `Cargo.toml`, runs the tests and publishes.

One-time setup (crates.io only allows Trusted Publishing for crates that already exist):

1. Publish the first version manually by a crate owner: `cargo login` with a personal token, then `cargo publish` from `rust/`.
2. On crates.io → `agdirlabs-email` → Settings → Trusted Publishing, add GitHub: owner `AgdirAS`, repository `agdirlabs-sdks`, workflow `rust-release.yml`, environment `crates-io`.
3. In GitHub → Settings → Environments, create `crates-io` (optionally with required reviewers). Revoke the personal token.

## Contributing

Issues and pull requests are welcome — see [CONTRIBUTING.md](https://github.com/AgdirAS/agdirlabs-sdks/blob/main/CONTRIBUTING.md). Changes are listed in [CHANGELOG.md](https://github.com/AgdirAS/agdirlabs-sdks/blob/main/rust/CHANGELOG.md).

## License

[MIT](https://github.com/AgdirAS/agdirlabs-sdks/blob/main/rust/LICENSE) © [Agdir AS](https://agdir.no)
