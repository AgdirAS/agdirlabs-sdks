"""Every API call as an :class:`Operation`, shared by the sync and async resources."""

from __future__ import annotations

import uuid
from typing import List, Literal, Optional

from . import _wire as wire
from ._http import Operation, seg
from ._types import (
    CreateDomainParams,
    CreateWebhookParams,
    DeleteResult,
    Domain,
    DomainWithRecords,
    Email,
    IdResult,
    Page,
    SecretResult,
    SendBatchResult,
    SendEmailParams,
    SendResult,
    UpdateWebhookParams,
    Webhook,
    WebhookAttempt,
    WebhookDelivery,
    WebhookDeliveryDetail,
    WebhookWithSecret,
)

MAX_BATCH_SIZE = 100
BatchValidation = Literal["strict", "permissive"]


def idempotency_key(given: Optional[str], max_retries: int) -> Optional[str]:
    if given:
        return given
    # Without a key a retried send could be delivered twice, so only generate one when we may retry.
    return str(uuid.uuid4()) if max_retries > 0 else None


# Emails


def send_email(params: SendEmailParams, key: Optional[str]) -> Operation[SendResult]:
    return Operation(
        "POST",
        "/emails",
        wire.send_result,
        retryable=key is not None,
        body=wire.send_email_body(params),
        headers={"Idempotency-Key": key},
    )


def send_batch(
    emails: List[SendEmailParams], key: Optional[str], validation: BatchValidation
) -> Operation[SendBatchResult]:
    if validation not in ("strict", "permissive"):
        raise ValueError("validation must be 'strict' or 'permissive'")
    return Operation(
        "POST",
        "/emails/batch",
        wire.send_batch_result,
        retryable=key is not None,
        body=[wire.send_email_body(e) for e in emails],
        headers={"Idempotency-Key": key, "x-batch-validation": "permissive" if validation == "permissive" else None},
    )


def get_email(email_id: str) -> Operation[Email]:
    return Operation("GET", f"/emails/{seg(email_id)}", wire.email, retryable=True)


def cancel_email(email_id: str) -> Operation[IdResult]:
    return Operation("POST", f"/emails/{seg(email_id)}/cancel", wire.id_result, retryable=True)


# Domains


def create_domain(params: CreateDomainParams) -> Operation[DomainWithRecords]:
    body = {"name": params["name"]}
    if params.get("region"):
        body["region"] = params["region"]
    return Operation("POST", "/domains", wire.domain_with_records, retryable=False, body=body)


def get_domain(domain_id: str) -> Operation[DomainWithRecords]:
    return Operation("GET", f"/domains/{seg(domain_id)}", wire.domain_with_records, retryable=True)


def list_domains(limit: Optional[int], after: Optional[str]) -> Operation[Page[Domain]]:
    return Operation("GET", "/domains", wire.page(wire.domain), retryable=True, query={"limit": limit, "after": after})


def verify_domain(domain_id: str) -> Operation[IdResult]:
    return Operation("POST", f"/domains/{seg(domain_id)}/verify", wire.id_result, retryable=True)


def delete_domain(domain_id: str) -> Operation[DeleteResult]:
    return Operation("DELETE", f"/domains/{seg(domain_id)}", wire.delete_result, retryable=False)


# Webhooks


def create_webhook(params: CreateWebhookParams) -> Operation[SecretResult]:
    body = {"endpoint": params["endpoint"], "events": list(params["events"])}
    return Operation("POST", "/webhooks", wire.secret_result, retryable=False, body=body)


def get_webhook(webhook_id: str) -> Operation[WebhookWithSecret]:
    return Operation("GET", f"/webhooks/{seg(webhook_id)}", wire.webhook_with_secret, retryable=True)


def list_webhooks(limit: Optional[int], after: Optional[str]) -> Operation[Page[Webhook]]:
    return Operation(
        "GET", "/webhooks", wire.page(wire.webhook), retryable=True, query={"limit": limit, "after": after}
    )


def update_webhook(webhook_id: str, params: UpdateWebhookParams) -> Operation[IdResult]:
    body = {k: v for k, v in params.items() if v is not None}
    unknown = set(body) - {"endpoint", "events", "status"}
    if unknown:
        raise TypeError(f"Unknown webhook field(s): {', '.join(sorted(unknown))}")
    return Operation("PATCH", f"/webhooks/{seg(webhook_id)}", wire.id_result, retryable=True, body=body)


def delete_webhook(webhook_id: str) -> Operation[DeleteResult]:
    return Operation("DELETE", f"/webhooks/{seg(webhook_id)}", wire.delete_result, retryable=False)


def rotate_secret(webhook_id: str) -> Operation[SecretResult]:
    return Operation("POST", f"/webhooks/{seg(webhook_id)}/signing-secret/rotate", wire.secret_result, retryable=False)


# Webhook deliveries ("webhook events" on the wire)


def list_deliveries(webhook_id: str, limit: Optional[int], after: Optional[str]) -> Operation[Page[WebhookDelivery]]:
    return Operation(
        "GET",
        f"/webhooks/{seg(webhook_id)}/events",
        wire.page(wire.delivery),
        retryable=True,
        query={"limit": limit, "after": after},
    )


def get_delivery(webhook_id: str, delivery_id: str) -> Operation[WebhookDeliveryDetail]:
    return Operation(
        "GET", f"/webhooks/{seg(webhook_id)}/events/{seg(delivery_id)}", wire.delivery_detail, retryable=True
    )


def list_attempts(
    webhook_id: str, delivery_id: str, limit: Optional[int], after: Optional[str]
) -> Operation[Page[WebhookAttempt]]:
    return Operation(
        "GET",
        f"/webhooks/{seg(webhook_id)}/events/{seg(delivery_id)}/attempts",
        wire.page(wire.attempt),
        retryable=True,
        query={"limit": limit, "after": after},
    )


def replay_delivery(webhook_id: str, delivery_id: str) -> Operation[IdResult]:
    return Operation(
        "POST", f"/webhooks/{seg(webhook_id)}/events/{seg(delivery_id)}/replay", wire.id_result, retryable=False
    )
