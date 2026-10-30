//! Errors returned by the client and by webhook verification.

use std::fmt;
use std::time::Duration;

/// Shorthand for `std::result::Result<T, agdirlabs_email::Error>`.
pub type Result<T, E = Error> = std::result::Result<T, E>;

/// Machine-readable error codes returned by the Agdir Email API, plus the client-side
/// [`NetworkError`](ErrorCode::NetworkError) and [`Timeout`](ErrorCode::Timeout).
///
/// New codes may be added at any time; unknown ones arrive as [`ErrorCode::Other`].
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
#[non_exhaustive]
pub enum ErrorCode {
    /// No `Authorization` header was sent (HTTP 401).
    MissingApiKey,
    /// The API key does not exist or was revoked (HTTP 403).
    InvalidApiKey,
    /// A sending-only key was used on an endpoint that needs full access (HTTP 401).
    RestrictedApiKey,
    /// The request is invalid (HTTP 422), or the domain is unverified, the team is paused or
    /// the domain limit is reached (HTTP 403).
    ValidationError,
    /// The resource does not exist (HTTP 404).
    NotFound,
    /// The idempotency key was reused with a different request body (HTTP 409).
    InvalidIdempotentRequest,
    /// Another request with the same idempotency key is still in progress (HTTP 409).
    ConcurrentIdempotentRequests,
    /// The daily sending quota is used up (HTTP 429). Never retried.
    DailyQuotaExceeded,
    /// The monthly sending quota or spend limit is used up (HTTP 429). Never retried.
    MonthlyQuotaExceeded,
    /// Too many requests per second (HTTP 429). Retried automatically.
    RateLimitExceeded,
    /// Something went wrong on our side (HTTP 5xx).
    InternalServerError,
    /// The API could not be reached (DNS, connection, TLS…). Client-side, status 0.
    NetworkError,
    /// No response arrived within the configured timeout. Client-side, status 0.
    Timeout,
    /// A code this version of the SDK does not know about yet.
    Other(String),
}

impl ErrorCode {
    /// Parses an API error `name` such as `"validation_error"`.
    pub fn parse(name: &str) -> Self {
        match name {
            "missing_api_key" => Self::MissingApiKey,
            "invalid_api_key" => Self::InvalidApiKey,
            "restricted_api_key" => Self::RestrictedApiKey,
            "validation_error" => Self::ValidationError,
            "not_found" => Self::NotFound,
            "invalid_idempotent_request" => Self::InvalidIdempotentRequest,
            "concurrent_idempotent_requests" => Self::ConcurrentIdempotentRequests,
            "daily_quota_exceeded" => Self::DailyQuotaExceeded,
            "monthly_quota_exceeded" => Self::MonthlyQuotaExceeded,
            "rate_limit_exceeded" => Self::RateLimitExceeded,
            "internal_server_error" => Self::InternalServerError,
            "network_error" => Self::NetworkError,
            "timeout" => Self::Timeout,
            other => Self::Other(other.to_owned()),
        }
    }

    /// The wire name, e.g. `"rate_limit_exceeded"`.
    pub fn as_str(&self) -> &str {
        match self {
            Self::MissingApiKey => "missing_api_key",
            Self::InvalidApiKey => "invalid_api_key",
            Self::RestrictedApiKey => "restricted_api_key",
            Self::ValidationError => "validation_error",
            Self::NotFound => "not_found",
            Self::InvalidIdempotentRequest => "invalid_idempotent_request",
            Self::ConcurrentIdempotentRequests => "concurrent_idempotent_requests",
            Self::DailyQuotaExceeded => "daily_quota_exceeded",
            Self::MonthlyQuotaExceeded => "monthly_quota_exceeded",
            Self::RateLimitExceeded => "rate_limit_exceeded",
            Self::InternalServerError => "internal_server_error",
            Self::NetworkError => "network_error",
            Self::Timeout => "timeout",
            Self::Other(name) => name,
        }
    }
}

