"""Public input (TypedDict) and output (dataclass) types."""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from typing import Any, Dict, Generic, List, Literal, Optional, TypedDict, TypeVar, Union

__all__ = [
    "WEBHOOK_EVENT_TYPES",
    "Bounce",
    "Click",
    "CreateDomainParams",
    "CreateWebhookParams",
    "DeleteResult",
    "DnsRecord",
    "Domain",
    "DomainStatus",
    "DomainWithRecords",
    "Email",
    "EmailStatus",
    "Failure",
    "IdResult",
    "Page",
    "SecretResult",
    "SendBatchError",
    "SendBatchResult",
    "SendEmailParams",
    "SendResult",
    "Tag",
    "UpdateWebhookParams",
    "Webhook",
    "WebhookAttempt",
    "WebhookDelivery",
    "WebhookDeliveryDetail",
    "WebhookDeliveryStatus",
    "WebhookEvent",
    "WebhookEventData",
    "WebhookEventType",
    "WebhookStatus",
    "WebhookWithSecret",
]

T = TypeVar("T")

EmailStatus = Literal[
    "queued",
    "scheduled",
    "sent",
    "delivery_delayed",
    "delivered",
    "opened",
    "clicked",
    "bounced",
    "failed",
    "complained",
    "canceled",
]
DomainStatus = Literal["not_started", "pending", "verified", "failed", "temporary_failure"]
WebhookEventType = Literal[
    "email.sent",
    "email.scheduled",
    "email.delivered",
    "email.delivery_delayed",
    "email.complained",
    "email.bounced",
    "email.opened",
    "email.clicked",
    "email.failed",
]
WebhookStatus = Literal["enabled", "disabled"]
WebhookDeliveryStatus = Literal["pending", "attempting", "success", "failed"]

WEBHOOK_EVENT_TYPES: tuple[WebhookEventType, ...] = (
    "email.sent",
    "email.scheduled",
    "email.delivered",
    "email.delivery_delayed",
    "email.complained",
    "email.bounced",
    "email.opened",
    "email.clicked",
    "email.failed",
)

# ---------------------------------------------------------------------------------------------------------------------
# Inputs


class Tag(TypedDict):
    """ASCII letters, numbers, ``_`` and ``-`` only. Echoed back in webhook events."""

    name: str
    value: str


# `from` is a keyword, so the required keys use the functional syntax.
_SendEmailRequired = TypedDict(
    "_SendEmailRequired",
    {"from": str, "to": Union[str, List[str]], "subject": str},
)


class SendEmailParams(_SendEmailRequired, total=False):
    """One email. ``from`` is ``"Name <addr@example.com>"`` or a bare address; one of ``html``/``text`` is required."""

    html: str
    text: str
    cc: Union[str, List[str]]
    bcc: Union[str, List[str]]
    reply_to: Union[str, List[str]]
    headers: Dict[str, str]
    tags: List[Tag]
    scheduled_at: Union[datetime, str]
    """Deliver later: a timezone-aware ``datetime`` or an ISO 8601 string with offset."""


class _CreateDomainRequired(TypedDict):
    name: str


class CreateDomainParams(_CreateDomainRequired, total=False):
    region: str


class CreateWebhookParams(TypedDict):
    endpoint: str
    """Public ``https://`` URL."""
    events: List[WebhookEventType]


class UpdateWebhookParams(TypedDict, total=False):
    endpoint: str
    events: List[WebhookEventType]
    status: WebhookStatus


# ---------------------------------------------------------------------------------------------------------------------
# Outputs


@dataclass(frozen=True)
class SendResult:
    id: str


@dataclass(frozen=True)
class SendBatchError:
    index: int
    """Index of the rejected email in the input list."""
    message: str


@dataclass(frozen=True)
class SendBatchResult:
    data: List[SendResult]
    errors: List[SendBatchError] = field(default_factory=list)
    """Only with ``validation="permissive"``: emails that were not accepted."""


@dataclass(frozen=True)
class IdResult:
    id: str


@dataclass(frozen=True)
class DeleteResult:
    id: str
    deleted: bool = True


@dataclass(frozen=True)
class SecretResult:
    id: str
    signing_secret: str
    """``whsec_…`` — pass it to :func:`verify_webhook`."""


@dataclass(frozen=True)
class Email:
    id: str
    from_: str
    to: List[str]
    cc: List[str]
    bcc: List[str]
    reply_to: List[str]
    subject: str
    html: Optional[str]
    text: Optional[str]
    created_at: datetime
    scheduled_at: Optional[datetime]
    status: EmailStatus
    """Most advanced delivery state reached so far."""


@dataclass(frozen=True)
class DnsRecord:
    purpose: Literal["DKIM", "SPF", "DMARC"]
    """What the record is for."""
    name: str
    """Relative to the registrable domain, e.g. ``agdir._domainkey`` or ``send``."""
    type: Literal["TXT", "MX", "CNAME"]
    value: str
    ttl: str
    status: DomainStatus
    priority: Optional[int] = None


@dataclass(frozen=True)
class Domain:
    id: str
    name: str
    status: DomainStatus
    region: str
    created_at: datetime
    open_tracking: bool
    click_tracking: bool


@dataclass(frozen=True)
class DomainWithRecords(Domain):
    records: List[DnsRecord] = field(default_factory=list)


@dataclass(frozen=True)
class Webhook:
    id: str
    endpoint: str
    status: WebhookStatus
    events: List[WebhookEventType]
    created_at: datetime


@dataclass(frozen=True)
class WebhookWithSecret(Webhook):
    signing_secret: str = ""
    """``whsec_…`` — pass it to :func:`verify_webhook`."""


@dataclass(frozen=True)
class WebhookDelivery:
    id: str
    type: WebhookEventType
    status: WebhookDeliveryStatus
    created_at: datetime


@dataclass(frozen=True)
class WebhookDeliveryDetail(WebhookDelivery):
    next_attempt_at: Optional[datetime] = None
    payload: Any = None
    """Exact JSON body sent to your endpoint."""


@dataclass(frozen=True)
class WebhookAttempt:
    id: str
    http_status: int
    """0 when no HTTP response was received."""
    response: str
    sent_at: datetime


@dataclass(frozen=True)
class Page(Generic[T]):
    data: List[T]
    has_more: bool
    next_cursor: Optional[str]
    """Pass as ``after`` to fetch the next page; ``None`` on the last page."""


@dataclass(frozen=True)
class Bounce:
    type: str
    sub_type: str
    message: str


@dataclass(frozen=True)
class Click:
    link: str
    ip_address: str
    user_agent: str
    timestamp: datetime


@dataclass(frozen=True)
class Failure:
    reason: str


@dataclass(frozen=True)
class WebhookEventData:
    email_id: str
    from_: str
    to: List[str]
    subject: str
    tags: Dict[str, str]
    created_at: datetime
    message_id: Optional[str] = None
    bounce: Optional[Bounce] = None
    click: Optional[Click] = None
    failed: Optional[Failure] = None


@dataclass(frozen=True)
class WebhookEvent:
    """An event received on your webhook endpoint, as returned by :func:`verify_webhook`."""

    type: WebhookEventType
    created_at: datetime
    data: WebhookEventData
