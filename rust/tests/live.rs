//! Opt-in integration test against a real Agdir Email API (never production in CI).
//!
//! Runs only when `AGDIR_EMAIL_LIVE_BASE_URL` and `AGDIR_EMAIL_LIVE_API_KEY` are set;
//! `AGDIR_EMAIL_LIVE_FROM` is a sender on a verified domain of that key's team.

use std::time::{SystemTime, UNIX_EPOCH};

use agdirlabs_email::stream::TryStreamExt;
use agdirlabs_email::time::{Duration, OffsetDateTime};
use agdirlabs_email::{
    AgdirEmail, BatchOptions, CreateWebhook, DomainStatus, EmailStatus, Error, ErrorCode,
    ListOptions, SendEmail, SendOptions, UpdateWebhook, WebhookEventType, WebhookStatus,
};

type TestResult = Result<(), Box<dyn std::error::Error>>;

struct Live {
    base_url: String,
    api_key: String,
    from: String,
}

fn live() -> Option<Live> {
    let var = |name: &str| std::env::var(name).ok().filter(|v| !v.is_empty());
    match (
        var("AGDIR_EMAIL_LIVE_BASE_URL"),
        var("AGDIR_EMAIL_LIVE_API_KEY"),
    ) {
        (Some(base_url), Some(api_key)) => Some(Live {
            base_url,
            api_key,
            from: var("AGDIR_EMAIL_LIVE_FROM")
                .unwrap_or_else(|| "hi@rust.sdk-test.example.com".into()),
        }),
        _ => {
            eprintln!("skipping: set AGDIR_EMAIL_LIVE_BASE_URL and AGDIR_EMAIL_LIVE_API_KEY");
            None
        }
    }
}

fn client(live: &Live) -> AgdirEmail {
    // Tests run in parallel against a 10 req/s team limit: allow extra rate-limit retries.
    AgdirEmail::builder()
        .api_key(&live.api_key)
        .base_url(&live.base_url)
        .max_retries(6)
        .build()
        .unwrap()
}

fn unique() -> String {
    let nanos = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_nanos();
    format!("{nanos:x}")
}

fn email(live: &Live, to: &str) -> SendEmail {
    SendEmail::new(
        format!("Rust SDK <{}>", live.from),
        to,
        "Agdir Email Rust SDK live test",
    )
    .text("Hello from the Rust SDK live test.")
    .html("<p>Hello from the <b>Rust SDK</b> live test.</p>")
    .tag("sdk", "rust")
}

#[tokio::test]
async fn scheduled_send_get_cancel() -> TestResult {
    let Some(live) = live() else { return Ok(()) };
    let agdir = client(&live);

    let at = OffsetDateTime::now_utc() + Duration::hours(1);
    let sent = agdir
        .emails()
        .send(email(&live, "delivered@example.com").scheduled_at(at))
        .await?;

    let fetched = agdir.emails().get(&sent.id).await?;
    assert_eq!(fetched.id, sent.id);
    assert_eq!(fetched.status, EmailStatus::Scheduled);
    assert_eq!(fetched.to, ["delivered@example.com"]);
    assert!(fetched.cc.is_empty() && fetched.reply_to.is_empty());
    assert_eq!(
        fetched.scheduled_at.map(|t| t.unix_timestamp()),
        Some(at.unix_timestamp())
    );

    assert_eq!(agdir.emails().cancel(&sent.id).await?.id, sent.id);
    assert_eq!(
        agdir.emails().get(&sent.id).await?.status,
        EmailStatus::Canceled
    );
    Ok(())
}

#[tokio::test]
async fn idempotent_replay_returns_the_same_id() -> TestResult {
    let Some(live) = live() else { return Ok(()) };
    let agdir = client(&live);
    let key = format!("rust-live-{}", unique());

    let first = agdir
        .emails()
        .send_with(
            email(&live, "delivered@example.com"),
            SendOptions::new().idempotency_key(&key),
        )
        .await?;
    let second = agdir
        .emails()
        .send_with(
            email(&live, "delivered@example.com"),
            SendOptions::new().idempotency_key(&key),
        )
        .await?;
    assert_eq!(first.id, second.id);
    Ok(())
}

