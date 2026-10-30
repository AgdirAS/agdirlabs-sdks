//! Sending, reading and canceling emails.

use std::collections::BTreeMap;

use reqwest::Method;
use serde::{Deserialize, Serialize};
use time::OffsetDateTime;

use crate::client::{AgdirEmail, Request};
use crate::error::Result;
use crate::serde_util::{null_to_empty, string_or_vec};
use crate::ObjectId;

/// The most emails one [`Emails::send_batch`] call accepts.
pub const MAX_BATCH_SIZE: usize = 100;

/// Anything that can be turned into a list of email addresses: a single `&str`/`String`,
/// or a `Vec`, array or slice of them.
///
/// Addresses are `"Name <addr@example.com>"` or bare `"addr@example.com"`.
pub trait IntoAddresses {
    /// Converts into a list of addresses.
    fn into_addresses(self) -> Vec<String>;
}

impl IntoAddresses for &str {
    fn into_addresses(self) -> Vec<String> {
        vec![self.to_owned()]
    }
}

impl IntoAddresses for String {
    fn into_addresses(self) -> Vec<String> {
        vec![self]
    }
}

impl IntoAddresses for &String {
    fn into_addresses(self) -> Vec<String> {
        vec![self.clone()]
    }
}

impl<T: Into<String>> IntoAddresses for Vec<T> {
    fn into_addresses(self) -> Vec<String> {
        self.into_iter().map(Into::into).collect()
    }
}

impl<T: Into<String>, const N: usize> IntoAddresses for [T; N] {
    fn into_addresses(self) -> Vec<String> {
        self.into_iter().map(Into::into).collect()
    }
}

impl<T: AsRef<str>> IntoAddresses for &[T] {
    fn into_addresses(self) -> Vec<String> {
        self.iter().map(|a| a.as_ref().to_owned()).collect()
    }
}

/// A tag attached to an email and echoed back in webhook events.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Tag {
    /// ASCII letters, digits, `_` and `-` only.
    pub name: String,
    /// ASCII letters, digits, `_` and `-` only.
    pub value: String,
}

/// An email to send. Build it with [`SendEmail::new`] and the chainable setters.
///
/// At least one of [`html`](Self::html) and [`text`](Self::text) is required.
///
/// ```
/// use agdirlabs_email::SendEmail;
///
/// let email = SendEmail::new("Acme <hello@acme.com>", ["ana@example.com", "ben@example.com"], "Your invoice")
///     .text("Invoice #1042 is ready.")
///     .cc("finance@acme.com")
///     .reply_to("support@acme.com")
///     .tag("kind", "invoice")
///     .header("X-Entity-Ref-ID", "inv-1042");
/// ```
#[derive(Debug, Clone, Default, PartialEq, Serialize)]
pub struct SendEmail {
    /// Sender, `"Name <addr@your-domain.com>"` or a bare address on a verified domain.
    pub from: String,
    /// Recipients.
    pub to: Vec<String>,
    /// Subject line.
    pub subject: String,
    /// HTML body.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub html: Option<String>,
    /// Plain-text body.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub text: Option<String>,
    /// Carbon-copy recipients.
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub cc: Vec<String>,
    /// Blind carbon-copy recipients.
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub bcc: Vec<String>,
    /// Reply-To addresses.
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub reply_to: Vec<String>,
    /// Extra MIME headers.
    #[serde(skip_serializing_if = "BTreeMap::is_empty")]
    pub headers: BTreeMap<String, String>,
    /// Tags, echoed back in webhook events.
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub tags: Vec<Tag>,
    /// Deliver later instead of now.
    #[serde(
        with = "time::serde::rfc3339::option",
        skip_serializing_if = "Option::is_none"
    )]
    pub scheduled_at: Option<OffsetDateTime>,
}

impl SendEmail {
    /// A new email from `from` to `to` (one address or several) with `subject`.
    pub fn new(
        from: impl Into<String>,
        to: impl IntoAddresses,
        subject: impl Into<String>,
    ) -> Self {
        SendEmail {
            from: from.into(),
            to: to.into_addresses(),
            subject: subject.into(),
            ..Self::default()
        }
    }

    /// Sets the HTML body.
    pub fn html(mut self, html: impl Into<String>) -> Self {
        self.html = Some(html.into());
        self
    }

