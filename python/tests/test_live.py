"""Opt-in integration tests against a real Agdir Email API (never production in CI).

AGDIR_EMAIL_LIVE_BASE_URL=<test API base URL> AGDIR_EMAIL_LIVE_API_KEY=re_... \
AGDIR_EMAIL_LIVE_FROM=hi@python.sdk-test.example.com pytest tests/test_live.py
"""

from __future__ import annotations

import os
import time
import uuid
from datetime import datetime, timedelta, timezone
from typing import Iterator, Optional

import pytest

from agdirlabs_email import AgdirEmail, AgdirEmailError, AsyncAgdirEmail, SendEmailParams

BASE_URL = os.environ.get("AGDIR_EMAIL_LIVE_BASE_URL") or ""
API_KEY = os.environ.get("AGDIR_EMAIL_LIVE_API_KEY") or ""
FROM = os.environ.get("AGDIR_EMAIL_LIVE_FROM") or "hi@python.sdk-test.example.com"

pytestmark = pytest.mark.skipif(
    not (BASE_URL and API_KEY), reason="set AGDIR_EMAIL_LIVE_BASE_URL and AGDIR_EMAIL_LIVE_API_KEY"
)


@pytest.fixture
def client() -> Iterator[AgdirEmail]:
    # Generous retries: the default rate limit is 10 req/s per team, and these tests exercise the retry path.
    with AgdirEmail(API_KEY, base_url=BASE_URL, max_retries=6) as c:
        yield c


def email(**extra: object) -> SendEmailParams:
    params: SendEmailParams = {
        "from": f"Python SDK <{FROM}>",
        "to": "delivered@example.com",
        "subject": "agdirlabs-email live test",
        "text": "Hello from the Python SDK",
    }
    params.update(extra)  # type: ignore[typeddict-item]
    return params


def test_schedule_get_cancel(client: AgdirEmail) -> None:
    scheduled_at = datetime.now(timezone.utc).replace(microsecond=0) + timedelta(hours=1)
    sent = client.emails.send(email(scheduled_at=scheduled_at, tags=[{"name": "sdk", "value": "python"}]))
    fetched = client.emails.get(sent.id)
    assert fetched.status == "scheduled"
    assert fetched.scheduled_at == scheduled_at
    assert fetched.to == ["delivered@example.com"] and fetched.cc == []
    assert fetched.created_at.tzinfo is not None
    assert client.emails.cancel(sent.id).id == sent.id
    assert client.emails.get(sent.id).status == "canceled"


def test_idempotent_replay_returns_the_same_id(client: AgdirEmail) -> None:
    key = f"python-live-{uuid.uuid4()}"
    params = email(scheduled_at=datetime.now(timezone.utc) + timedelta(hours=1))
    first = client.emails.send(params, idempotency_key=key)
    second = client.emails.send(params, idempotency_key=key)
    assert first.id == second.id
    client.emails.cancel(first.id)


def test_permissive_batch_reports_the_bad_recipient(client: AgdirEmail) -> None:
    later = datetime.now(timezone.utc) + timedelta(hours=1)
    result = client.emails.send_batch(
        [email(scheduled_at=later), email(to="not-an-address", scheduled_at=later)], validation="permissive"
    )
    assert len(result.data) == 1
    assert [e.index for e in result.errors] == [1]
    client.emails.cancel(result.data[0].id)


def test_validation_error_on_a_bad_recipient(client: AgdirEmail) -> None:
    with pytest.raises(AgdirEmailError) as info:
        client.emails.send(email(to="not-an-address"))
    assert info.value.code == "validation_error" and info.value.status == 422


def test_invalid_api_key() -> None:
    with AgdirEmail("re_invalid", base_url=BASE_URL) as c, pytest.raises(AgdirEmailError) as info:
        c.domains.list()
    assert info.value.code == "invalid_api_key" and info.value.status == 403


def test_domain_lifecycle(client: AgdirEmail) -> None:
    name = f"python-{int(time.time() * 1000)}.test"
    domain_id: Optional[str] = None
    try:
        created = client.domains.create({"name": name})
        domain_id = created.id
        assert created.name == name
        assert {r.purpose for r in created.records} >= {"DKIM", "SPF"}
        assert client.domains.get(created.id).id == created.id
        assert client.domains.verify(created.id).id == created.id
        assert created.id in [d.id for d in client.domains.list_all(limit=1)]
        assert client.domains.delete(created.id).deleted is True
        domain_id = None
        with pytest.raises(AgdirEmailError) as info:
            client.domains.get(created.id)
        assert info.value.code == "not_found" and info.value.status == 404
    finally:
        if domain_id:
            client.domains.delete(domain_id)


def test_webhook_lifecycle(client: AgdirEmail) -> None:
    created = client.webhooks.create({"endpoint": "https://93.184.216.34/hook", "events": ["email.delivered"]})
    try:
        assert created.signing_secret.startswith("whsec_")
        webhook = client.webhooks.get(created.id)
        assert webhook.endpoint == "https://93.184.216.34/hook"
        assert webhook.status == "enabled" and webhook.events == ["email.delivered"]
        assert webhook.signing_secret == created.signing_secret
        client.webhooks.update(created.id, {"status": "disabled"})
        assert client.webhooks.get(created.id).status == "disabled"
        rotated = client.webhooks.rotate_secret(created.id)
        assert rotated.signing_secret.startswith("whsec_") and rotated.signing_secret != created.signing_secret
        page = client.webhooks.deliveries.list(created.id)
        assert page.data == [] and page.has_more is False and page.next_cursor is None
    finally:
        assert client.webhooks.delete(created.id).deleted is True


async def test_async_client_round_trip() -> None:
    async with AsyncAgdirEmail(API_KEY, base_url=BASE_URL, max_retries=6) as c:
        sent = await c.emails.send(email(scheduled_at=datetime.now(timezone.utc) + timedelta(hours=1)))
        assert (await c.emails.get(sent.id)).status == "scheduled"
        await c.emails.cancel(sent.id)
        assert [d.name async for d in c.domains.list_all() if d.name == FROM.split("@")[1]]
