//! Webhook endpoints, their deliveries and delivery attempts.

use futures_core::Stream;
use reqwest::Method;
use serde::{Deserialize, Serialize};
use time::OffsetDateTime;

use crate::client::{AgdirEmail, Request};
use crate::error::Result;
use crate::pagination::{paginate, Identified, ListOptions, Page};
use crate::{Deleted, ObjectId};

/// Events a webhook can subscribe to.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[non_exhaustive]
pub enum WebhookEventType {
    /// `email.sent`: handed to the receiving mail server.
    #[serde(rename = "email.sent")]
    Sent,
    /// `email.scheduled`: accepted for later delivery.
    #[serde(rename = "email.scheduled")]
    Scheduled,
    /// `email.delivered`: accepted by the receiving server.
    #[serde(rename = "email.delivered")]
    Delivered,
    /// `email.delivery_delayed`: the receiving server is deferring it.
    #[serde(rename = "email.delivery_delayed")]
    DeliveryDelayed,
    /// `email.complained`: marked as spam.
    #[serde(rename = "email.complained")]
    Complained,
    /// `email.bounced`: bounced.
    #[serde(rename = "email.bounced")]
    Bounced,
    /// `email.opened`: opened.
    #[serde(rename = "email.opened")]
    Opened,
    /// `email.clicked`: a link was clicked.
    #[serde(rename = "email.clicked")]
    Clicked,
    /// `email.failed`: could not be sent.
    #[serde(rename = "email.failed")]
    Failed,
    /// An event type this version of the SDK does not know about yet.
    #[serde(other)]
    Unknown,
}

impl WebhookEventType {
    /// Every known event type.
    pub const ALL: [WebhookEventType; 9] = [
        Self::Sent,
        Self::Scheduled,
        Self::Delivered,
        Self::DeliveryDelayed,
        Self::Complained,
        Self::Bounced,
        Self::Opened,
        Self::Clicked,
        Self::Failed,
    ];

    /// The wire name, e.g. `"email.delivered"`.
    pub fn as_str(&self) -> &'static str {
        match self {
            Self::Sent => "email.sent",
            Self::Scheduled => "email.scheduled",
            Self::Delivered => "email.delivered",
            Self::DeliveryDelayed => "email.delivery_delayed",
            Self::Complained => "email.complained",
            Self::Bounced => "email.bounced",
            Self::Opened => "email.opened",
            Self::Clicked => "email.clicked",
            Self::Failed => "email.failed",
            Self::Unknown => "unknown",
        }
    }
}

/// Whether a webhook receives events.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
#[non_exhaustive]
pub enum WebhookStatus {
    /// Receives events.
    Enabled,
    /// Paused, by you or after failing for 3 days.
    Disabled,
    /// A status this version of the SDK does not know about yet.
    #[serde(other)]
    Unknown,
}

/// A webhook endpoint.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Webhook {
    /// Webhook id.
    pub id: String,
    /// The `https://` URL events are posted to.
    pub endpoint: String,
    /// Enabled or disabled.
    pub status: WebhookStatus,
    /// Subscribed event types.
    pub events: Vec<WebhookEventType>,
    /// When the webhook was created.
    #[serde(with = "time::serde::rfc3339")]
    pub created_at: OffsetDateTime,
    /// `whsec_…`, for [`verify_webhook`](crate::verify_webhook). Filled by `get`; `None` in list results.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub signing_secret: Option<String>,
}

impl Identified for Webhook {
    fn id(&self) -> &str {
        &self.id
    }
}

/// A webhook id with its signing secret, from [`Webhooks::create`] and [`Webhooks::rotate_secret`].
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct WebhookSecret {
    /// Webhook id.
    pub id: String,
    /// `whsec_…` — keep it to verify incoming events.
    pub signing_secret: String,
}

/// A webhook to register, for [`Webhooks::create`].
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct CreateWebhook {
    /// Public `https://` URL.
    pub endpoint: String,
    /// Event types to receive.
    pub events: Vec<WebhookEventType>,
}