    /// Sets the plain-text body.
    pub fn text(mut self, text: impl Into<String>) -> Self {
        self.text = Some(text.into());
        self
    }

    /// Adds Cc recipients.
    pub fn cc(mut self, cc: impl IntoAddresses) -> Self {
        self.cc.extend(cc.into_addresses());
        self
    }

    /// Adds Bcc recipients.
    pub fn bcc(mut self, bcc: impl IntoAddresses) -> Self {
        self.bcc.extend(bcc.into_addresses());
        self
    }

    /// Adds Reply-To addresses.
    pub fn reply_to(mut self, reply_to: impl IntoAddresses) -> Self {
        self.reply_to.extend(reply_to.into_addresses());
        self
    }

    /// Adds a MIME header.
    pub fn header(mut self, name: impl Into<String>, value: impl Into<String>) -> Self {
        self.headers.insert(name.into(), value.into());
        self
    }

    /// Adds a tag (ASCII letters, digits, `_` and `-`).
    pub fn tag(mut self, name: impl Into<String>, value: impl Into<String>) -> Self {
        self.tags.push(Tag {
            name: name.into(),
            value: value.into(),
        });
        self
    }

    /// Schedules delivery, e.g. `OffsetDateTime::now_utc() + time::Duration::hours(1)`.
    pub fn scheduled_at(mut self, at: OffsetDateTime) -> Self {
        self.scheduled_at = Some(at);
        self
    }
}

/// Options for [`Emails::send_with`].
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct SendOptions {
    /// Makes the send safe to repeat: the same key with the same body returns the original
    /// result instead of sending again. The client generates one automatically when retries
    /// are enabled; pass your own (e.g. `welcome/<user id>`) to be safe across process restarts.
    pub idempotency_key: Option<String>,
}

impl SendOptions {
    /// Default options.
    pub fn new() -> Self {
        Self::default()
    }

    /// Sets the idempotency key.
    pub fn idempotency_key(mut self, key: impl Into<String>) -> Self {
        self.idempotency_key = Some(key.into());
        self
    }
}

/// How [`Emails::send_batch_with`] treats invalid emails.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
#[non_exhaustive]
pub enum BatchValidation {
    /// One invalid email rejects the whole batch with a `validation_error` (default).
    #[default]
    Strict,
    /// Valid emails are accepted; invalid ones are reported in [`BatchResult::errors`].
    Permissive,
}

/// Options for [`Emails::send_batch_with`].
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct BatchOptions {
    /// See [`SendOptions::idempotency_key`].
    pub idempotency_key: Option<String>,
    /// Strict (default) or permissive validation.
    pub validation: BatchValidation,
}

impl BatchOptions {
    /// Default options: strict validation, generated idempotency key.
    pub fn new() -> Self {
        Self::default()
    }

    /// Sets the idempotency key.
    pub fn idempotency_key(mut self, key: impl Into<String>) -> Self {
        self.idempotency_key = Some(key.into());
        self
    }

    /// Accept the valid emails and report the invalid ones instead of rejecting the batch.
    pub fn permissive(mut self) -> Self {
        self.validation = BatchValidation::Permissive;
        self
    }
}

/// The result of a send.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct SendResult {
    /// The email's id, for [`Emails::get`] and [`Emails::cancel`].
    pub id: String,
}

/// An email in a permissive batch that was not accepted.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct BatchError {
    /// Position in the input list.
    pub index: usize,
    /// Why it was rejected.
    pub message: String,
}

/// The result of [`Emails::send_batch`].
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct BatchResult {
    /// Accepted emails, in input order.
    pub data: Vec<SendResult>,
    /// Only with permissive validation: emails that were not accepted.
    #[serde(default, deserialize_with = "null_to_empty")]
    pub errors: Vec<BatchError>,
}

/// Delivery state of an email.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
#[non_exhaustive]
pub enum EmailStatus {
    /// Accepted, waiting to be sent.
    Queued,
    /// Waiting for its `scheduled_at` time.
    Scheduled,
    /// Handed to the receiving mail server.
    Sent,
    /// The receiving server is deferring it; delivery is being retried.
    DeliveryDelayed,
    /// Accepted by the receiving server.
    Delivered,
    /// Opened by the recipient.
    Opened,
    /// A link in it was clicked.
    Clicked,
    /// Bounced permanently.
    Bounced,
    /// Could not be sent.
    Failed,
    /// Marked as spam by the recipient.
    Complained,
    /// Canceled before it was sent.
    Canceled,
    /// A status this version of the SDK does not know about yet.
    #[serde(other)]
    Unknown,
}

