//! Client behaviour against a stubbed HTTP server.

use std::sync::Mutex;
use std::time::Duration;

use agdirlabs_email::stream::TryStreamExt;
use agdirlabs_email::time::macros::datetime;
use agdirlabs_email::{
    AgdirEmail, BatchOptions, CreateDomain, CreateWebhook, DnsRecordPurpose, DomainStatus,
    EmailStatus, Error, ErrorCode, ListOptions, SendEmail, SendOptions, UpdateWebhook,
    WebhookDeliveryStatus, WebhookEventType, WebhookStatus, DEFAULT_BASE_URL, USER_AGENT,
};
use serde_json::{json, Value};
use wiremock::matchers::{body_json, header, method, path, query_param};
use wiremock::{Mock, MockServer, Request, ResponseTemplate};

/// Serializes tests that touch process environment variables.
static ENV: Mutex<()> = Mutex::new(());

fn client(server: &MockServer) -> AgdirEmail {
    AgdirEmail::builder()
        .api_key("re_test")
        .base_url(server.uri())
        .build()
        .unwrap()
}

fn email() -> SendEmail {
    SendEmail::new("a@acme.com", "b@example.com", "s").text("t")
}

fn body(req: &Request) -> Value {
    serde_json::from_slice(&req.body).unwrap()
}

#[test]
fn configuration_from_arguments_and_environment() {
    let _guard = ENV.lock().unwrap_or_else(|e| e.into_inner());
    std::env::remove_var("AGDIR_EMAIL_API_KEY");
    std::env::set_var("AGDIR_EMAIL_BASE_URL", "");

    assert_eq!(
        AgdirEmail::new("re_test").unwrap().base_url(),
        DEFAULT_BASE_URL
    );

    let err = AgdirEmail::from_env().unwrap_err();
    assert!(matches!(err, Error::Config(_)));
    assert!(err.to_string().contains("AGDIR_EMAIL_API_KEY"));
    assert!(err.to_string().contains("https://app.agdir.email"));

    std::env::set_var("AGDIR_EMAIL_API_KEY", "re_env");
    std::env::set_var("AGDIR_EMAIL_BASE_URL", "http://localhost:8080/");
    assert_eq!(
        AgdirEmail::from_env().unwrap().base_url(),
        "http://localhost:8080"
    );

    // Empty values count as unset.
    std::env::set_var("AGDIR_EMAIL_API_KEY", "");
    assert!(AgdirEmail::from_env().is_err());
    assert!(AgdirEmail::builder().api_key("").build().is_err());

    std::env::remove_var("AGDIR_EMAIL_API_KEY");
    std::env::remove_var("AGDIR_EMAIL_BASE_URL");

    let err = AgdirEmail::builder()
        .api_key("re_test")
        .base_url("not a url")
        .build()
        .unwrap_err();
    assert!(matches!(err, Error::Config(m) if m.contains("Invalid base URL")));
}

#[test]
fn user_agent_names_the_crate_version() {
    assert_eq!(
        USER_AGENT,
        format!("agdirlabs-email-rust/{}", env!("CARGO_PKG_VERSION"))
    );
}

