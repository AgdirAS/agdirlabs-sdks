//! Sending domains and their DNS records.

use futures_core::Stream;
use reqwest::Method;
use serde::{Deserialize, Serialize};
use time::OffsetDateTime;

use crate::client::{AgdirEmail, Request};
use crate::error::Result;
use crate::pagination::{paginate, Identified, ListOptions, Page};
use crate::serde_util::{null_to_empty, null_to_false};
use crate::{Deleted, ObjectId};

/// Verification state of a domain or one of its DNS records.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
#[non_exhaustive]
pub enum DomainStatus {
    /// DNS has not been checked yet.
    NotStarted,
    /// Waiting for the DNS records to appear.
    Pending,
    /// Ready to send.
    Verified,
    /// The records were not found in time; fix DNS and verify again.
    Failed,
    /// A temporary DNS problem; checks continue.
    TemporaryFailure,
    /// A status this version of the SDK does not know about yet.
    #[serde(other)]
    Unknown,
}

/// What a DNS record is for.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[non_exhaustive]
pub enum DnsRecordPurpose {
    /// Signs your mail.
    #[serde(rename = "DKIM")]
    Dkim,
    /// Authorizes the bounce (MAIL FROM) domain.
    #[serde(rename = "SPF")]
    Spf,
    /// Tells receivers how to handle unauthenticated mail.
    #[serde(rename = "DMARC")]
    Dmarc,
    /// A purpose this version of the SDK does not know about yet.
    #[serde(other)]
    Unknown,
}

/// A DNS record to publish for a domain.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct DnsRecord {
    /// What the record is for.
    #[serde(rename = "record")]
    pub purpose: DnsRecordPurpose,
    /// Host name relative to the registrable domain, e.g. `agdir._domainkey` or `send`.
    pub name: String,
    /// `TXT`, `MX` or `CNAME`.
    #[serde(rename = "type")]
    pub record_type: String,
    /// Record value.
    pub value: String,
    /// TTL, usually `"Auto"`.
    pub ttl: String,
    /// MX priority.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub priority: Option<u32>,
    /// Whether this record was found.
    pub status: DomainStatus,
}

/// A sending domain.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Domain {
    /// Domain id.
    pub id: String,
    /// Domain name, e.g. `acme.com`.
    pub name: String,
    /// Verification state.
    pub status: DomainStatus,
    /// Sending region, e.g. `eu-north-1`.
    pub region: String,
    /// When the domain was added.
    #[serde(with = "time::serde::rfc3339")]
    pub created_at: OffsetDateTime,
    /// Whether open tracking is on.
    #[serde(default, deserialize_with = "null_to_false")]
    pub open_tracking: bool,
    /// Whether click tracking is on.
    #[serde(default, deserialize_with = "null_to_false")]
    pub click_tracking: bool,
    /// DNS records to publish. Filled by `create` and `get`; empty in list results.
    #[serde(default, deserialize_with = "null_to_empty")]
    pub records: Vec<DnsRecord>,
}

impl Identified for Domain {
    fn id(&self) -> &str {
        &self.id
    }
}

/// A domain to add, for [`Domains::create`].
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize)]
pub struct CreateDomain {
    /// Domain name, e.g. `acme.com` or `mail.acme.com`.
    pub name: String,
    /// Sending region. Defaults to `eu-north-1`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub region: Option<String>,
}

impl CreateDomain {
    /// A domain named `name` in the default region.
    pub fn new(name: impl Into<String>) -> Self {
        CreateDomain {
            name: name.into(),
            region: None,
        }
    }

    /// Sets the sending region.
    pub fn region(mut self, region: impl Into<String>) -> Self {
        self.region = Some(region.into());
        self
    }
}

impl From<&str> for CreateDomain {
    fn from(name: &str) -> Self {
        Self::new(name)
    }
}

impl From<String> for CreateDomain {
    fn from(name: String) -> Self {
        Self::new(name)
    }
}

/// The `domains` resource. Get it with [`AgdirEmail::domains`].
#[derive(Debug, Clone, Copy)]
pub struct Domains<'a> {
    pub(crate) client: &'a AgdirEmail,
}

impl Domains<'_> {
    /// Adds a sending domain: `create("acme.com")` or `create(CreateDomain::new(..).region(..))`.
    /// Publish the returned `records` in DNS, then call [`verify`](Self::verify).
    pub async fn create(&self, domain: impl Into<CreateDomain>) -> Result<Domain> {
        let req = Request::new(Method::POST, &["domains"], false).json(&domain.into());
        self.client.request(req).await
    }

    /// Reads a domain with its DNS records.
    pub async fn get(&self, id: &str) -> Result<Domain> {
        self.client.request(Request::get(&["domains", id])).await
    }

    /// One page of domains.
    pub async fn list(&self, options: ListOptions) -> Result<Page<Domain>> {
        let req = Request::get(&["domains"]).list_query(options.limit, options.after.as_deref());
        self.client.request(req).await
    }

    /// Every domain, fetching further pages as you consume the stream.
    pub fn list_all(
        &self,
        options: ListOptions,
    ) -> impl Stream<Item = Result<Domain>> + Send + Unpin + 'static {
        let client = self.client.clone();
        paginate(options, move |options| {
            let client = client.clone();
            async move { client.domains().list(options).await }
        })
    }

    /// Starts a DNS check now instead of waiting for the next automatic one.
    pub async fn verify(&self, id: &str) -> Result<ObjectId> {
        let path = ["domains", id, "verify"];
        self.client
            .request(Request::new(Method::POST, &path, true))
            .await
    }

    /// Removes a domain. Emails can no longer be sent from it.
    pub async fn delete(&self, id: &str) -> Result<Deleted> {
        let path = ["domains", id];
        self.client
            .request(Request::new(Method::DELETE, &path, false))
            .await
    }
}