/// An email and its delivery status, from [`Emails::get`].
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Email {
    /// Email id.
    pub id: String,
    /// Sender.
    pub from: String,
    /// Recipients.
    #[serde(deserialize_with = "string_or_vec")]
    pub to: Vec<String>,
    /// Cc recipients.
    #[serde(default, deserialize_with = "string_or_vec")]
    pub cc: Vec<String>,
    /// Bcc recipients.
    #[serde(default, deserialize_with = "string_or_vec")]
    pub bcc: Vec<String>,
    /// Reply-To addresses.
    #[serde(default, deserialize_with = "string_or_vec")]
    pub reply_to: Vec<String>,
    /// Subject line.
    pub subject: String,
    /// HTML body.
    #[serde(default)]
    pub html: Option<String>,
    /// Plain-text body.
    #[serde(default)]
    pub text: Option<String>,
    /// When the email was accepted.
    #[serde(with = "time::serde::rfc3339")]
    pub created_at: OffsetDateTime,
    /// When it is (or was) scheduled for.
    #[serde(default, with = "time::serde::rfc3339::option")]
    pub scheduled_at: Option<OffsetDateTime>,
    /// Most advanced delivery state reached so far.
    #[serde(rename = "last_event")]
    pub status: EmailStatus,
}

/// The `emails` resource. Get it with [`AgdirEmail::emails`].
#[derive(Debug, Clone, Copy)]
pub struct Emails<'a> {
    pub(crate) client: &'a AgdirEmail,
}

impl Emails<'_> {
    /// Accepts one email for delivery (or for its `scheduled_at` time).
    pub async fn send(&self, email: SendEmail) -> Result<SendResult> {
        self.send_with(email, SendOptions::default()).await
    }

    /// [`send`](Self::send) with options, e.g. your own idempotency key.
    pub async fn send_with(&self, email: SendEmail, options: SendOptions) -> Result<SendResult> {
        let key = self.idempotency_key(options.idempotency_key);
        let req = Request::new(Method::POST, &["emails"], key.is_some())
            .json(&email)
            .header("idempotency-key", key);
        self.client.request(req).await
    }

    /// Accepts up to [`MAX_BATCH_SIZE`] emails in one request. One invalid email rejects the
    /// whole batch; see [`send_batch_with`](Self::send_batch_with) for permissive validation.
    pub async fn send_batch(&self, emails: Vec<SendEmail>) -> Result<BatchResult> {
        self.send_batch_with(emails, BatchOptions::default()).await
    }

    /// [`send_batch`](Self::send_batch) with options.
    pub async fn send_batch_with(
        &self,
        emails: Vec<SendEmail>,
        options: BatchOptions,
    ) -> Result<BatchResult> {
        let key = self.idempotency_key(options.idempotency_key);
        let validation = match options.validation {
            BatchValidation::Permissive => Some("permissive".to_owned()),
            BatchValidation::Strict => None,
        };
        let req = Request::new(Method::POST, &["emails", "batch"], key.is_some())
            .json(&emails)
            .header("idempotency-key", key)
            .header("x-batch-validation", validation);
        self.client.request(req).await
    }

    /// Reads an email and its current delivery status. Requires a full-access key.
    pub async fn get(&self, id: &str) -> Result<Email> {
        self.client.request(Request::get(&["emails", id])).await
    }

    /// Cancels a scheduled email that has not been sent yet.
    pub async fn cancel(&self, id: &str) -> Result<ObjectId> {
        let path = ["emails", id, "cancel"];
        self.client
            .request(Request::new(Method::POST, &path, true))
            .await
    }

    fn idempotency_key(&self, key: Option<String>) -> Option<String> {
        // Without a key a retried send could be delivered twice, so only generate one when we may retry.
        key.filter(|k| !k.is_empty()).or_else(|| {
            (self.client.inner.max_retries > 0).then(|| uuid::Uuid::new_v4().to_string())
        })
    }
}
