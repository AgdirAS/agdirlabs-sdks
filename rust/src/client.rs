//! The [`AgdirEmail`] client, its builder and the shared request/retry logic.

use std::fmt;
use std::sync::Arc;
use std::time::{Duration, SystemTime};

use reqwest::{Method, StatusCode, Url};
use serde::de::DeserializeOwned;
use serde::Serialize;

use crate::domains::Domains;
use crate::emails::Emails;
use crate::error::{Error, ErrorCode, Result};
use crate::webhooks::Webhooks;

/// The production API: `https://api.agdir.email`.
pub const DEFAULT_BASE_URL: &str = "https://api.agdir.email";
/// This crate's version.
pub const VERSION: &str = env!("CARGO_PKG_VERSION");
/// The `User-Agent` sent with every request: `agdirlabs-email-rust/<version>`.
pub const USER_AGENT: &str = concat!("agdirlabs-email-rust/", env!("CARGO_PKG_VERSION"));

const API_KEY_ENV: &str = "AGDIR_EMAIL_API_KEY";
const BASE_URL_ENV: &str = "AGDIR_EMAIL_BASE_URL";
const DEFAULT_MAX_RETRIES: u32 = 2;
const DEFAULT_TIMEOUT: Duration = Duration::from_secs(30);
const RETRY_STATUSES: [u16; 5] = [408, 500, 502, 503, 504];

/// Agdir Email API client.
///
/// Cheap to clone (it is an `Arc` inside): create one and share it.
///
/// ```no_run
/// # async fn run() -> agdirlabs_email::Result<()> {
/// use agdirlabs_email::{AgdirEmail, SendEmail};
///
/// let agdir = AgdirEmail::new("re_123")?;
/// let sent = agdir
///     .emails()
///     .send(SendEmail::new("Acme <hello@acme.com>", "user@example.com", "Hi").text("Hello!"))
///     .await?;
/// println!("{}", sent.id);
/// # Ok(()) }
/// ```
#[derive(Clone)]
pub struct AgdirEmail {
    pub(crate) inner: Arc<Inner>,
}

pub(crate) struct Inner {
    api_key: String,
    base_url: Url,
    http: reqwest::Client,
    pub(crate) max_retries: u32,
    timeout: Duration,
}

impl fmt::Debug for AgdirEmail {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("AgdirEmail")
            .field("base_url", &self.inner.base_url.as_str())
            .field("max_retries", &self.inner.max_retries)
            .field("timeout", &self.inner.timeout)
            .finish_non_exhaustive()
    }
}

impl AgdirEmail {
    /// A client for `api_key` with default settings. The base URL comes from
    /// `AGDIR_EMAIL_BASE_URL` if set, else [`DEFAULT_BASE_URL`].
    pub fn new(api_key: impl Into<String>) -> Result<Self> {
        Self::builder().api_key(api_key).build()
    }

    /// A client configured from the environment: `AGDIR_EMAIL_API_KEY` (required) and
    /// `AGDIR_EMAIL_BASE_URL` (optional).
    pub fn from_env() -> Result<Self> {
        Self::builder().build()
    }

    /// Starts a [`ClientBuilder`] for custom retries, timeouts, base URL or HTTP client.
    pub fn builder() -> ClientBuilder {
        ClientBuilder::default()
    }

    /// The base URL requests are sent to.
    pub fn base_url(&self) -> &str {
        self.inner.base_url.as_str().trim_end_matches('/')
    }

    /// Send, read and cancel emails.
    pub fn emails(&self) -> Emails<'_> {
        Emails { client: self }
    }

    /// Manage sending domains.
    pub fn domains(&self) -> Domains<'_> {
        Domains { client: self }
    }

    /// Manage webhook endpoints and inspect their deliveries.
    pub fn webhooks(&self) -> Webhooks<'_> {
        Webhooks { client: self }
    }
}

/// Configures an [`AgdirEmail`] client.
///
/// ```no_run
/// use std::time::Duration;
/// use agdirlabs_email::AgdirEmail;
///
/// let agdir = AgdirEmail::builder()
///     .api_key("re_123")
///     .base_url("https://api.agdir.email")
///     .max_retries(3)
///     .timeout(Duration::from_secs(10))
///     .build()?;
/// # Ok::<(), agdirlabs_email::Error>(())
/// ```
#[derive(Debug, Default)]
pub struct ClientBuilder {
    api_key: Option<String>,
    base_url: Option<String>,
    max_retries: Option<u32>,
    timeout: Option<Duration>,
    http_client: Option<reqwest::Client>,
}

fn env_var(name: &str) -> Option<String> {
    // Empty values (e.g. `AGDIR_EMAIL_BASE_URL=` in a .env file) count as unset.
    std::env::var(name).ok().filter(|v| !v.is_empty())
}