#[tokio::test]
async fn send_posts_wire_json_with_auth_user_agent_and_idempotency_key() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/emails"))
        .and(header("authorization", "Bearer re_test"))
        .and(header("user-agent", USER_AGENT))
        .and(header("accept", "application/json"))
        .and(header("content-type", "application/json"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({ "id": "em_1" })))
        .expect(1)
        .mount(&server)
        .await;

    let sent = client(&server)
        .emails()
        .send(
            SendEmail::new("Acme <hello@acme.com>", "user@example.com", "Hi")
                .text("Hello")
                .html("<p>Hello</p>")
                .reply_to("support@acme.com")
                .cc(["c1@example.com", "c2@example.com"])
                .header("X-Ref", "42")
                .tag("kind", "welcome")
                .scheduled_at(datetime!(2026-11-01 09:00 UTC)),
        )
        .await
        .unwrap();
    assert_eq!(sent.id, "em_1");

    let requests = server.received_requests().await.unwrap();
    let key = requests[0].headers.get("idempotency-key").unwrap();
    assert!(uuid_like(key.to_str().unwrap()), "{key:?}");
    assert_eq!(
        body(&requests[0]),
        json!({
            "from": "Acme <hello@acme.com>",
            "to": ["user@example.com"],
            "subject": "Hi",
            "html": "<p>Hello</p>",
            "text": "Hello",
            "cc": ["c1@example.com", "c2@example.com"],
            "reply_to": ["support@acme.com"],
            "headers": { "X-Ref": "42" },
            "tags": [{ "name": "kind", "value": "welcome" }],
            "scheduled_at": "2026-11-01T09:00:00Z"
        })
    );
}

fn uuid_like(s: &str) -> bool {
    s.len() == 36 && s.chars().all(|c| c.is_ascii_hexdigit() || c == '-')
}

#[tokio::test]
async fn uses_the_callers_idempotency_key_and_none_when_retries_are_off() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/emails"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({ "id": "a" })))
        .mount(&server)
        .await;

    client(&server)
        .emails()
        .send_with(email(), SendOptions::new().idempotency_key("order-42"))
        .await
        .unwrap();
    AgdirEmail::builder()
        .api_key("re_test")
        .base_url(server.uri())
        .max_retries(0)
        .build()
        .unwrap()
        .emails()
        .send(email())
        .await
        .unwrap();

    let requests = server.received_requests().await.unwrap();
    assert_eq!(
        requests[0].headers.get("idempotency-key").unwrap(),
        "order-42"
    );
    assert!(requests[1].headers.get("idempotency-key").is_none());
}

#[tokio::test]
async fn base_url_may_carry_a_path_prefix_and_ids_are_encoded() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/v1/emails/a%2Fb%3Fc"))
        .respond_with(ResponseTemplate::new(404).set_body_json(json!({
            "statusCode": 404, "name": "not_found", "message": "Email not found"
        })))
        .expect(1)
        .mount(&server)
        .await;

    let agdir = AgdirEmail::builder()
        .api_key("re_test")
        .base_url(format!("{}/v1/", server.uri()))
        .build()
        .unwrap();
    let err = agdir.emails().get("a/b?c").await.unwrap_err();
    assert!(err.is(&ErrorCode::NotFound));
    assert_eq!(err.status(), 404);
}

#[tokio::test]
async fn sends_permissive_batches_and_returns_per_index_errors() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/emails/batch"))
        .and(header("x-batch-validation", "permissive"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "data": [{ "id": "em_1" }],
            "errors": [{ "index": 1, "message": "bad to" }]
        })))
        .expect(1)
        .mount(&server)
        .await;
    Mock::given(method("POST"))
        .and(path("/emails/batch"))
        .respond_with(
            ResponseTemplate::new(200).set_body_json(json!({ "data": [{ "id": "em_2" }] })),
        )
        .mount(&server)
        .await;

    let agdir = client(&server);
    let result = agdir
        .emails()
        .send_batch_with(
            vec![email(), SendEmail::new("a@acme.com", "nope", "s").text("t")],
            BatchOptions::new().permissive(),
        )
        .await
        .unwrap();
    assert_eq!(result.data[0].id, "em_1");
    assert_eq!(result.errors[0].index, 1);
    assert_eq!(result.errors[0].message, "bad to");

    let strict = agdir.emails().send_batch(vec![email()]).await.unwrap();
    assert_eq!(strict.data[0].id, "em_2");
    assert!(strict.errors.is_empty());

    let requests = server.received_requests().await.unwrap();
    assert_eq!(body(&requests[0]).as_array().unwrap().len(), 2);
    assert!(requests[1].headers.get("x-batch-validation").is_none());
    assert!(requests[1].headers.get("idempotency-key").is_some());
}

