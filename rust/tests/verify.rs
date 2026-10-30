//! Webhook signature verification.

use std::collections::HashMap;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use agdirlabs_email::time::macros::datetime;
use agdirlabs_email::{
    sign_webhook, verify_webhook, verify_webhook_with, VerifyOptions, WebhookEventType,
    WebhookVerificationError,
};
use base64::engine::general_purpose::STANDARD as BASE64;
use base64::Engine as _;
use hmac::{Hmac, Mac};
use sha2::Sha256;

const SECRET: &str = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw";
const OTHER_SECRET: &str = "whsec_dGhpcyBpcyBhIGRpZmZlcmVudCBzZWNyZXQ=";

const PAYLOAD: &str = r#"{"type":"email.bounced","created_at":"2026-10-30T10:00:05.000Z","data":{"created_at":"2026-10-30T10:00:00.000Z","email_id":"em_1","message_id":"<m1@eu-north-1.amazonses.com>","from":"Acme <hello@acme.com>","to":["b@example.com"],"subject":"Hi","tags":{"kind":"welcome"},"bounce":{"message":"User unknown","subType":"General","type":"Permanent"}}}"#;

fn now() -> i64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_secs() as i64
}

/// Signs independently of the crate, straight from the Standard Webhooks spec.
fn reference_signature(secret: &str, id: &str, ts: i64, payload: &str) -> String {
    let key = BASE64
        .decode(secret.strip_prefix("whsec_").unwrap())
        .unwrap();
    let mut mac = Hmac::<Sha256>::new_from_slice(&key).unwrap();
    mac.update(format!("{id}.{ts}.{payload}").as_bytes());
    format!("v1,{}", BASE64.encode(mac.finalize().into_bytes()))
}

fn headers(id: &str, ts: i64, signature: &str) -> http::HeaderMap {
    let mut h = http::HeaderMap::new();
    h.insert("webhook-id", id.parse().unwrap());
    h.insert("webhook-timestamp", ts.to_string().parse().unwrap());
    h.insert("webhook-signature", signature.parse().unwrap());
    h
}

#[test]
fn accepts_a_valid_signature_and_maps_the_event() {
    let ts = now();
    let sig = reference_signature(SECRET, "msg_1", ts, PAYLOAD);
    assert_eq!(
        sign_webhook(SECRET, "msg_1", ts, PAYLOAD.as_bytes()).unwrap(),
        sig
    );

    let event = verify_webhook(PAYLOAD, &headers("msg_1", ts, &sig), SECRET).unwrap();
    assert_eq!(event.event_type, WebhookEventType::Bounced);
    assert_eq!(event.created_at, datetime!(2026-10-30 10:00:05 UTC));
    assert_eq!(event.data.email_id, "em_1");
    assert_eq!(
        event.data.message_id.as_deref(),
        Some("<m1@eu-north-1.amazonses.com>")
    );
    assert_eq!(event.data.to, ["b@example.com"]);
    assert_eq!(event.data.tags["kind"], "welcome");
    assert_eq!(event.data.created_at, datetime!(2026-10-30 10:00 UTC));
    let bounce = event.data.bounce.unwrap();
    assert_eq!(bounce.bounce_type, "Permanent");
    assert_eq!(bounce.sub_type, "General");
    assert_eq!(bounce.message, "User unknown");
    assert!(event.data.click.is_none() && event.data.failed.is_none());
}

#[test]
fn maps_click_and_failure_details() {
    let payload = r#"{"type":"email.clicked","created_at":"2026-10-30T10:00:05Z","data":{"created_at":"2026-10-30T10:00:00Z","email_id":"em_1","from":"a@acme.com","to":"b@example.com","subject":"Hi","click":{"ipAddress":"203.0.113.9","link":"https://acme.com","timestamp":"2026-10-30T10:00:04Z","userAgent":"Mozilla/5.0"},"failed":{"reason":"nope"}}}"#;
    let ts = now();
    let sig = reference_signature(SECRET, "msg_2", ts, payload);
    let event = verify_webhook(payload, &headers("msg_2", ts, &sig), SECRET).unwrap();
    assert_eq!(event.event_type, WebhookEventType::Clicked);
    let click = event.data.click.unwrap();
    assert_eq!(click.ip_address, "203.0.113.9");
    assert_eq!(click.user_agent, "Mozilla/5.0");
    assert_eq!(click.timestamp, datetime!(2026-10-30 10:00:04 UTC));
    assert_eq!(event.data.failed.unwrap().reason, "nope");
    assert!(event.data.tags.is_empty());
}

#[test]
fn rejects_a_tampered_body() {
    let ts = now();
    let sig = reference_signature(SECRET, "msg_1", ts, PAYLOAD);
    let tampered = PAYLOAD.replace("em_1", "em_2");
    assert_eq!(
        verify_webhook(tampered, &headers("msg_1", ts, &sig), SECRET),
        Err(WebhookVerificationError::InvalidSignature)
    );
    // The id and timestamp are signed too.
    assert_eq!(
        verify_webhook(PAYLOAD, &headers("msg_X", ts, &sig), SECRET),
        Err(WebhookVerificationError::InvalidSignature)
    );
}