impl ClientBuilder {
    /// The API key (`re_…`). Defaults to the `AGDIR_EMAIL_API_KEY` environment variable.
    pub fn api_key(mut self, api_key: impl Into<String>) -> Self {
        self.api_key = Some(api_key.into());
        self
    }

    /// The API base URL. Defaults to `AGDIR_EMAIL_BASE_URL`, else [`DEFAULT_BASE_URL`].
    pub fn base_url(mut self, base_url: impl Into<String>) -> Self {
        self.base_url = Some(base_url.into());
        self
    }

    /// Retries for rate limits, server errors and network failures. Default 2; 0 disables.
    pub fn max_retries(mut self, max_retries: u32) -> Self {
        self.max_retries = Some(max_retries);
        self
    }

    /// Timeout for each attempt (not the whole call including retries). Default 30 s.
    pub fn timeout(mut self, timeout: Duration) -> Self {
        self.timeout = Some(timeout);
        self
    }

    /// Use your own `reqwest::Client` (proxies, custom TLS roots, connection pool sharing).
    /// The per-attempt [`timeout`](Self::timeout) still applies.
    pub fn http_client(mut self, client: reqwest::Client) -> Self {
        self.http_client = Some(client);
        self
    }

    /// Builds the client. Fails if no API key is configured or the base URL is invalid.
    pub fn build(self) -> Result<AgdirEmail> {
        let api_key = self
            .api_key
            .filter(|k| !k.is_empty())
            .or_else(|| env_var(API_KEY_ENV))
            .ok_or_else(|| {
                Error::Config(format!(
                    "Missing API key: call .api_key(...) or set {API_KEY_ENV}. Create one at https://app.agdir.email."
                ))
            })?;
        let raw_base = self
            .base_url
            .filter(|u| !u.is_empty())
            .or_else(|| env_var(BASE_URL_ENV))
            .unwrap_or_else(|| DEFAULT_BASE_URL.to_owned());
        let base_url = Url::parse(raw_base.trim_end_matches('/'))
            .ok()
            .filter(|u| !u.cannot_be_a_base() && matches!(u.scheme(), "http" | "https"))
            .ok_or_else(|| Error::Config(format!("Invalid base URL: {raw_base}")))?;
        let http = match self.http_client {
            Some(client) => client,
            None => reqwest::Client::builder()
                .user_agent(USER_AGENT)
                .build()
                .map_err(|e| Error::Config(format!("Could not create the HTTP client: {e}")))?,
        };
        Ok(AgdirEmail {
            inner: Arc::new(Inner {
                api_key,
                base_url,
                http,
                max_retries: self.max_retries.unwrap_or(DEFAULT_MAX_RETRIES),
                timeout: self.timeout.unwrap_or(DEFAULT_TIMEOUT),
            }),
        })
    }
}

/// One API call, before retries.
pub(crate) struct Request<'a> {
    pub method: Method,
    pub path: &'a [&'a str],
    pub query: Vec<(&'static str, String)>,
    pub body: Option<serde_json::Value>,
    pub headers: Vec<(&'static str, String)>,
    /// Safe to repeat: GETs, and writes that are idempotent or carry an idempotency key.
    pub retryable: bool,
}

impl<'a> Request<'a> {
    pub fn new(method: Method, path: &'a [&'a str], retryable: bool) -> Self {
        Request {
            method,
            path,
            query: Vec::new(),
            body: None,
            headers: Vec::new(),
            retryable,
        }
    }

    pub fn get(path: &'a [&'a str]) -> Self {
        Self::new(Method::GET, path, true)
    }

    pub fn json(mut self, body: &impl Serialize) -> Self {
        // Serializing our own request types cannot fail.
        self.body = Some(serde_json::to_value(body).expect("request body serializes"));
        self
    }

    pub fn header(mut self, name: &'static str, value: Option<String>) -> Self {
        if let Some(value) = value {
            self.headers.push((name, value));
        }
        self
    }

    pub fn list_query(mut self, limit: Option<u32>, after: Option<&str>) -> Self {
        if let Some(limit) = limit {
            self.query.push(("limit", limit.to_string()));
        }
        if let Some(after) = after {
            self.query.push(("after", after.to_owned()));
        }
        self
    }
}

/// Exponential backoff with jitter: ~0.5 s, 1 s, 2 s … capped at 8 s.
fn backoff(attempt: u32) -> Duration {
    let base_ms = 8000u64.min(500u64.saturating_mul(1u64 << attempt.min(16)));
    Duration::from_millis(base_ms / 2 + fastrand::u64(0..=base_ms / 2))
}

