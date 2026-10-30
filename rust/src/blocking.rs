//! A synchronous client, for scripts, CLIs and non-async web frameworks.
//!
//! Enabled with the `blocking` feature. It has the same methods as the async client, minus
//! `.await`; `list_all` returns an [`Iterator`].
//!
//! ```no_run
//! use agdirlabs_email::blocking::AgdirEmail;
//! use agdirlabs_email::SendEmail;
//!
//! let agdir = AgdirEmail::from_env()?;
//! let sent = agdir.emails().send(
//!     SendEmail::new("Acme <hello@acme.com>", "user@example.com", "Hi").text("Hello!"),
//! )?;
//! println!("{}", sent.id);
//! # Ok::<(), agdirlabs_email::Error>(())
//! ```
//!
//! It runs requests on a private single-threaded Tokio runtime, so **do not call it from inside
//! an async runtime** (it panics there, like `reqwest::blocking`); use the async client instead.

use std::sync::Arc;
use std::time::Duration;

use futures_core::Stream;
use futures_util::StreamExt;
use tokio::runtime::Runtime;

use crate::error::{Error, Result};
use crate::{
    BatchOptions, BatchResult, CreateDomain, CreateWebhook, Deleted, Domain, Email, ListOptions,
    ObjectId, Page, SendEmail, SendOptions, SendResult, UpdateWebhook, Webhook, WebhookAttempt,
    WebhookDelivery, WebhookSecret,
};

/// Synchronous Agdir Email API client. Cheap to clone.
#[derive(Clone, Debug)]
pub struct AgdirEmail {
    inner: crate::AgdirEmail,
    runtime: Arc<Runtime>,
}

/// Configures a blocking [`AgdirEmail`]; same options as [`crate::ClientBuilder`].
#[derive(Debug, Default)]
pub struct ClientBuilder {
    inner: crate::ClientBuilder,
}

impl ClientBuilder {
    /// The API key (`re_…`). Defaults to `AGDIR_EMAIL_API_KEY`.
    pub fn api_key(self, api_key: impl Into<String>) -> Self {
        ClientBuilder {
            inner: self.inner.api_key(api_key),
        }
    }

    /// The API base URL. Defaults to `AGDIR_EMAIL_BASE_URL`, else `https://api.agdir.email`.
    pub fn base_url(self, base_url: impl Into<String>) -> Self {
        ClientBuilder {
            inner: self.inner.base_url(base_url),
        }
    }

    /// Retries for rate limits, server errors and network failures. Default 2; 0 disables.
    pub fn max_retries(self, max_retries: u32) -> Self {
        ClientBuilder {
            inner: self.inner.max_retries(max_retries),
        }
    }

    /// Timeout for each attempt. Default 30 s.
    pub fn timeout(self, timeout: Duration) -> Self {
        ClientBuilder {
            inner: self.inner.timeout(timeout),
        }
    }

    /// Use your own (async) `reqwest::Client`.
    pub fn http_client(self, client: reqwest::Client) -> Self {
        ClientBuilder {
            inner: self.inner.http_client(client),
        }
    }

    /// Builds the client.
    pub fn build(self) -> Result<AgdirEmail> {
        let inner = self.inner.build()?;
        let runtime = tokio::runtime::Builder::new_current_thread()
            .enable_all()
            .build()
            .map_err(|e| Error::Config(format!("Could not start the Tokio runtime: {e}")))?;
        Ok(AgdirEmail {
            inner,
            runtime: Arc::new(runtime),
        })
    }
}

impl AgdirEmail {
    /// A client for `api_key` with default settings.
    pub fn new(api_key: impl Into<String>) -> Result<Self> {
        Self::builder().api_key(api_key).build()
    }

    /// A client configured from `AGDIR_EMAIL_API_KEY` and `AGDIR_EMAIL_BASE_URL`.
    pub fn from_env() -> Result<Self> {
        Self::builder().build()
    }

    /// Starts a [`ClientBuilder`].
    pub fn builder() -> ClientBuilder {
        ClientBuilder::default()
    }

    /// The base URL requests are sent to.
    pub fn base_url(&self) -> &str {
        self.inner.base_url()
    }

    /// The async client this one wraps.
    pub fn async_client(&self) -> &crate::AgdirEmail {
        &self.inner
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

    fn block_on<F: std::future::Future>(&self, future: F) -> F::Output {
        self.runtime.block_on(future)
    }

    fn iter<T: Send + 'static>(
        &self,
        stream: impl Stream<Item = Result<T>> + Send + 'static,
    ) -> impl Iterator<Item = Result<T>> + '_ {
        let mut stream = Box::pin(stream);
        std::iter::from_fn(move || self.block_on(stream.next()))
    }
}

/// Blocking `emails` resource.
#[derive(Debug, Clone, Copy)]
pub struct Emails<'a> {
    client: &'a AgdirEmail,
}

impl Emails<'_> {
    /// See [`crate::Emails::send`].
    pub fn send(&self, email: SendEmail) -> Result<SendResult> {
        self.client.block_on(self.client.inner.emails().send(email))
    }

    /// See [`crate::Emails::send_with`].
    pub fn send_with(&self, email: SendEmail, options: SendOptions) -> Result<SendResult> {
        self.client
            .block_on(self.client.inner.emails().send_with(email, options))
    }

    /// See [`crate::Emails::send_batch`].
    pub fn send_batch(&self, emails: Vec<SendEmail>) -> Result<BatchResult> {
        self.client
            .block_on(self.client.inner.emails().send_batch(emails))
    }

    /// See [`crate::Emails::send_batch_with`].
    pub fn send_batch_with(
        &self,
        emails: Vec<SendEmail>,
        options: BatchOptions,
    ) -> Result<BatchResult> {
        self.client
            .block_on(self.client.inner.emails().send_batch_with(emails, options))
    }

    /// See [`crate::Emails::get`].
    pub fn get(&self, id: &str) -> Result<Email> {
        self.client.block_on(self.client.inner.emails().get(id))
    }

    /// See [`crate::Emails::cancel`].
    pub fn cancel(&self, id: &str) -> Result<ObjectId> {
        self.client.block_on(self.client.inner.emails().cancel(id))
    }
}