impl CreateWebhook {
    /// A webhook posting `events` to `endpoint`.
    pub fn new(
        endpoint: impl Into<String>,
        events: impl IntoIterator<Item = WebhookEventType>,
    ) -> Self {
        CreateWebhook {
            endpoint: endpoint.into(),
            events: events.into_iter().collect(),
        }
    }
}

/// Changes for [`Webhooks::update`]; unset fields are left as they are.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize)]
pub struct UpdateWebhook {
    /// New endpoint URL.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub endpoint: Option<String>,
    /// New event types.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub events: Option<Vec<WebhookEventType>>,
    /// Enable or disable.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub status: Option<WebhookStatus>,
}

impl UpdateWebhook {
    /// No changes yet.
    pub fn new() -> Self {
        Self::default()
    }

    /// Changes the endpoint URL.
    pub fn endpoint(mut self, endpoint: impl Into<String>) -> Self {
        self.endpoint = Some(endpoint.into());
        self
    }

    /// Replaces the subscribed event types.
    pub fn events(mut self, events: impl IntoIterator<Item = WebhookEventType>) -> Self {
        self.events = Some(events.into_iter().collect());
        self
    }

    /// Enables or disables the webhook.
    pub fn status(mut self, status: WebhookStatus) -> Self {
        self.status = Some(status);
        self
    }
}

/// State of one delivery to your endpoint.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
#[non_exhaustive]
pub enum WebhookDeliveryStatus {
    /// Waiting for its next attempt.
    Pending,
    /// Being sent right now.
    Attempting,
    /// Your endpoint answered 2xx.
    Success,
    /// All retries failed.
    Failed,
    /// A status this version of the SDK does not know about yet.
    #[serde(other)]
    Unknown,
}

/// One message sent (or queued) to a webhook endpoint.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct WebhookDelivery {
    /// Delivery id (also the `webhook-id` header your endpoint receives).
    pub id: String,
    /// Event type.
    #[serde(rename = "type")]
    pub event_type: WebhookEventType,
    /// Delivery state.
    pub status: WebhookDeliveryStatus,
    /// When the delivery was created.
    #[serde(with = "time::serde::rfc3339")]
    pub created_at: OffsetDateTime,
    /// When the next retry is due. Only filled by `get`.
    #[serde(
        default,
        with = "time::serde::rfc3339::option",
        skip_serializing_if = "Option::is_none"
    )]
    pub next_attempt_at: Option<OffsetDateTime>,
    /// The exact JSON body sent to your endpoint. Only filled by `get`.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub payload: Option<serde_json::Value>,
}

impl Identified for WebhookDelivery {
    fn id(&self) -> &str {
        &self.id
    }
}

/// One HTTP attempt of a delivery.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct WebhookAttempt {
    /// Attempt id.
    pub id: String,
    /// Your endpoint's HTTP status; 0 when no response was received.
    #[serde(rename = "http_status_code", default)]
    pub http_status: u16,
    /// Your endpoint's response body (truncated), or the connection error.
    #[serde(default)]
    pub response: String,
    /// When the attempt was made.
    #[serde(with = "time::serde::rfc3339")]
    pub sent_at: OffsetDateTime,
}

impl Identified for WebhookAttempt {
    fn id(&self) -> &str {
        &self.id
    }
}

/// The `webhooks` resource. Get it with [`AgdirEmail::webhooks`].
#[derive(Debug, Clone, Copy)]
pub struct Webhooks<'a> {
    pub(crate) client: &'a AgdirEmail,
}

