//! Verifying signed webhook requests (Standard Webhooks).

use std::collections::{BTreeMap, HashMap};
use std::hash::BuildHasher;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use base64::engine::general_purpose::STANDARD as BASE64;
use base64::Engine as _;
use hmac::{Hmac, Mac};
use serde::{Deserialize, Serialize};
use sha2::Sha256;
use time::OffsetDateTime;

use crate::error::WebhookVerificationError;
use crate::serde_util::string_or_vec;
use crate::webhooks::WebhookEventType;

type HmacSha256 = Hmac<Sha256>;

/// Default tolerance between the `webhook-timestamp` header and the current time.
pub const DEFAULT_TOLERANCE: Duration = Duration::from_secs(300);

/// Read access to request headers, so [`verify_webhook`] works with any web framework.
///
/// Implemented for [`http::HeaderMap`] (axum, actix-web 4 via `.headers()` conversion,
/// hyper, warp, reqwest), `HashMap<String, String>`, `BTreeMap<String, String>` and
/// slices/arrays/`Vec`s of `(name, value)` pairs. Names are matched case-insensitively.
pub trait WebhookHeaders {
    /// The value of header `name` (lowercase), if present.
    fn header(&self, name: &str) -> Option<&str>;
}

impl WebhookHeaders for http::HeaderMap {
    fn header(&self, name: &str) -> Option<&str> {
        self.get(name).and_then(|v| v.to_str().ok())
    }
}

impl<K: AsRef<str>, V: AsRef<str>, S: BuildHasher> WebhookHeaders for HashMap<K, V, S> {
    fn header(&self, name: &str) -> Option<&str> {
        find(self.iter(), name)
    }
}

impl<K: AsRef<str>, V: AsRef<str>> WebhookHeaders for BTreeMap<K, V> {
    fn header(&self, name: &str) -> Option<&str> {
        find(self.iter(), name)
    }
}

impl<K: AsRef<str>, V: AsRef<str>> WebhookHeaders for [(K, V)] {
    fn header(&self, name: &str) -> Option<&str> {
        find(self.iter().map(|(k, v)| (k, v)), name)
    }
}

impl<K: AsRef<str>, V: AsRef<str>, const N: usize> WebhookHeaders for [(K, V); N] {
    fn header(&self, name: &str) -> Option<&str> {
        self.as_slice().header(name)
    }
}

impl<K: AsRef<str>, V: AsRef<str>> WebhookHeaders for Vec<(K, V)> {
    fn header(&self, name: &str) -> Option<&str> {
        self.as_slice().header(name)
    }
}

impl<T: WebhookHeaders + ?Sized> WebhookHeaders for &T {
    fn header(&self, name: &str) -> Option<&str> {
        (**self).header(name)
    }
}

fn find<'a, K: AsRef<str> + 'a, V: AsRef<str> + 'a>(
    mut pairs: impl Iterator<Item = (&'a K, &'a V)>,
    name: &str,
) -> Option<&'a str> {
    pairs
        .find(|(k, _)| k.as_ref().eq_ignore_ascii_case(name))
        .map(|(_, v)| v.as_ref())
}

/// Options for [`verify_webhook_with`].
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct VerifyOptions {
    /// Reject messages whose timestamp is further than this from now. Default 5 minutes.
    pub tolerance: Duration,
    /// The current time; `None` uses the system clock. For tests.
    pub now: Option<SystemTime>,
}

impl Default for VerifyOptions {
    fn default() -> Self {
        VerifyOptions {
            tolerance: DEFAULT_TOLERANCE,
            now: None,
        }
    }
}

/// Bounce details of an `email.bounced` event.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Bounce {
    /// `Permanent`, `Transient` or `Undetermined`.
    #[serde(rename = "type", default)]
    pub bounce_type: String,
    /// E.g. `General`, `NoEmail`, `Suppressed`.
    #[serde(rename = "subType", default)]
    pub sub_type: String,
    /// The receiving server's explanation.
    #[serde(default)]
    pub message: String,
}

/// Click details of an `email.clicked` event.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Click {
    /// The clicked URL.
    pub link: String,
    /// The clicker's IP address.
    #[serde(rename = "ipAddress", default)]
    pub ip_address: String,
    /// The clicker's user agent.
    #[serde(rename = "userAgent", default)]
    pub user_agent: String,
    /// When the link was clicked.
    #[serde(with = "time::serde::rfc3339")]
    pub timestamp: OffsetDateTime,
}

/// Failure details of an `email.failed` event.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Failure {
    /// Why the email could not be sent.
    pub reason: String,
}

/// The email an event is about.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct WebhookEventData {
    /// The email's id, as returned by `send`. Use it to deduplicate retried deliveries.
    pub email_id: String,
    /// The `Message-ID` of the sent email, once known.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub message_id: Option<String>,
    /// Sender.
    pub from: String,
    /// Recipients.
    #[serde(default, deserialize_with = "string_or_vec")]
    pub to: Vec<String>,
    /// Subject line.
    #[serde(default)]
    pub subject: String,
    /// The email's tags.
    #[serde(default)]
    pub tags: BTreeMap<String, String>,
    /// When the email was accepted.
    #[serde(with = "time::serde::rfc3339")]
    pub created_at: OffsetDateTime,
    /// Only for `email.bounced`.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub bounce: Option<Bounce>,
    /// Only for `email.clicked`.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub click: Option<Click>,
    /// Only for `email.failed`.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub failed: Option<Failure>,
}

