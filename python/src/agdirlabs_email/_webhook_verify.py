from __future__ import annotations

import base64
import binascii
import hashlib
import hmac
import json
import time
from datetime import datetime
from typing import Any, Mapping, Optional, Sequence, Union

from ._errors import WebhookVerificationError
from ._types import WebhookEvent
from ._wire import webhook_event

__all__ = ["verify_webhook"]

HeaderValue = Union[str, bytes, Sequence[str], None]


def _header(headers: Mapping[str, Any], name: str) -> Optional[str]:
    value: HeaderValue = headers.get(name)
    if value is None:
        # Plain dicts may use any capitalisation (`Webhook-Id`); framework header objects are case-insensitive.
        for key, candidate in headers.items():
            if isinstance(key, str) and key.lower() == name:
                value = candidate
                break
    if isinstance(value, bytes):
        return value.decode("latin-1")
    if isinstance(value, str):
        return value
    if value:
        return str(value[0])
    return None


def verify_webhook(
    payload: Union[str, bytes],
    headers: Mapping[str, Any],
    secret: str,
    *,
    tolerance_seconds: float = 300,
    now: Optional[datetime] = None,
) -> WebhookEvent:
    """Checks that a webhook request came from Agdir Email and returns the parsed event.

    Messages are signed per the Standard Webhooks spec (``webhook-id``, ``webhook-timestamp``,
    ``webhook-signature``). Raises :class:`WebhookVerificationError` if anything doesn't match.

    :param payload: The raw request body, exactly as received (``request.body`` in Django, ``request.get_data()`` in
        Flask, ``await request.body()`` in FastAPI) — not re-serialized JSON.
    :param headers: Request headers as any mapping, e.g. ``request.headers`` or a plain ``dict`` (in Django use
        ``request.headers``, not ``request.META``).
    :param secret: The webhook's ``whsec_…`` signing secret.
    :param tolerance_seconds: Reject messages older or newer than this. Default 300.
    :param now: For tests.
    """
    msg_id = _header(headers, "webhook-id")
    timestamp = _header(headers, "webhook-timestamp")
    signatures = _header(headers, "webhook-signature")
    if not msg_id or not timestamp or not signatures:
        raise WebhookVerificationError("Missing webhook signature headers")

    try:
        sent_at = int(timestamp)
    except ValueError:
        raise WebhookVerificationError("Webhook timestamp is outside the allowed tolerance") from None
    current = now.timestamp() if now is not None else time.time()
    if abs(current - sent_at) > tolerance_seconds:
        raise WebhookVerificationError("Webhook timestamp is outside the allowed tolerance")

    encoded = secret[len("whsec_") :] if secret.startswith("whsec_") else secret
    try:
        key = base64.b64decode(encoded, validate=True)
    except (binascii.Error, ValueError):
        raise WebhookVerificationError("Signing secret is not valid base64") from None

    body = payload if isinstance(payload, bytes) else payload.encode("utf-8")
    signed = msg_id.encode("utf-8") + b"." + timestamp.encode("utf-8") + b"." + body
    expected = base64.b64encode(hmac.new(key, signed, hashlib.sha256).digest()).decode("ascii")

    # The header may hold several space-separated `v1,<sig>` entries (e.g. during secret rotation).
    valid = False
    for entry in signatures.split():
        version, _, signature = entry.partition(",")
        if version == "v1" and hmac.compare_digest(signature.encode("ascii", "replace"), expected.encode("ascii")):
            valid = True
    if not valid:
        raise WebhookVerificationError("Webhook signature does not match")

    try:
        return webhook_event(json.loads(body))
    except (ValueError, KeyError, TypeError, AttributeError):
        raise WebhookVerificationError("Webhook payload is not a valid Agdir Email event") from None
