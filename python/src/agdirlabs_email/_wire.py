"""Mapping between the SDK's snake_case objects and the API's JSON."""

from __future__ import annotations

import re
from datetime import datetime, timezone
from typing import Any, Callable, Dict, List, Mapping, Optional, TypeVar, Union

from ._types import (
    Bounce,
    Click,
    DeleteResult,
    DnsRecord,
    Domain,
    DomainWithRecords,
    Email,
    Failure,
    IdResult,
    Page,
    SecretResult,
    SendBatchError,
    SendBatchResult,
    SendEmailParams,
    SendResult,
    Webhook,
    WebhookAttempt,
    WebhookDelivery,
    WebhookDeliveryDetail,
    WebhookEvent,
    WebhookEventData,
    WebhookWithSecret,
)

T = TypeVar("T")
Json = Dict[str, Any]

_ISO = re.compile(
    r"^(?P<date>\d{4}-\d{2}-\d{2})[T ](?P<time>\d{2}:\d{2}(?::\d{2})?)"
    r"(?:\.(?P<frac>\d+))?(?P<tz>Z|z|[+-]\d{2}(?::?\d{2})?)?$"
)


def parse_datetime(value: str) -> datetime:
    """Parses the API's ISO 8601 timestamps (``2026-10-30T09:44:49.077Z``) on every supported Python version."""
    m = _ISO.match(value.strip())
    if not m:
        return datetime.fromisoformat(value)
    frac = (m.group("frac") or "")[:6].ljust(6, "0")
    tz = m.group("tz")
    if tz is None or tz in ("Z", "z"):
        offset = "+00:00"
    elif len(tz) == 3:
        offset = tz + ":00"
    elif ":" not in tz:
        offset = f"{tz[:3]}:{tz[3:]}"
    else:
        offset = tz
    return datetime.fromisoformat(f"{m.group('date')}T{m.group('time')}.{frac}{offset}")


def _optional_datetime(value: Optional[str]) -> Optional[datetime]:
    return parse_datetime(value) if value else None


def _str_list(value: Union[str, List[str], None]) -> List[str]:
    if value is None:
        return []
    return [value] if isinstance(value, str) else list(value)


def format_datetime(value: Union[datetime, str]) -> str:
    if isinstance(value, str):
        return value
    if value.tzinfo is None or value.utcoffset() is None:
        raise ValueError("scheduled_at must be a timezone-aware datetime, e.g. datetime.now(timezone.utc)")
    return value.astimezone(timezone.utc).isoformat().replace("+00:00", "Z")


_SEND_KEYS = ("from", "to", "subject", "html", "text", "cc", "bcc", "reply_to", "headers", "tags")


def send_email_body(params: SendEmailParams) -> Json:
    source: Mapping[str, Any] = params
    body: Json = {key: source[key] for key in _SEND_KEYS if source.get(key) is not None}
    scheduled_at = source.get("scheduled_at")
    if scheduled_at is not None:
        body["scheduled_at"] = format_datetime(scheduled_at)
    unknown = set(source) - set(_SEND_KEYS) - {"scheduled_at"}
    if unknown:
        raise TypeError(f"Unknown email field(s): {', '.join(sorted(unknown))}")
    return body


def send_result(body: Json) -> SendResult:
    return SendResult(id=body["id"])


def send_batch_result(body: Json) -> SendBatchResult:
    return SendBatchResult(
        data=[SendResult(id=d["id"]) for d in body.get("data") or []],
        errors=[SendBatchError(index=e["index"], message=e["message"]) for e in body.get("errors") or []],
    )


def id_result(body: Json) -> IdResult:
    return IdResult(id=body["id"])


def delete_result(body: Json) -> DeleteResult:
    return DeleteResult(id=body["id"], deleted=True)


def secret_result(body: Json) -> SecretResult:
    return SecretResult(id=body["id"], signing_secret=body["signing_secret"])