/// An event received on your webhook endpoint, as returned by [`verify_webhook`].
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct WebhookEvent {
    /// Event type, e.g. [`WebhookEventType::Delivered`].
    #[serde(rename = "type")]
    pub event_type: WebhookEventType,
    /// When the event happened.
    #[serde(with = "time::serde::rfc3339")]
    pub created_at: OffsetDateTime,
    /// The email it is about.
    pub data: WebhookEventData,
}

fn mac(
    secret: &str,
    id: &str,
    timestamp: &str,
    payload: &[u8],
) -> Result<HmacSha256, WebhookVerificationError> {
    let encoded = secret.strip_prefix("whsec_").unwrap_or(secret);
    let key = BASE64
        .decode(encoded)
        .map_err(|_| WebhookVerificationError::InvalidSecret)?;
    let mut mac =
        HmacSha256::new_from_slice(&key).map_err(|_| WebhookVerificationError::InvalidSecret)?;
    mac.update(id.as_bytes());
    mac.update(b".");
    mac.update(timestamp.as_bytes());
    mac.update(b".");
    mac.update(payload);
    Ok(mac)
}

/// Computes the `webhook-signature` header value (`v1,<base64>`) for a message.
///
/// Useful to build signed requests in your own tests.
pub fn sign_webhook(
    secret: &str,
    id: &str,
    timestamp: i64,
    payload: &[u8],
) -> Result<String, WebhookVerificationError> {
    let tag = mac(secret, id, &timestamp.to_string(), payload)?
        .finalize()
        .into_bytes();
    Ok(format!("v1,{}", BASE64.encode(tag)))
}

/// Checks that a webhook request came from Agdir Email and returns the parsed event.
///
/// Pass the **raw** request body (not re-serialized JSON), the request headers and the
/// webhook's `whsec_…` signing secret. Messages are signed per the Standard Webhooks spec
/// (`webhook-id`, `webhook-timestamp`, `webhook-signature`); signatures are compared in
/// constant time and messages more than 5 minutes old (or ahead) are rejected.
///
/// ```
/// use agdirlabs_email::{sign_webhook, verify_webhook, WebhookEventType};
///
/// let secret = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw";
/// let body = br#"{"type":"email.delivered","created_at":"2026-10-30T10:00:00Z","data":{"email_id":"em_1","from":"a@acme.com","to":["b@example.com"],"subject":"Hi","created_at":"2026-10-30T09:59:58Z"}}"#;
/// let now = std::time::SystemTime::now().duration_since(std::time::UNIX_EPOCH).unwrap().as_secs() as i64;
/// let signature = sign_webhook(secret, "msg_1", now, body)?;
/// let headers = [
///     ("webhook-id", "msg_1".to_string()),
///     ("webhook-timestamp", now.to_string()),
///     ("webhook-signature", signature),
/// ];
///
/// let event = verify_webhook(body, &headers, secret)?;
/// assert_eq!(event.event_type, WebhookEventType::Delivered);
/// assert_eq!(event.data.email_id, "em_1");
/// # Ok::<(), agdirlabs_email::WebhookVerificationError>(())
/// ```
pub fn verify_webhook(
    payload: impl AsRef<[u8]>,
    headers: &(impl WebhookHeaders + ?Sized),
    secret: &str,
) -> Result<WebhookEvent, WebhookVerificationError> {
    verify_webhook_with(payload, headers, secret, VerifyOptions::default())
}

/// [`verify_webhook`] with a custom tolerance or clock.
pub fn verify_webhook_with(
    payload: impl AsRef<[u8]>,
    headers: &(impl WebhookHeaders + ?Sized),
    secret: &str,
    options: VerifyOptions,
) -> Result<WebhookEvent, WebhookVerificationError> {
    let payload = payload.as_ref();
    let (Some(id), Some(timestamp), Some(signatures)) = (
        headers.header("webhook-id").filter(|v| !v.is_empty()),
        headers
            .header("webhook-timestamp")
            .filter(|v| !v.is_empty()),
        headers
            .header("webhook-signature")
            .filter(|v| !v.is_empty()),
    ) else {
        return Err(WebhookVerificationError::MissingHeaders);
    };

    let sent_at: i64 = timestamp
        .trim()
        .parse()
        .map_err(|_| WebhookVerificationError::InvalidTimestamp)?;
    let now = options
        .now
        .unwrap_or_else(SystemTime::now)
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs() as i64)
        .unwrap_or(0);
    let tolerance = i64::try_from(options.tolerance.as_secs()).unwrap_or(i64::MAX);
    if now.abs_diff(sent_at) > tolerance.unsigned_abs() {
        return Err(WebhookVerificationError::InvalidTimestamp);
    }

    let expected = mac(secret, id, timestamp, payload)?;
    // The header may hold several space-separated `v1,<sig>` entries (e.g. during secret rotation).
    let valid = signatures
        .split_whitespace()
        .filter_map(|entry| entry.strip_prefix("v1,"))
        .filter_map(|sig| BASE64.decode(sig).ok())
        .any(|sig| expected.clone().verify_slice(&sig).is_ok());
    if !valid {
        return Err(WebhookVerificationError::InvalidSignature);
    }

    serde_json::from_slice(payload)
        .map_err(|e| WebhookVerificationError::InvalidPayload(e.to_string()))
}