#[test]
fn rejects_the_wrong_secret_and_invalid_secrets() {
    let ts = now();
    let sig = reference_signature(OTHER_SECRET, "msg_1", ts, PAYLOAD);
    assert_eq!(
        verify_webhook(PAYLOAD, &headers("msg_1", ts, &sig), SECRET),
        Err(WebhookVerificationError::InvalidSignature)
    );
    assert_eq!(
        verify_webhook(PAYLOAD, &headers("msg_1", ts, &sig), "whsec_not base64!"),
        Err(WebhookVerificationError::InvalidSecret)
    );
}

#[test]
fn rejects_missing_headers() {
    let ts = now();
    let sig = reference_signature(SECRET, "msg_1", ts, PAYLOAD);
    for missing in ["webhook-id", "webhook-timestamp", "webhook-signature"] {
        let mut h = headers("msg_1", ts, &sig);
        h.remove(missing);
        assert_eq!(
            verify_webhook(PAYLOAD, &h, SECRET),
            Err(WebhookVerificationError::MissingHeaders),
            "without {missing}"
        );
    }
}

#[test]
fn rejects_stale_future_and_malformed_timestamps() {
    let ts = now() - 301;
    let sig = reference_signature(SECRET, "msg_1", ts, PAYLOAD);
    assert_eq!(
        verify_webhook(PAYLOAD, &headers("msg_1", ts, &sig), SECRET),
        Err(WebhookVerificationError::InvalidTimestamp)
    );

    let ts = now() + 301;
    let sig = reference_signature(SECRET, "msg_1", ts, PAYLOAD);
    assert_eq!(
        verify_webhook(PAYLOAD, &headers("msg_1", ts, &sig), SECRET),
        Err(WebhookVerificationError::InvalidTimestamp)
    );

    let mut h = headers("msg_1", now(), &sig);
    h.insert("webhook-timestamp", "yesterday".parse().unwrap());
    assert_eq!(
        verify_webhook(PAYLOAD, &h, SECRET),
        Err(WebhookVerificationError::InvalidTimestamp)
    );
}

#[test]
fn tolerance_and_clock_are_configurable() {
    let ts = 1_700_000_000;
    let sig = reference_signature(SECRET, "msg_1", ts, PAYLOAD);
    let h = headers("msg_1", ts, &sig);
    let at = |secs: u64| VerifyOptions {
        now: Some(UNIX_EPOCH + Duration::from_secs(secs)),
        ..VerifyOptions::default()
    };
    assert!(verify_webhook_with(PAYLOAD, &h, SECRET, at(1_700_000_300)).is_ok());
    assert!(verify_webhook_with(PAYLOAD, &h, SECRET, at(1_700_000_301)).is_err());
    let wide = VerifyOptions {
        tolerance: Duration::from_secs(3600),
        ..at(1_700_003_000)
    };
    assert!(verify_webhook_with(PAYLOAD, &h, SECRET, wide).is_ok());
}

#[test]
fn accepts_any_matching_entry_of_a_multi_signature_header() {
    let ts = now();
    let good = reference_signature(SECRET, "msg_1", ts, PAYLOAD);
    let old = reference_signature(OTHER_SECRET, "msg_1", ts, PAYLOAD);
    let header = format!("v1a,ignored {old} v1,!!notbase64 {good}");
    assert!(verify_webhook(PAYLOAD, &headers("msg_1", ts, &header), SECRET).is_ok());
    assert!(verify_webhook(PAYLOAD, &headers("msg_1", ts, &old), SECRET).is_err());
}

#[test]
fn works_with_maps_and_pair_lists_case_insensitively() {
    let ts = now();
    let sig = reference_signature(SECRET, "msg_1", ts, PAYLOAD);

    let map: HashMap<String, String> = [
        ("Webhook-Id".to_string(), "msg_1".to_string()),
        ("Webhook-Timestamp".to_string(), ts.to_string()),
        ("Webhook-Signature".to_string(), sig.clone()),
    ]
    .into_iter()
    .collect();
    assert!(verify_webhook(PAYLOAD.as_bytes(), &map, SECRET).is_ok());

    let ts_string = ts.to_string();
    let pairs = vec![
        ("webhook-id", "msg_1"),
        ("webhook-timestamp", ts_string.as_str()),
        ("webhook-signature", sig.as_str()),
    ];
    assert!(verify_webhook(PAYLOAD, &pairs, SECRET).is_ok());
    assert!(verify_webhook(PAYLOAD, pairs.as_slice(), SECRET).is_ok());
}

#[test]
fn signed_but_invalid_json_is_rejected() {
    let ts = now();
    let payload = "not json";
    let sig = reference_signature(SECRET, "msg_1", ts, payload);
    assert!(matches!(
        verify_webhook(payload, &headers("msg_1", ts, &sig), SECRET),
        Err(WebhookVerificationError::InvalidPayload(_))
    ));
}
