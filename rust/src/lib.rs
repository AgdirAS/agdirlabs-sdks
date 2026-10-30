#![doc = include_str!("../README.md")]
#![deny(missing_docs)]
#![forbid(unsafe_code)]

#[cfg(feature = "blocking")]
pub mod blocking;
mod client;
mod domains;
mod emails;
mod error;
mod pagination;
mod serde_util;
mod verify;
mod webhooks;

use serde::{Deserialize, Serialize};

pub use client::{AgdirEmail, ClientBuilder, DEFAULT_BASE_URL, USER_AGENT, VERSION};
pub use domains::{CreateDomain, DnsRecord, DnsRecordPurpose, Domain, DomainStatus, Domains};
pub use emails::{
    BatchError, BatchOptions, BatchResult, BatchValidation, Email, EmailStatus, Emails,
    IntoAddresses, SendEmail, SendOptions, SendResult, Tag, MAX_BATCH_SIZE,
};
pub use error::{Error, ErrorCode, Result, WebhookVerificationError};
pub use pagination::{ListOptions, Page};
pub use verify::{
    sign_webhook, verify_webhook, verify_webhook_with, Bounce, Click, Failure, VerifyOptions,
    WebhookEvent, WebhookEventData, WebhookHeaders, DEFAULT_TOLERANCE,
};
pub use webhooks::{
    CreateWebhook, UpdateWebhook, Webhook, WebhookAttempt, WebhookDeliveries, WebhookDelivery,
    WebhookDeliveryStatus, WebhookEventType, WebhookSecret, WebhookStatus, Webhooks,
};

/// The [`time`] crate, re-exported for [`SendEmail::scheduled_at`] and the timestamps on
/// returned objects (`time::OffsetDateTime`).
pub use time;

/// The [`reqwest`] crate, re-exported for [`ClientBuilder::http_client`].
pub use reqwest;

/// The id of the object an action applied to (cancel, verify, update, replay).
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ObjectId {
    /// Object id.
    pub id: String,
}

/// The result of a delete.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Deleted {
    /// Id of the deleted object.
    pub id: String,
    /// Always `true`.
    #[serde(default = "yes")]
    pub deleted: bool,
}

fn yes() -> bool {
    true
}

/// `use agdirlabs_email::prelude::*;` imports the client, request builders, errors and the
/// stream extension traits needed to consume `list_all`.
pub mod prelude {
    pub use crate::{
        AgdirEmail, CreateDomain, CreateWebhook, Error, ErrorCode, ListOptions, Result, SendEmail,
        UpdateWebhook, WebhookEventType, WebhookStatus,
    };
    pub use futures_util::{StreamExt as _, TryStreamExt as _};
}

/// Stream utilities for consuming `list_all`, re-exported from `futures-util` so you don't
/// need another dependency: `use agdirlabs_email::stream::TryStreamExt;`.
pub mod stream {
    pub use futures_core::Stream;
    pub use futures_util::{StreamExt, TryStreamExt};
}
