from __future__ import annotations

from typing import Iterator, List

import pytest
import respx

from agdirlabs_email import _http

BASE_URL = "https://api.test"


@pytest.fixture(autouse=True)
def _clean_env(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("AGDIR_EMAIL_API_KEY", raising=False)
    monkeypatch.delenv("AGDIR_EMAIL_BASE_URL", raising=False)


@pytest.fixture
def sleeps(monkeypatch: pytest.MonkeyPatch) -> List[float]:
    """Records retry waits instead of sleeping."""
    waits: List[float] = []

    async def async_sleep(seconds: float) -> None:
        waits.append(seconds)

    monkeypatch.setattr(_http, "_sleep", waits.append)
    monkeypatch.setattr(_http, "_async_sleep", async_sleep)
    return waits


@pytest.fixture
def api(sleeps: List[float]) -> Iterator[respx.MockRouter]:
    with respx.mock(base_url=BASE_URL, assert_all_called=False, assert_all_mocked=True) as router:
        yield router


EMAIL_JSON = {
    "object": "email",
    "id": "em_1",
    "from": "Acme <hello@acme.com>",
    "to": ["user@example.com"],
    "cc": None,
    "bcc": None,
    "reply_to": "support@acme.com",
    "subject": "Hi",
    "html": None,
    "text": "Hello",
    "created_at": "2026-10-30T09:44:49.077Z",
    "scheduled_at": "2026-11-01T09:00:00+00:00",
    "last_event": "scheduled",
}

DOMAIN_JSON = {
    "object": "domain",
    "id": "dom_1",
    "name": "acme.com",
    "status": "pending",
    "created_at": "2026-10-30T09:44:49.077Z",
    "region": "eu-north-1",
    "capabilities": {"sending": "enabled", "receiving": "disabled"},
    "open_tracking": False,
    "click_tracking": False,
    "records": [
        {
            "record": "DKIM",
            "name": "agdir._domainkey",
            "type": "TXT",
            "ttl": "Auto",
            "status": "pending",
            "value": "p=MIG",
        },
        {
            "record": "SPF",
            "name": "send",
            "type": "MX",
            "ttl": "Auto",
            "status": "pending",
            "value": "feedback-smtp.eu-north-1.amazonses.com",
            "priority": 10,
        },
    ],
}

WEBHOOK_JSON = {
    "object": "webhook",
    "id": "wh_1",
    "endpoint": "https://acme.com/hook",
    "created_at": "2026-10-30T09:44:49Z",
    "status": "enabled",
    "events": ["email.delivered", "email.bounced"],
    "signing_secret": "whsec_c2VjcmV0",
}
