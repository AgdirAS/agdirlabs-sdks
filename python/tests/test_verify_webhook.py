from __future__ import annotations

import base64
import hashlib
import hmac
import json
import os
from datetime import datetime, timezone
from typing import Dict

import pytest

from agdirlabs_email import Bounce, WebhookVerificationError, verify_webhook

KEY = os.urandom(24)
SECRET = "whsec_" + base64.b64encode(KEY).decode()
NOW = datetime(2026, 10, 30, 12, 0, tzinfo=timezone.utc)
TIMESTAMP = str(int(NOW.timestamp()))

PAYLOAD = json.dumps(
    {
        "type": "email.bounced",
        "created_at": "2026-10-30T11:59:59.000Z",
        "data": {
            "created_at": "2026-10-30T11:59:00.000Z",
            "email_id": "em_1",
            "from": "Acme <hello@acme.com>",
            "to": ["user@example.com"],
            "subject": "Hi",
            "tags": {"kind": "welcome"},
            "bounce": {"message": "No such user", "subType": "General", "type": "Permanent"},
        },
    }
)


def sign(payload: str, msg_id: str = "msg_1", timestamp: str = TIMESTAMP, key: bytes = KEY) -> str:
    mac = hmac.new(key, f"{msg_id}.{timestamp}.{payload}".encode(), hashlib.sha256).digest()
    return "v1," + base64.b64encode(mac).decode()


def headers(payload: str = PAYLOAD, **overrides: str) -> Dict[str, str]:
    h = {"webhook-id": "msg_1", "webhook-timestamp": TIMESTAMP, "webhook-signature": sign(payload)}
    h.update(overrides)
    return h


def test_accepts_a_valid_signature_and_returns_the_event() -> None:
    event = verify_webhook(PAYLOAD, headers(), SECRET, now=NOW)
    assert event.type == "email.bounced"
    assert event.created_at == datetime(2026, 10, 30, 11, 59, 59, tzinfo=timezone.utc)
    assert event.data.email_id == "em_1"
    assert event.data.from_ == "Acme <hello@acme.com>"
    assert event.data.to == ["user@example.com"]
    assert event.data.tags == {"kind": "welcome"}
    assert event.data.message_id is None and event.data.click is None and event.data.failed is None
    assert event.data.bounce == Bounce(type="Permanent", sub_type="General", message="No such user")


def test_accepts_bytes_any_header_case_and_one_of_several_signatures() -> None:
    other = sign(PAYLOAD, key=os.urandom(24))
    h = {
        "Webhook-Id": "msg_1",
        "Webhook-Timestamp": TIMESTAMP,
        "Webhook-Signature": f"v1a,ignored {other} {sign(PAYLOAD)}",
    }
    assert verify_webhook(PAYLOAD.encode(), h, SECRET, now=NOW).data.email_id == "em_1"


def test_accepts_list_header_values_and_secrets_without_prefix() -> None:
    h = {k: [v] for k, v in headers().items()}
    assert verify_webhook(PAYLOAD, h, SECRET[len("whsec_") :], now=NOW).type == "email.bounced"


def test_maps_click_and_message_id() -> None:
    payload = json.dumps(
        {
            "type": "email.clicked",
            "created_at": "2026-10-30T12:00:00Z",
            "data": {
                "created_at": "2026-10-30T11:00:00Z",
                "email_id": "em_1",
                "message_id": "<abc@eu-north-1.amazonses.com>",
                "from": "a@acme.com",
                "to": "b@example.com",
                "subject": "s",
                "click": {
                    "ipAddress": "1.2.3.4",
                    "link": "https://acme.com",
                    "timestamp": "2026-10-30T11:59:00Z",
                    "userAgent": "UA",
                },
            },
        }
    )
    event = verify_webhook(payload, headers(payload), SECRET, now=NOW)
    assert event.data.message_id == "<abc@eu-north-1.amazonses.com>"
    assert event.data.to == ["b@example.com"] and event.data.tags == {}
    assert event.data.click is not None and event.data.click.ip_address == "1.2.3.4"
    assert event.data.click.timestamp == datetime(2026, 10, 30, 11, 59, tzinfo=timezone.utc)


def test_rejects_a_tampered_body() -> None:
    with pytest.raises(WebhookVerificationError, match="signature"):
        verify_webhook(PAYLOAD.replace("em_1", "em_2"), headers(), SECRET, now=NOW)


def test_rejects_a_wrong_secret() -> None:
    wrong = "whsec_" + base64.b64encode(os.urandom(24)).decode()
    with pytest.raises(WebhookVerificationError, match="signature"):
        verify_webhook(PAYLOAD, headers(), wrong, now=NOW)
    with pytest.raises(WebhookVerificationError, match="base64"):
        verify_webhook(PAYLOAD, headers(), "whsec_not*base64", now=NOW)


@pytest.mark.parametrize("missing", ["webhook-id", "webhook-timestamp", "webhook-signature"])
def test_rejects_missing_headers(missing: str) -> None:
    h = headers()
    del h[missing]
    with pytest.raises(WebhookVerificationError, match="Missing"):
        verify_webhook(PAYLOAD, h, SECRET, now=NOW)


def test_rejects_timestamps_outside_the_tolerance() -> None:
    stale = str(int(NOW.timestamp()) - 301)
    h = headers(**{"webhook-timestamp": stale, "webhook-signature": sign(PAYLOAD, timestamp=stale)})
    with pytest.raises(WebhookVerificationError, match="tolerance"):
        verify_webhook(PAYLOAD, h, SECRET, now=NOW)
    assert verify_webhook(PAYLOAD, h, SECRET, now=NOW, tolerance_seconds=600).type == "email.bounced"
    future = str(int(NOW.timestamp()) + 301)
    h = headers(**{"webhook-timestamp": future, "webhook-signature": sign(PAYLOAD, timestamp=future)})
    with pytest.raises(WebhookVerificationError):
        verify_webhook(PAYLOAD, h, SECRET, now=NOW)
    with pytest.raises(WebhookVerificationError):
        verify_webhook(PAYLOAD, headers(**{"webhook-timestamp": "soon"}), SECRET, now=NOW)


def test_rejects_signed_payloads_that_are_not_events() -> None:
    with pytest.raises(WebhookVerificationError, match="valid"):
        verify_webhook("not json", headers("not json"), SECRET, now=NOW)