#[tokio::test]
async fn maps_an_email_with_dates_status_and_empty_lists() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/emails/em_1"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "object": "email",
            "id": "em_1",
            "from": "a@acme.com",
            "to": ["b@example.com"],
            "cc": null,
            "bcc": null,
            "reply_to": null,
            "subject": "s",
            "html": null,
            "text": "t",
            "created_at": "2026-10-30T10:00:00.000Z",
            "scheduled_at": "2026-10-30T11:00:00.000Z",
            "last_event": "delivered"
        })))
        .mount(&server)
        .await;

    let email = client(&server).emails().get("em_1").await.unwrap();
    assert_eq!(email.to, vec!["b@example.com"]);
    assert!(email.cc.is_empty() && email.bcc.is_empty() && email.reply_to.is_empty());
    assert_eq!(email.html, None);
    assert_eq!(email.text.as_deref(), Some("t"));
    assert_eq!(email.created_at, datetime!(2026-10-30 10:00 UTC));
    assert_eq!(email.scheduled_at, Some(datetime!(2026-10-30 11:00 UTC)));
    assert_eq!(email.status, EmailStatus::Delivered);
}

#[tokio::test]
async fn cancel_posts_to_the_cancel_path() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/emails/em_1/cancel"))
        .respond_with(
            ResponseTemplate::new(200).set_body_json(json!({ "object": "email", "id": "em_1" })),
        )
        .expect(1)
        .mount(&server)
        .await;
    assert_eq!(
        client(&server).emails().cancel("em_1").await.unwrap().id,
        "em_1"
    );
}

#[tokio::test]
async fn api_errors_carry_code_status_message_and_retry_after() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/emails"))
        .respond_with(ResponseTemplate::new(422).set_body_json(json!({
            "statusCode": 422, "name": "validation_error", "message": "Invalid `to` field."
        })))
        .mount(&server)
        .await;

    let err = client(&server).emails().send(email()).await.unwrap_err();
    match &err {
        Error::Api {
            code,
            status,
            message,
            retry_after,
        } => {
            assert_eq!(code, &ErrorCode::ValidationError);
            assert_eq!(*status, 422);
            assert_eq!(message, "Invalid `to` field.");
            assert_eq!(*retry_after, None);
        }
        other => panic!("unexpected {other:?}"),
    }
    assert_eq!(
        err.to_string(),
        "Invalid `to` field. (validation_error, HTTP 422)"
    );
    assert_eq!(server.received_requests().await.unwrap().len(), 1);
}

#[tokio::test]
async fn non_json_errors_fall_back_to_the_status_line() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .respond_with(ResponseTemplate::new(502).set_body_string("<html>Bad gateway</html>"))
        .mount(&server)
        .await;
    let agdir = AgdirEmail::builder()
        .api_key("re_test")
        .base_url(server.uri())
        .max_retries(0)
        .build()
        .unwrap();

    let err = agdir.emails().get("em_1").await.unwrap_err();
    assert!(err.is(&ErrorCode::InternalServerError));
    assert_eq!(err.status(), 502);
    assert!(
        matches!(&err, Error::Api { message, .. } if message == "Request failed with HTTP 502")
    );

    let unknown = ErrorCode::parse("brand_new_code");
    assert_eq!(unknown, ErrorCode::Other("brand_new_code".into()));
    assert_eq!(unknown.as_str(), "brand_new_code");
}