/// Blocking `domains` resource.
#[derive(Debug, Clone, Copy)]
pub struct Domains<'a> {
    client: &'a AgdirEmail,
}

impl<'a> Domains<'a> {
    /// See [`crate::Domains::create`].
    pub fn create(&self, domain: impl Into<CreateDomain>) -> Result<Domain> {
        self.client
            .block_on(self.client.inner.domains().create(domain))
    }

    /// See [`crate::Domains::get`].
    pub fn get(&self, id: &str) -> Result<Domain> {
        self.client.block_on(self.client.inner.domains().get(id))
    }

    /// See [`crate::Domains::list`].
    pub fn list(&self, options: ListOptions) -> Result<Page<Domain>> {
        self.client
            .block_on(self.client.inner.domains().list(options))
    }

    /// Every domain, fetching further pages as you iterate.
    pub fn list_all(&self, options: ListOptions) -> impl Iterator<Item = Result<Domain>> + 'a {
        let client = self.client;
        client.iter(client.inner.domains().list_all(options))
    }

    /// See [`crate::Domains::verify`].
    pub fn verify(&self, id: &str) -> Result<ObjectId> {
        self.client.block_on(self.client.inner.domains().verify(id))
    }

    /// See [`crate::Domains::delete`].
    pub fn delete(&self, id: &str) -> Result<Deleted> {
        self.client.block_on(self.client.inner.domains().delete(id))
    }
}

/// Blocking `webhooks` resource.
#[derive(Debug, Clone, Copy)]
pub struct Webhooks<'a> {
    client: &'a AgdirEmail,
}

impl<'a> Webhooks<'a> {
    /// Deliveries of your endpoints.
    pub fn deliveries(&self) -> WebhookDeliveries<'a> {
        WebhookDeliveries {
            client: self.client,
        }
    }

    /// See [`crate::Webhooks::create`].
    pub fn create(&self, webhook: CreateWebhook) -> Result<WebhookSecret> {
        self.client
            .block_on(self.client.inner.webhooks().create(webhook))
    }

    /// See [`crate::Webhooks::get`].
    pub fn get(&self, id: &str) -> Result<Webhook> {
        self.client.block_on(self.client.inner.webhooks().get(id))
    }

    /// See [`crate::Webhooks::list`].
    pub fn list(&self, options: ListOptions) -> Result<Page<Webhook>> {
        self.client
            .block_on(self.client.inner.webhooks().list(options))
    }

    /// Every webhook, fetching further pages as you iterate.
    pub fn list_all(&self, options: ListOptions) -> impl Iterator<Item = Result<Webhook>> + 'a {
        let client = self.client;
        client.iter(client.inner.webhooks().list_all(options))
    }

    /// See [`crate::Webhooks::update`].
    pub fn update(&self, id: &str, changes: UpdateWebhook) -> Result<ObjectId> {
        self.client
            .block_on(self.client.inner.webhooks().update(id, changes))
    }

    /// See [`crate::Webhooks::delete`].
    pub fn delete(&self, id: &str) -> Result<Deleted> {
        self.client
            .block_on(self.client.inner.webhooks().delete(id))
    }

    /// See [`crate::Webhooks::rotate_secret`].
    pub fn rotate_secret(&self, id: &str) -> Result<WebhookSecret> {
        self.client
            .block_on(self.client.inner.webhooks().rotate_secret(id))
    }
}

/// Blocking webhook deliveries.
#[derive(Debug, Clone, Copy)]
pub struct WebhookDeliveries<'a> {
    client: &'a AgdirEmail,
}

impl<'a> WebhookDeliveries<'a> {
    /// See [`crate::WebhookDeliveries::list`].
    pub fn list(&self, webhook_id: &str, options: ListOptions) -> Result<Page<WebhookDelivery>> {
        self.client.block_on(
            self.client
                .inner
                .webhooks()
                .deliveries()
                .list(webhook_id, options),
        )
    }

    /// Every delivery for a webhook, fetching further pages as you iterate.
    pub fn list_all(
        &self,
        webhook_id: &str,
        options: ListOptions,
    ) -> impl Iterator<Item = Result<WebhookDelivery>> + 'a {
        let client = self.client;
        client.iter(
            client
                .inner
                .webhooks()
                .deliveries()
                .list_all(webhook_id, options),
        )
    }

    /// See [`crate::WebhookDeliveries::get`].
    pub fn get(&self, webhook_id: &str, delivery_id: &str) -> Result<WebhookDelivery> {
        self.client.block_on(
            self.client
                .inner
                .webhooks()
                .deliveries()
                .get(webhook_id, delivery_id),
        )
    }

    /// See [`crate::WebhookDeliveries::attempts`].
    pub fn attempts(
        &self,
        webhook_id: &str,
        delivery_id: &str,
        options: ListOptions,
    ) -> Result<Page<WebhookAttempt>> {
        self.client
            .block_on(self.client.inner.webhooks().deliveries().attempts(
                webhook_id,
                delivery_id,
                options,
            ))
    }

    /// See [`crate::WebhookDeliveries::replay`].
    pub fn replay(&self, webhook_id: &str, delivery_id: &str) -> Result<ObjectId> {
        self.client.block_on(
            self.client
                .inner
                .webhooks()
                .deliveries()
                .replay(webhook_id, delivery_id),
        )
    }
}