def email(e: Json) -> Email:
    return Email(
        id=e["id"],
        from_=e["from"],
        to=_str_list(e.get("to")),
        cc=_str_list(e.get("cc")),
        bcc=_str_list(e.get("bcc")),
        reply_to=_str_list(e.get("reply_to")),
        subject=e["subject"],
        html=e.get("html"),
        text=e.get("text"),
        created_at=parse_datetime(e["created_at"]),
        scheduled_at=_optional_datetime(e.get("scheduled_at")),
        status=e["last_event"],
    )


def _record(r: Json) -> DnsRecord:
    return DnsRecord(
        purpose=r["record"],
        name=r["name"],
        type=r["type"],
        value=r["value"],
        ttl=r["ttl"],
        status=r["status"],
        priority=r.get("priority"),
    )


def _domain_fields(d: Json) -> Json:
    return {
        "id": d["id"],
        "name": d["name"],
        "status": d["status"],
        "region": d["region"],
        "created_at": parse_datetime(d["created_at"]),
        "open_tracking": bool(d.get("open_tracking")),
        "click_tracking": bool(d.get("click_tracking")),
    }


def domain(d: Json) -> Domain:
    return Domain(**_domain_fields(d))


def domain_with_records(d: Json) -> DomainWithRecords:
    return DomainWithRecords(**_domain_fields(d), records=[_record(r) for r in d.get("records") or []])


def _webhook_fields(w: Json) -> Json:
    return {
        "id": w["id"],
        "endpoint": w["endpoint"],
        "status": w["status"],
        "events": list(w.get("events") or []),
        "created_at": parse_datetime(w["created_at"]),
    }


def webhook(w: Json) -> Webhook:
    return Webhook(**_webhook_fields(w))


def webhook_with_secret(w: Json) -> WebhookWithSecret:
    return WebhookWithSecret(**_webhook_fields(w), signing_secret=w["signing_secret"])


def _delivery_fields(e: Json) -> Json:
    return {"id": e["id"], "type": e["type"], "status": e["status"], "created_at": parse_datetime(e["created_at"])}


def delivery(e: Json) -> WebhookDelivery:
    return WebhookDelivery(**_delivery_fields(e))


def delivery_detail(e: Json) -> WebhookDeliveryDetail:
    return WebhookDeliveryDetail(
        **_delivery_fields(e),
        next_attempt_at=_optional_datetime(e.get("next_attempt_at")),
        payload=e.get("payload"),
    )


def attempt(a: Json) -> WebhookAttempt:
    return WebhookAttempt(
        id=a["id"],
        http_status=int(a.get("http_status_code") or 0),
        response=a.get("response") or "",
        sent_at=parse_datetime(a["sent_at"]),
    )


def page(map_item: Callable[[Json], T]) -> Callable[[Json], Page[T]]:
    def parse(body: Json) -> Page[T]:
        items: List[Json] = body.get("data") or []
        has_more = bool(body.get("has_more"))
        next_cursor = str(items[-1]["id"]) if has_more and items else None
        return Page(data=[map_item(item) for item in items], has_more=has_more, next_cursor=next_cursor)

    return parse


def webhook_event(body: Json) -> WebhookEvent:
    d: Json = body.get("data") or {}
    bounce = d.get("bounce")
    click = d.get("click")
    failed = d.get("failed")
    return WebhookEvent(
        type=body["type"],
        created_at=parse_datetime(body["created_at"]),
        data=WebhookEventData(
            email_id=d["email_id"],
            message_id=d.get("message_id") or None,
            from_=d["from"],
            to=_str_list(d.get("to")),
            subject=d["subject"],
            tags=dict(d.get("tags") or {}),
            created_at=parse_datetime(d["created_at"]),
            bounce=Bounce(type=bounce["type"], sub_type=bounce["subType"], message=bounce["message"])
            if bounce
            else None,
            click=Click(
                link=click["link"],
                ip_address=click["ipAddress"],
                user_agent=click["userAgent"],
                timestamp=parse_datetime(click["timestamp"]),
            )
            if click
            else None,
            failed=Failure(reason=failed["reason"]) if failed else None,
        ),
    )