#[tokio::test]
async fn retries_rate_limits_honouring_retry_after() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/emails"))
        .respond_with(
            ResponseTemplate::new(429)
                .insert_header("retry-after", "0")
                .set_body_json(json!({
                    "statusCode": 429, "name": "rate_limit_exceeded", "message": "Too many requests"
                })),
        )
        .up_to_n_times(2)
        .mount(&server)
        .await;
    Mock::given(method("POST"))
        .and(path("/emails"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({ "id": "em_1" })))
        .mount(&server)
        .await;

    let sent = client(&server).emails().send(email()).await.unwrap();
    assert_eq!(sent.id, "em_1");
    let requests = server.received_requests().await.unwrap();
    assert_eq!(requests.len(), 3);
    // Every attempt carries the same idempotency key.
    let keys: Vec<_> = requests
        .iter()
        .map(|r| r.headers.get("idempotency-key").unwrap().clone())
        .collect();
    assert!(keys.windows(2).all(|w| w[0] == w[1]));
}

#[tokio::test]
async fn gives_up_after_max_retries_and_reports_retry_after() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .respond_with(
            ResponseTemplate::new(429)
                .insert_header("retry-after", "0")
                .set_body_json(json!({
                    "statusCode": 429, "name": "rate_limit_exceeded", "message": "Too many requests"
                })),
        )
        .mount(&server)
        .await;

    let err = client(&server).domains().get("d_1").await.unwrap_err();
    assert!(err.is(&ErrorCode::RateLimitExceeded));
    assert_eq!(err.retry_after(), Some(Duration::ZERO));
    assert_eq!(server.received_requests().await.unwrap().len(), 3);
}

#[tokio::test]
async fn never_retries_quota_errors() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .respond_with(ResponseTemplate::new(429).set_body_json(json!({
            "statusCode": 429, "name": "daily_quota_exceeded", "message": "Daily quota exceeded"
        })))
        .mount(&server)
        .await;

    let err = client(&server).emails().send(email()).await.unwrap_err();
    assert!(err.is(&ErrorCode::DailyQuotaExceeded));
    assert_eq!(server.received_requests().await.unwrap().len(), 1);
}

#[tokio::test]
async fn retries_server_errors_only_for_retryable_requests() {
    let server = MockServer::start().await;
    let unavailable = ResponseTemplate::new(503)
        .insert_header("retry-after", "0")
        .set_body_json(
            json!({ "statusCode": 503, "name": "internal_server_error", "message": "down" }),
        );
    Mock::given(method("GET"))
        .and(path("/domains/d_1"))
        .respond_with(unavailable.clone())
        .up_to_n_times(2)
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/domains/d_1"))
        .respond_with(ResponseTemplate::new(200).set_body_json(domain_json()))
        .mount(&server)
        .await;
    Mock::given(method("POST"))
        .and(path("/domains"))
        .respond_with(unavailable)
        .mount(&server)
        .await;

    let agdir = client(&server);
    assert_eq!(agdir.domains().get("d_1").await.unwrap().id, "d_1");
    // Creating a domain is not idempotent: a 503 is returned as is.
    let err = agdir.domains().create("acme.com").await.unwrap_err();
    assert_eq!(err.status(), 503);

    let requests = server.received_requests().await.unwrap();
    assert_eq!(
        requests
            .iter()
            .filter(|r| r.method.as_str() == "GET")
            .count(),
        3
    );
    assert_eq!(
        requests
            .iter()
            .filter(|r| r.method.as_str() == "POST")
            .count(),
        1
    );
}

#[tokio::test]
async fn network_errors_and_timeouts() {
    // Nothing listens on port 1.
    let agdir = AgdirEmail::builder()
        .api_key("re_test")
        .base_url("http://127.0.0.1:1")
        .max_retries(0)
        .build()
        .unwrap();
    let err = agdir.emails().get("em_1").await.unwrap_err();
    assert!(matches!(err, Error::Network { .. }), "{err:?}");
    assert_eq!(err.code(), Some(&ErrorCode::NetworkError));
    assert_eq!(err.status(), 0);

    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .respond_with(
            ResponseTemplate::new(200)
                .set_body_json(json!({ "id": "x" }))
                .set_delay(Duration::from_millis(500)),
        )
        .mount(&server)
        .await;
    let agdir = AgdirEmail::builder()
        .api_key("re_test")
        .base_url(server.uri())
        .max_retries(0)
        .timeout(Duration::from_millis(100))
        .build()
        .unwrap();
    let err = agdir.emails().get("em_1").await.unwrap_err();
    assert!(matches!(err, Error::Timeout { .. }), "{err:?}");
    assert_eq!(err.code(), Some(&ErrorCode::Timeout));
    assert_eq!(err.to_string(), "Request timed out after 100 ms");
}