pub(crate) fn parse_retry_after(raw: &str) -> Option<Duration> {
    let raw = raw.trim();
    if let Ok(seconds) = raw.parse::<f64>() {
        return (seconds.is_finite()).then(|| Duration::from_secs_f64(seconds.max(0.0)));
    }
    let date = httpdate::parse_http_date(raw).ok()?;
    Some(date.duration_since(SystemTime::now()).unwrap_or_default())
}

fn api_error(status: StatusCode, retry_after: Option<Duration>, body: &[u8]) -> Error {
    #[derive(serde::Deserialize)]
    struct Body {
        name: Option<String>,
        message: Option<String>,
    }
    let body: Option<Body> = serde_json::from_slice(body).ok();
    let (name, message) = body.map(|b| (b.name, b.message)).unwrap_or_default();
    let code = match name {
        Some(name) => ErrorCode::parse(&name),
        None if status.is_server_error() => ErrorCode::InternalServerError,
        None => ErrorCode::Other("unknown_error".to_owned()),
    };
    Error::Api {
        code,
        status: status.as_u16(),
        message: message.unwrap_or_else(|| format!("Request failed with HTTP {}", status.as_u16())),
        retry_after,
    }
}

impl AgdirEmail {
    fn url(&self, req: &Request<'_>) -> Url {
        let mut url = self.inner.base_url.clone();
        url.path_segments_mut()
            .expect("base URL was validated in build()")
            .pop_if_empty()
            .extend(req.path);
        if !req.query.is_empty() {
            url.query_pairs_mut().extend_pairs(&req.query);
        }
        url
    }

    /// Sends one API request, retrying rate limits, server errors and network failures when safe.
    pub(crate) async fn request<T: DeserializeOwned>(&self, req: Request<'_>) -> Result<T> {
        let inner = &*self.inner;
        let url = self.url(&req);
        let body = req
            .body
            .as_ref()
            .map(|b| serde_json::to_vec(b).expect("JSON value serializes"));

        let mut attempt: u32 = 0;
        loop {
            let can_retry = attempt < inner.max_retries;
            let mut builder = inner
                .http
                .request(req.method.clone(), url.clone())
                .bearer_auth(&inner.api_key)
                .header(reqwest::header::USER_AGENT, USER_AGENT)
                .header(reqwest::header::ACCEPT, "application/json")
                .timeout(inner.timeout);
            for (name, value) in &req.headers {
                builder = builder.header(*name, value);
            }
            if let Some(body) = &body {
                builder = builder
                    .header(reqwest::header::CONTENT_TYPE, "application/json")
                    .body(body.clone());
            }

            let outcome = match builder.send().await {
                Ok(res) => {
                    let status = res.status();
                    let retry_after = res
                        .headers()
                        .get(reqwest::header::RETRY_AFTER)
                        .and_then(|v| v.to_str().ok())
                        .and_then(parse_retry_after);
                    res.bytes().await.map(|bytes| (status, retry_after, bytes))
                }
                Err(err) => Err(err),
            };

            let (status, retry_after, bytes) = match outcome {
                Ok(parts) => parts,
                Err(err) => {
                    // A timed-out POST may have been accepted: only retry it when it is idempotent.
                    if can_retry && req.retryable {
                        tokio::time::sleep(backoff(attempt)).await;
                        attempt += 1;
                        continue;
                    }
                    return Err(if err.is_timeout() {
                        Error::Timeout {
                            timeout: inner.timeout,
                        }
                    } else {
                        Error::Network { source: err }
                    });
                }
            };

            if status.is_success() {
                return serde_json::from_slice(&bytes).map_err(|source| Error::Decode {
                    status: status.as_u16(),
                    source,
                });
            }

            let error = api_error(status, retry_after, &bytes);
            // Rate limits are refused before any work is done, so they are always safe to retry.
            let retry = can_retry
                && (error.is(&ErrorCode::RateLimitExceeded)
                    || (req.retryable && RETRY_STATUSES.contains(&status.as_u16())));
            if !retry {
                return Err(error);
            }
            tokio::time::sleep(retry_after.unwrap_or_else(|| backoff(attempt))).await;
            attempt += 1;
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn backoff_grows_and_is_capped() {
        for attempt in 0..20 {
            let d = backoff(attempt);
            let base = 8000u64.min(500 * (1u64 << attempt.min(16)));
            assert!(d.as_millis() as u64 >= base / 2 && d.as_millis() as u64 <= base);
        }
    }

    #[test]
    fn parses_retry_after() {
        assert_eq!(parse_retry_after("2"), Some(Duration::from_secs(2)));
        assert_eq!(parse_retry_after("0.5"), Some(Duration::from_millis(500)));
        assert_eq!(
            parse_retry_after("Sun, 06 Nov 1994 08:49:37 GMT"),
            Some(Duration::ZERO)
        );
        assert_eq!(parse_retry_after("soon"), None);
    }
}