impl fmt::Display for ErrorCode {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

/// Every way a client call can fail.
///
/// Use [`Error::code`], [`Error::status`] and [`Error::retry_after`] for a uniform view, or
/// match on the variants:
///
/// ```
/// # fn handle(err: agdirlabs_email::Error) {
/// use agdirlabs_email::{Error, ErrorCode};
///
/// match err {
///     Error::Api { code: ErrorCode::ValidationError, message, .. } => eprintln!("fix the request: {message}"),
///     Error::Api { code: ErrorCode::DailyQuotaExceeded, .. } => eprintln!("try again tomorrow"),
///     other => eprintln!("email failed: {other}"),
/// }
/// # }
/// ```
#[derive(Debug, thiserror::Error)]
#[non_exhaustive]
pub enum Error {
    /// The API answered with an error status.
    #[error("{message} ({code}, HTTP {status})")]
    Api {
        /// Machine-readable code, e.g. [`ErrorCode::ValidationError`].
        code: ErrorCode,
        /// HTTP status code.
        status: u16,
        /// Human-readable explanation.
        message: String,
        /// How long the API asked us to wait (`retry-after`), if it said.
        retry_after: Option<Duration>,
    },
    /// The API could not be reached (DNS, connection refused, TLS, connection reset…).
    #[error("Could not reach the Agdir Email API: {source}")]
    Network {
        /// The underlying HTTP client error.
        #[source]
        source: reqwest::Error,
    },
    /// No complete response arrived within the per-attempt timeout.
    #[error("Request timed out after {} ms", timeout.as_millis())]
    Timeout {
        /// The configured per-attempt timeout.
        timeout: Duration,
    },
    /// A successful response could not be decoded. Usually means the base URL does not point
    /// at the Agdir Email API.
    #[error("Could not decode the API response (HTTP {status}): {source}")]
    Decode {
        /// HTTP status of the response.
        status: u16,
        /// The JSON error.
        #[source]
        source: serde_json::Error,
    },
    /// The client is misconfigured, e.g. no API key was given.
    #[error("{0}")]
    Config(String),
}

impl Error {
    /// The error code: the API's `name`, or [`ErrorCode::NetworkError`] /
    /// [`ErrorCode::Timeout`] for client-side failures. `None` for decode and
    /// configuration errors.
    pub fn code(&self) -> Option<&ErrorCode> {
        static NETWORK: ErrorCode = ErrorCode::NetworkError;
        static TIMEOUT: ErrorCode = ErrorCode::Timeout;
        match self {
            Self::Api { code, .. } => Some(code),
            Self::Network { .. } => Some(&NETWORK),
            Self::Timeout { .. } => Some(&TIMEOUT),
            _ => None,
        }
    }

    /// HTTP status, or 0 when no response was received.
    pub fn status(&self) -> u16 {
        match self {
            Self::Api { status, .. } | Self::Decode { status, .. } => *status,
            _ => 0,
        }
    }

    /// How long the API asked us to wait before retrying, if it said.
    pub fn retry_after(&self) -> Option<Duration> {
        match self {
            Self::Api { retry_after, .. } => *retry_after,
            _ => None,
        }
    }

    /// `true` if this is an API error with the given code.
    pub fn is(&self, code: &ErrorCode) -> bool {
        self.code() == Some(code)
    }
}

/// Returned by [`verify_webhook`](crate::verify_webhook) when a request's signature,
/// timestamp or body is invalid. Answer such requests with HTTP 400.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
#[non_exhaustive]
pub enum WebhookVerificationError {
    /// `webhook-id`, `webhook-timestamp` or `webhook-signature` is missing.
    #[error("Missing webhook signature headers")]
    MissingHeaders,
    /// The timestamp is not a number or is too far from the current time.
    #[error("Webhook timestamp is outside the allowed tolerance")]
    InvalidTimestamp,
    /// The signing secret is not `whsec_` followed by base64.
    #[error("Signing secret is not valid base64")]
    InvalidSecret,
    /// No `v1` signature matches.
    #[error("Webhook signature does not match")]
    InvalidSignature,
    /// The signature is valid but the body is not a webhook event.
    #[error("Webhook payload is not valid JSON: {0}")]
    InvalidPayload(String),
}