#[tokio::test]
async fn undecodable_success_responses_are_decode_errors() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .respond_with(ResponseTemplate::new(200).set_body_string("<html>hello</html>"))
        .mount(&server)
        .await;
    let err = client(&server).emails().get("em_1").await.unwrap_err();
    assert!(matches!(err, Error::Decode { status: 200, .. }), "{err:?}");
}

fn domain_json() -> Value {
    json!({
        "object": "domain",
        "id": "d_1",
        "name": "acme.com",
        "status": "pending",
        "created_at": "2026-10-30T10:00:00.000Z",
        "region": "eu-north-1",
        "capabilities": { "sending": "enabled", "receiving": "disabled" },
        "open_tracking": false,
        "click_tracking": false,
        "records": [
            { "record": "DKIM", "name": "agdir._domainkey", "type": "TXT", "ttl": "Auto", "status": "pending", "value": "p=MIGf" },
            { "record": "SPF", "name": "send", "type": "MX", "ttl": "Auto", "status": "pending", "value": "feedback-smtp.eu-north-1.amazonses.com", "priority": 10 }
        ]
    })
}

#[tokio::test]
async fn domains_create_get_verify_delete() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/domains"))
        .and(body_json(
            json!({ "name": "acme.com", "region": "eu-north-1" }),
        ))
        .respond_with(ResponseTemplate::new(200).set_body_json(domain_json()))
        .expect(1)
        .mount(&server)
        .await;
    Mock::given(method("POST"))
        .and(path("/domains/d_1/verify"))
        .respond_with(
            ResponseTemplate::new(200).set_body_json(json!({ "object": "domain", "id": "d_1" })),
        )
        .expect(1)
        .mount(&server)
        .await;
    Mock::given(method("DELETE"))
        .and(path("/domains/d_1"))
        .respond_with(
            ResponseTemplate::new(200)
                .set_body_json(json!({ "object": "domain", "id": "d_1", "deleted": true })),
        )
        .expect(1)
        .mount(&server)
        .await;

    let agdir = client(&server);
    let domain = agdir
        .domains()
        .create(CreateDomain::new("acme.com").region("eu-north-1"))
        .await
        .unwrap();
    assert_eq!(domain.status, DomainStatus::Pending);
    assert_eq!(domain.created_at, datetime!(2026-10-30 10:00 UTC));
    assert_eq!(domain.records.len(), 2);
    assert_eq!(domain.records[0].purpose, DnsRecordPurpose::Dkim);
    assert_eq!(domain.records[0].record_type, "TXT");
    assert_eq!(domain.records[0].priority, None);
    assert_eq!(domain.records[1].purpose, DnsRecordPurpose::Spf);
    assert_eq!(domain.records[1].priority, Some(10));

    assert_eq!(agdir.domains().verify("d_1").await.unwrap().id, "d_1");
    let deleted = agdir.domains().delete("d_1").await.unwrap();
    assert!(deleted.deleted);
    assert_eq!(deleted.id, "d_1");
}

fn list_item(id: &str) -> Value {
    let mut d = domain_json();
    d["id"] = json!(id);
    d.as_object_mut().unwrap().remove("records");
    d
}