impl<'a> Webhooks<'a> {
    /// Messages sent to your endpoints, with their HTTP attempts.
    pub fn deliveries(&self) -> WebhookDeliveries<'a> {
        WebhookDeliveries {
            client: self.client,
        }
    }

    /// Registers an endpoint. Keep the returned `signing_secret` to verify incoming events.
    pub async fn create(&self, webhook: CreateWebhook) -> Result<WebhookSecret> {
        let req = Request::new(Method::POST, &["webhooks"], false).json(&webhook);
        self.client.request(req).await
    }

    /// Reads a webhook, including its signing secret.
    pub async fn get(&self, id: &str) -> Result<Webhook> {
        self.client.request(Request::get(&["webhooks", id])).await
    }

    /// One page of webhooks.
    pub async fn list(&self, options: ListOptions) -> Result<Page<Webhook>> {
        let req = Request::get(&["webhooks"]).list_query(options.limit, options.after.as_deref());
        self.client.request(req).await
    }

    /// Every webhook, fetching further pages as you consume the stream.
    pub fn list_all(
        &self,
        options: ListOptions,
    ) -> impl Stream<Item = Result<Webhook>> + Send + Unpin + 'static {
        let client = self.client.clone();
        paginate(options, move |options| {
            let client = client.clone();
            async move { client.webhooks().list(options).await }
        })
    }

    /// Changes the endpoint, events or status.
    pub async fn update(&self, id: &str, changes: UpdateWebhook) -> Result<ObjectId> {
        let path = ["webhooks", id];
        let req = Request::new(Method::PATCH, &path, true).json(&changes);
        self.client.request(req).await
    }

    /// Removes a webhook.
    pub async fn delete(&self, id: &str) -> Result<Deleted> {
        let path = ["webhooks", id];
        self.client
            .request(Request::new(Method::DELETE, &path, false))
            .await
    }

    /// Issues a new signing secret; the old one stops working immediately.
    pub async fn rotate_secret(&self, id: &str) -> Result<WebhookSecret> {
        let path = ["webhooks", id, "signing-secret", "rotate"];
        self.client
            .request(Request::new(Method::POST, &path, false))
            .await
    }
}

/// Deliveries ("webhook events") of an endpoint. Get it with [`Webhooks::deliveries`].
#[derive(Debug, Clone, Copy)]
pub struct WebhookDeliveries<'a> {
    pub(crate) client: &'a AgdirEmail,
}

impl WebhookDeliveries<'_> {
    /// One page of deliveries for a webhook, newest first.
    pub async fn list(
        &self,
        webhook_id: &str,
        options: ListOptions,
    ) -> Result<Page<WebhookDelivery>> {
        let path = ["webhooks", webhook_id, "events"];
        let req = Request::get(&path).list_query(options.limit, options.after.as_deref());
        self.client.request(req).await
    }

    /// Every delivery for a webhook, fetching further pages as you consume the stream.
    pub fn list_all(
        &self,
        webhook_id: &str,
        options: ListOptions,
    ) -> impl Stream<Item = Result<WebhookDelivery>> + Send + Unpin + 'static {
        let client = self.client.clone();
        let webhook_id = webhook_id.to_owned();
        paginate(options, move |options| {
            let client = client.clone();
            let webhook_id = webhook_id.clone();
            async move {
                client
                    .webhooks()
                    .deliveries()
                    .list(&webhook_id, options)
                    .await
            }
        })
    }

    /// One delivery, with its payload and next retry time.
    pub async fn get(&self, webhook_id: &str, delivery_id: &str) -> Result<WebhookDelivery> {
        let path = ["webhooks", webhook_id, "events", delivery_id];
        self.client.request(Request::get(&path)).await
    }

    /// HTTP attempts for one delivery, newest first.
    pub async fn attempts(
        &self,
        webhook_id: &str,
        delivery_id: &str,
        options: ListOptions,
    ) -> Result<Page<WebhookAttempt>> {
        let path = ["webhooks", webhook_id, "events", delivery_id, "attempts"];
        let req = Request::get(&path).list_query(options.limit, options.after.as_deref());
        self.client.request(req).await
    }

    /// Sends the identical message again, once, right now (also to disabled endpoints).
    pub async fn replay(&self, webhook_id: &str, delivery_id: &str) -> Result<ObjectId> {
        let path = ["webhooks", webhook_id, "events", delivery_id, "replay"];
        self.client
            .request(Request::new(Method::POST, &path, false))
            .await
    }
}