#[tokio::test]
async fn permissive_batch_reports_the_bad_recipient() -> TestResult {
    let Some(live) = live() else { return Ok(()) };
    let agdir = client(&live);

    let result = agdir
        .emails()
        .send_batch_with(
            vec![
                email(&live, "delivered@example.com"),
                email(&live, "not-an-email"),
            ],
            BatchOptions::new().permissive(),
        )
        .await?;
    assert_eq!(result.data.len(), 1);
    assert_eq!(result.errors.len(), 1);
    assert_eq!(result.errors[0].index, 1);
    Ok(())
}

#[tokio::test]
async fn validation_and_auth_errors() -> TestResult {
    let Some(live) = live() else { return Ok(()) };
    let agdir = client(&live);

    let err = agdir
        .emails()
        .send(email(&live, "not-an-email"))
        .await
        .unwrap_err();
    assert!(
        matches!(
            &err,
            Error::Api {
                code: ErrorCode::ValidationError,
                status: 422,
                ..
            }
        ),
        "{err:?}"
    );

    let invalid = AgdirEmail::builder()
        .api_key("re_invalid")
        .base_url(&live.base_url)
        .build()?;
    let err = invalid
        .domains()
        .list(ListOptions::default())
        .await
        .unwrap_err();
    assert!(err.is(&ErrorCode::InvalidApiKey), "{err:?}");
    assert_eq!(err.status(), 403);
    Ok(())
}

#[tokio::test]
async fn domain_lifecycle() -> TestResult {
    let Some(live) = live() else { return Ok(()) };
    let agdir = client(&live);
    let name = format!("rust-{}.test", unique());

    let domain = agdir.domains().create(name.as_str()).await?;
    let result = async {
        assert_eq!(domain.name, name);
        assert!(!domain.records.is_empty());

        let fetched = agdir.domains().get(&domain.id).await?;
        assert_eq!(fetched.id, domain.id);
        assert_ne!(fetched.status, DomainStatus::Unknown);

        assert_eq!(agdir.domains().verify(&domain.id).await?.id, domain.id);

        let all: Vec<_> = agdir
            .domains()
            .list_all(ListOptions::new().limit(1))
            .try_collect()
            .await?;
        assert!(all.iter().any(|d| d.id == domain.id));
        Ok::<_, Box<dyn std::error::Error>>(())
    }
    .await;

    let deleted = agdir.domains().delete(&domain.id).await?;
    assert!(deleted.deleted);
    result?;

    let err = agdir.domains().get(&domain.id).await.unwrap_err();
    assert!(err.is(&ErrorCode::NotFound), "{err:?}");
    Ok(())
}

#[tokio::test]
async fn webhook_lifecycle() -> TestResult {
    let Some(live) = live() else { return Ok(()) };
    let agdir = client(&live);

    let created = agdir
        .webhooks()
        .create(CreateWebhook::new(
            "https://93.184.216.34/hook",
            [WebhookEventType::Delivered, WebhookEventType::Bounced],
        ))
        .await?;
    let result = async {
        assert!(created.signing_secret.starts_with("whsec_"));

        let webhook = agdir.webhooks().get(&created.id).await?;
        assert_eq!(webhook.endpoint, "https://93.184.216.34/hook");
        assert_eq!(webhook.status, WebhookStatus::Enabled);
        assert_eq!(
            webhook.signing_secret.as_deref(),
            Some(created.signing_secret.as_str())
        );

        agdir
            .webhooks()
            .update(
                &created.id,
                UpdateWebhook::new().status(WebhookStatus::Disabled),
            )
            .await?;
        assert_eq!(
            agdir.webhooks().get(&created.id).await?.status,
            WebhookStatus::Disabled
        );

        let rotated = agdir.webhooks().rotate_secret(&created.id).await?;
        assert_ne!(rotated.signing_secret, created.signing_secret);

        let deliveries = agdir
            .webhooks()
            .deliveries()
            .list(&created.id, ListOptions::default())
            .await?;
        assert!(deliveries.data.is_empty());
        assert!(!deliveries.has_more);
        Ok::<_, Box<dyn std::error::Error>>(())
    }
    .await;

    assert!(agdir.webhooks().delete(&created.id).await?.deleted);
    result
}