#[tokio::test]
async fn list_returns_a_page_and_list_all_follows_cursors() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/domains"))
        .and(query_param("after", "d_2"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "object": "list", "has_more": false, "data": [list_item("d_3")]
        })))
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/domains"))
        .and(query_param("limit", "2"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "object": "list", "has_more": true, "data": [list_item("d_1"), list_item("d_2")]
        })))
        .mount(&server)
        .await;

    let agdir = client(&server);
    let page = agdir
        .domains()
        .list(ListOptions::new().limit(2))
        .await
        .unwrap();
    assert!(page.has_more);
    assert_eq!(page.next_cursor.as_deref(), Some("d_2"));
    assert!(page.data[0].records.is_empty());

    let all: Vec<_> = agdir
        .domains()
        .list_all(ListOptions::new().limit(2))
        .try_collect()
        .await
        .unwrap();
    let ids: Vec<_> = all.iter().map(|d| d.id.as_str()).collect();
    assert_eq!(ids, ["d_1", "d_2", "d_3"]);

    let requests = server.received_requests().await.unwrap();
    let last = requests.last().unwrap().url.query().unwrap().to_owned();
    assert!(
        last.contains("limit=2") && last.contains("after=d_2"),
        "{last}"
    );
}

#[tokio::test]
async fn list_all_ends_after_an_error() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .respond_with(ResponseTemplate::new(401).set_body_json(json!({
            "statusCode": 401, "name": "restricted_api_key", "message": "This API key is restricted to sending emails"
        })))
        .mount(&server)
        .await;

    let results: Vec<_> = futures_util::StreamExt::collect::<Vec<_>>(
        client(&server).webhooks().list_all(ListOptions::default()),
    )
    .await;
    assert_eq!(results.len(), 1);
    assert!(results[0]
        .as_ref()
        .unwrap_err()
        .is(&ErrorCode::RestrictedApiKey));
}

#[tokio::test]
async fn webhooks_crud_and_secret_rotation() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/webhooks"))
        .and(body_json(json!({
            "endpoint": "https://acme.com/hook",
            "events": ["email.delivered", "email.bounced"]
        })))
        .respond_with(ResponseTemplate::new(200).set_body_json(
            json!({ "object": "webhook", "id": "wh_1", "signing_secret": "whsec_abc" }),
        ))
        .expect(1)
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/webhooks/wh_1"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "object": "webhook", "id": "wh_1", "endpoint": "https://acme.com/hook",
            "created_at": "2026-10-30T10:00:00.000Z", "status": "enabled",
            "events": ["email.delivered", "email.bounced"], "signing_secret": "whsec_abc"
        })))
        .mount(&server)
        .await;
    Mock::given(method("PATCH"))
        .and(path("/webhooks/wh_1"))
        .and(body_json(json!({ "status": "disabled" })))
        .respond_with(
            ResponseTemplate::new(200).set_body_json(json!({ "object": "webhook", "id": "wh_1" })),
        )
        .expect(1)
        .mount(&server)
        .await;
    Mock::given(method("POST"))
        .and(path("/webhooks/wh_1/signing-secret/rotate"))
        .respond_with(ResponseTemplate::new(200).set_body_json(
            json!({ "object": "webhook", "id": "wh_1", "signing_secret": "whsec_new" }),
        ))
        .expect(1)
        .mount(&server)
        .await;
    Mock::given(method("DELETE"))
        .and(path("/webhooks/wh_1"))
        .respond_with(
            ResponseTemplate::new(200)
                .set_body_json(json!({ "object": "webhook", "id": "wh_1", "deleted": true })),
        )
        .expect(1)
        .mount(&server)
        .await;

    let agdir = client(&server);
    let created = agdir
        .webhooks()
        .create(CreateWebhook::new(
            "https://acme.com/hook",
            [WebhookEventType::Delivered, WebhookEventType::Bounced],
        ))
        .await
        .unwrap();
    assert_eq!(created.signing_secret, "whsec_abc");

    let webhook = agdir.webhooks().get("wh_1").await.unwrap();
    assert_eq!(webhook.status, WebhookStatus::Enabled);
    assert_eq!(
        webhook.events,
        [WebhookEventType::Delivered, WebhookEventType::Bounced]
    );
    assert_eq!(webhook.signing_secret.as_deref(), Some("whsec_abc"));

    agdir
        .webhooks()
        .update("wh_1", UpdateWebhook::new().status(WebhookStatus::Disabled))
        .await
        .unwrap();
    assert_eq!(
        agdir
            .webhooks()
            .rotate_secret("wh_1")
            .await
            .unwrap()
            .signing_secret,
        "whsec_new"
    );
    assert!(agdir.webhooks().delete("wh_1").await.unwrap().deleted);
}

