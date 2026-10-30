from __future__ import annotations

import json
import warnings
from datetime import datetime, timezone
from typing import Any

import httpx
import pytest
import respx

pytest.importorskip("django")

# These tests also cover the pre-6.1 EMAIL_BACKEND / get_connection() APIs that newer Django deprecates.
pytestmark = [
    pytest.mark.filterwarnings("ignore:.*deprecated"),
    pytest.mark.filterwarnings("ignore:.*Django 2028"),
]

from django.conf import settings

if not settings.configured:
    with warnings.catch_warnings():
        warnings.simplefilter("ignore")
        settings.configure(
            EMAIL_BACKEND="agdirlabs_email.django.EmailBackend",
            AGDIR_EMAIL_API_KEY="re_django",
            AGDIR_EMAIL_BASE_URL="https://api.test",
            DEFAULT_FROM_EMAIL="Acme <hello@acme.com>",
        )

from django.core import mail
from django.core.mail import EmailMessage, EmailMultiAlternatives
from django.test import override_settings

from agdirlabs_email import AgdirEmailError
from agdirlabs_email.django import AttachmentsNotSupportedError, EmailBackend


def sent_bodies(route: respx.Route) -> Any:
    return [json.loads(call.request.content) for call in route.calls]


def test_send_mail_uses_the_backend_and_settings(api: respx.MockRouter) -> None:
    route = api.post("/emails").respond(200, json={"id": "em_1"})
    assert mail.send_mail("Hi", "Hello there", None, ["user@example.com"]) == 1
    request = route.calls.last.request
    assert request.headers["authorization"] == "Bearer re_django"
    assert sent_bodies(route) == [
        {"from": "Acme <hello@acme.com>", "to": ["user@example.com"], "subject": "Hi", "text": "Hello there"}
    ]


def test_maps_multi_alternatives_with_cc_bcc_reply_to_and_headers(api: respx.MockRouter) -> None:
    route = api.post("/emails").respond(200, json={"id": "em_2"})
    message = EmailMultiAlternatives(
        subject="Invoice",
        body="Plain body",
        from_email="Billing <billing@acme.com>",
        to=["ana@example.com", "ben@example.com"],
        cc=["finance@acme.com"],
        bcc=["audit@acme.com"],
        reply_to=["support@acme.com"],
        headers={"X-Entity-Ref-ID": "inv-1042", "To": "display override"},
    )
    message.attach_alternative("<p>HTML body</p>", "text/html")
    assert message.send() == 1
    assert sent_bodies(route) == [
        {
            "from": "Billing <billing@acme.com>",
            "to": ["ana@example.com", "ben@example.com"],
            "cc": ["finance@acme.com"],
            "bcc": ["audit@acme.com"],
            "reply_to": ["support@acme.com"],
            "subject": "Invoice",
            "text": "Plain body",
            "html": "<p>HTML body</p>",
            "headers": {"X-Entity-Ref-ID": "inv-1042"},
        }
    ]
    assert message.agdir_email_id == "em_2"


def test_html_content_subtype_tags_and_schedule(api: respx.MockRouter) -> None:
    route = api.post("/emails").respond(200, json={"id": "em_3"})
    message = EmailMessage("Hi", "<b>Hello</b>", to=["user@example.com"], headers={"Reply-To": "a@x.com, b@x.com"})
    message.content_subtype = "html"
    message.tags = [{"name": "kind", "value": "welcome"}]
    message.scheduled_at = datetime(2026, 11, 1, 9, 0, tzinfo=timezone.utc)
    message.send()
    (sent,) = sent_bodies(route)
    assert sent["html"] == "<b>Hello</b>" and "text" not in sent
    assert sent["reply_to"] == ["a@x.com", "b@x.com"]
    assert "headers" not in sent
    assert sent["tags"] == [{"name": "kind", "value": "welcome"}]
    assert sent["scheduled_at"] == "2026-11-01T09:00:00Z"


def test_sends_several_messages_on_one_connection_and_skips_ones_without_recipients(api: respx.MockRouter) -> None:
    route = api.post("/emails").respond(200, json={"id": "em_4"})
    messages = [
        EmailMessage("A", "a", to=["a@example.com"]),
        EmailMessage("B", "b", to=[]),
        EmailMessage("C", "c", bcc=["c@example.com"]),
    ]
    with mail.get_connection() as connection:
        assert connection.send_messages(messages) == 2
    assert [b["subject"] for b in sent_bodies(route)] == ["A", "C"]


def test_rejects_attachments_clearly(api: respx.MockRouter) -> None:
    message = EmailMessage("Hi", "Hello", to=["user@example.com"])
    message.attach("report.csv", "a,b\n", "text/csv")
    with pytest.raises(AttachmentsNotSupportedError, match="attachments"):
        message.send()


def test_raises_api_errors_unless_fail_silently(api: respx.MockRouter) -> None:
    api.post("/emails").respond(403, json={"name": "validation_error", "message": "The domain is not verified"})
    with pytest.raises(AgdirEmailError, match="not verified"):
        mail.send_mail("Hi", "Hello", "x@unverified.com", ["user@example.com"])
    assert mail.send_mail("Hi", "Hello", "x@unverified.com", ["user@example.com"], fail_silently=True) == 0


@pytest.mark.skipif(not hasattr(mail, "mailers"), reason="MAILERS is new in Django 6.1")
def test_configured_through_mailers_without_deprecation_warnings(api: respx.MockRouter) -> None:
    route = api.post("/emails").respond(200, json={"id": "em_6"})
    mailers = {
        "default": {
            "BACKEND": "agdirlabs_email.django.EmailBackend",
            "OPTIONS": {"api_key": "re_mailers", "max_retries": 1},
        }
    }
    with override_settings(MAILERS=mailers), warnings.catch_warnings():
        warnings.simplefilter("error")
        assert mail.send_mail("Hi", "Hello", None, ["user@example.com"]) == 1
    assert route.calls.last.request.headers["authorization"] == "Bearer re_mailers"


def test_connection_kwargs_override_settings() -> None:
    with respx.mock(assert_all_mocked=True) as router:
        route = router.post("https://other.test/emails").mock(return_value=httpx.Response(200, json={"id": "em_5"}))
        connection = mail.get_connection(api_key="re_kwarg", base_url="https://other.test", max_retries=0)
        assert isinstance(connection, EmailBackend)
        EmailMessage("Hi", "Hello", to=["user@example.com"], connection=connection).send()
    request = route.calls.last.request
    assert request.headers["authorization"] == "Bearer re_kwarg"
    assert "idempotency-key" not in request.headers