#[tokio::test]
async fn webhook_deliveries_and_attempts() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/webhooks/wh_1/events"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "object": "list", "has_more": false,
            "data": [{ "id": "ev_1", "type": "email.delivered", "created_at": "2026-10-30T10:00:00.000Z", "status": "failed" }]
        })))
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/webhooks/wh_1/events/ev_1"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "object": "webhook_event", "id": "ev_1", "type": "email.delivered",
            "created_at": "2026-10-30T10:00:00.000Z", "status": "pending",
            "next_attempt_at": "2026-10-30T10:05:00.000Z", "payload": { "type": "email.delivered" }
        })))
        .mount(&server)
        .await;
    Mock::given(method("GET"))
        .and(path("/webhooks/wh_1/events/ev_1/attempts"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "object": "list", "has_more": false,
            "data": [
                { "id": "at_2", "http_status_code": 500, "response": "oops", "sent_at": "2026-10-30T10:00:05.000Z" },
                { "id": "at_1", "http_status_code": 0, "response": "connect ECONNREFUSED", "sent_at": "2026-10-30T10:00:00.000Z" }
            ]
        })))
        .mount(&server)
        .await;
    Mock::given(method("POST"))
        .and(path("/webhooks/wh_1/events/ev_1/replay"))
        .respond_with(
            ResponseTemplate::new(200)
                .set_body_json(json!({ "object": "webhook_event", "id": "ev_2" })),
        )
        .expect(1)
        .mount(&server)
        .await;

    let agdir = client(&server);
    let deliveries = agdir.webhooks().deliveries();
    let page = deliveries
        .list("wh_1", ListOptions::default())
        .await
        .unwrap();
    assert_eq!(page.data[0].event_type, WebhookEventType::Delivered);
    assert_eq!(page.data[0].status, WebhookDeliveryStatus::Failed);
    assert_eq!(page.next_cursor, None);

    let all: Vec<_> = deliveries
        .list_all("wh_1", ListOptions::default())
        .try_collect()
        .await
        .unwrap();
    assert_eq!(all.len(), 1);

    let detail = deliveries.get("wh_1", "ev_1").await.unwrap();
    assert_eq!(
        detail.next_attempt_at,
        Some(datetime!(2026-10-30 10:05 UTC))
    );
    assert_eq!(detail.payload, Some(json!({ "type": "email.delivered" })));

    let attempts = deliveries
        .attempts("wh_1", "ev_1", ListOptions::default())
        .await
        .unwrap();
    assert_eq!(attempts.data[0].http_status, 500);
    assert_eq!(attempts.data[1].http_status, 0);
    assert_eq!(attempts.data[1].sent_at, datetime!(2026-10-30 10:00 UTC));

    assert_eq!(deliveries.replay("wh_1", "ev_1").await.unwrap().id, "ev_2");
}

#[tokio::test]
async fn unknown_enum_values_do_not_break_decoding() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .respond_with(ResponseTemplate::new(200).set_body_json(json!({
            "object": "email", "id": "em_1", "from": "a@acme.com", "to": "b@example.com",
            "subject": "s", "created_at": "2026-10-30T10:00:00Z", "last_event": "teleported"
        })))
        .mount(&server)
        .await;
    let email = client(&server).emails().get("em_1").await.unwrap();
    assert_eq!(email.status, EmailStatus::Unknown);
    assert_eq!(email.to, ["b@example.com"]);
}
