from __future__ import annotations

import json
import re
from datetime import datetime, timedelta, timezone
from typing import Any, List

import httpx
import pytest
import respx

from agdirlabs_email import (
    DEFAULT_BASE_URL,
    USER_AGENT,
    AgdirEmail,
    AgdirEmailError,
    DnsRecord,
    IdResult,
    Page,
    SendBatchError,
    SendEmailParams,
    SendResult,
    __version__,
)

from .conftest import BASE_URL, DOMAIN_JSON, EMAIL_JSON, WEBHOOK_JSON

EMAIL: SendEmailParams = {"from": "a@acme.com", "to": "b@example.com", "subject": "s", "text": "t"}


def client(**kwargs: Any) -> AgdirEmail:
    return AgdirEmail("re_test", base_url=BASE_URL, **kwargs)


def body(request: httpx.Request) -> Any:
    return json.loads(request.content)


# Configuration


def test_uses_api_agdir_email_by_default(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("AGDIR_EMAIL_BASE_URL", "")
    assert AgdirEmail("re_test").base_url == DEFAULT_BASE_URL == "https://api.agdir.email"


def test_reads_key_and_base_url_from_env_empty_values_count_as_unset(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("AGDIR_EMAIL_API_KEY", "re_env")
    monkeypatch.setenv("AGDIR_EMAIL_BASE_URL", "http://localhost:8080/")
    assert AgdirEmail().base_url == "http://localhost:8080"
    monkeypatch.setenv("AGDIR_EMAIL_API_KEY", "")
    with pytest.raises(AgdirEmailError, match=r"AGDIR_EMAIL_API_KEY.*https://app\.agdir\.email") as info:
        AgdirEmail()
    assert info.value.code == "missing_api_key"


def test_rejects_invalid_options() -> None:
    with pytest.raises(ValueError, match="max_retries"):
        AgdirEmail("re_test", max_retries=-1)
    with pytest.raises(ValueError, match="timeout"):
        AgdirEmail("re_test", timeout=0)


def test_version_matches_user_agent() -> None:
    assert f"agdirlabs-email-python/{__version__}" == USER_AGENT
    assert re.fullmatch(r"\d+\.\d+\.\d+", __version__)


# Emails


def test_send_maps_params_with_auth_user_agent_and_idempotency_key(api: respx.MockRouter) -> None:
    route = api.post("/emails").respond(200, json={"id": "em_1"})
    result = client().emails.send(
        {
            "from": "Acme <hello@acme.com>",
            "to": "user@example.com",
            "subject": "Hi",
            "text": "Hello",
            "reply_to": "support@acme.com",
            "scheduled_at": datetime(2026, 11, 1, 10, 0, tzinfo=timezone(timedelta(hours=1))),
            "tags": [{"name": "kind", "value": "welcome"}],
            "headers": {"X-Entity-Ref-ID": "1"},
        }
    )
    assert result == SendResult(id="em_1")
    request = route.calls.last.request
    assert request.headers["authorization"] == "Bearer re_test"
    assert request.headers["user-agent"] == USER_AGENT
    assert request.headers["accept"] == "application/json"
    assert request.headers["content-type"] == "application/json"
    assert re.fullmatch(r"[0-9a-f-]{36}", request.headers["idempotency-key"])
    assert body(request) == {
        "from": "Acme <hello@acme.com>",
        "to": "user@example.com",
        "subject": "Hi",
        "text": "Hello",
        "reply_to": "support@acme.com",
        "scheduled_at": "2026-11-01T09:00:00Z",
        "tags": [{"name": "kind", "value": "welcome"}],
        "headers": {"X-Entity-Ref-ID": "1"},
    }


def test_send_rejects_naive_datetimes_and_unknown_fields(api: respx.MockRouter) -> None:
    with pytest.raises(ValueError, match="timezone-aware"):
        client().emails.send({**EMAIL, "scheduled_at": datetime(2026, 11, 1, 9, 0)})
    with pytest.raises(TypeError, match="from_"):
        client().emails.send({**EMAIL, "from_": "x@acme.com"})  # type: ignore[typeddict-unknown-key]


def test_uses_caller_idempotency_key_and_none_when_retries_are_off(api: respx.MockRouter) -> None:
    route = api.post("/emails").respond(200, json={"id": "a"})
    client().emails.send(EMAIL, idempotency_key="order-42")
    client(max_retries=0).emails.send(EMAIL)
    assert route.calls[0].request.headers["idempotency-key"] == "order-42"
    assert "idempotency-key" not in route.calls[1].request.headers


def test_sends_permissive_batches_and_returns_per_index_errors(api: respx.MockRouter) -> None:
    route = api.post("/emails/batch").respond(
        200, json={"data": [{"id": "em_1"}], "errors": [{"index": 1, "message": "bad to"}]}
    )
    result = client().emails.send_batch([EMAIL, {**EMAIL, "to": "nope"}], validation="permissive")
    assert result.data == [SendResult("em_1")]
    assert result.errors == [SendBatchError(index=1, message="bad to")]
    request = route.calls.last.request
    assert request.headers["x-batch-validation"] == "permissive"
    assert [e["to"] for e in body(request)] == ["b@example.com", "nope"]


def test_strict_batch_sends_no_validation_header(api: respx.MockRouter) -> None:
    route = api.post("/emails/batch").respond(200, json={"data": [{"id": "em_1"}]})
    assert client().emails.send_batch([EMAIL]).errors == []
    assert "x-batch-validation" not in route.calls.last.request.headers
    with pytest.raises(ValueError):
        client().emails.send_batch([EMAIL], validation="loose")  # type: ignore[arg-type]


def test_maps_an_email_with_datetimes_and_empty_lists(api: respx.MockRouter) -> None:
    api.get("/emails/em_1").respond(200, json=EMAIL_JSON)
    email = client().emails.get("em_1")
    assert email.from_ == "Acme <hello@acme.com>"
    assert email.to == ["user@example.com"]
    assert email.cc == [] and email.bcc == []
    assert email.reply_to == ["support@acme.com"]
    assert email.html is None and email.text == "Hello"
    assert email.created_at == datetime(2026, 10, 30, 9, 44, 49, 77000, tzinfo=timezone.utc)
    assert email.scheduled_at == datetime(2026, 11, 1, 9, 0, tzinfo=timezone.utc)
    assert email.status == "scheduled"


def test_cancel(api: respx.MockRouter) -> None:
    api.post("/emails/em_1/cancel").respond(200, json={"object": "email", "id": "em_1"})
    assert client().emails.cancel("em_1") == IdResult("em_1")


# Domains


def test_maps_domains_with_dns_records(api: respx.MockRouter) -> None:
    route = api.post("/domains").respond(200, json=DOMAIN_JSON)
    domain = client().domains.create({"name": "acme.com"})
    assert body(route.calls.last.request) == {"name": "acme.com"}
    assert domain.id == "dom_1" and domain.status == "pending" and domain.region == "eu-north-1"
    assert domain.open_tracking is False
    assert domain.records[0] == DnsRecord(
        purpose="DKIM", name="agdir._domainkey", type="TXT", value="p=MIG", ttl="Auto", status="pending"
    )
    assert domain.records[1].priority == 10
    assert "idempotency-key" not in route.calls.last.request.headers


def test_domain_verify_and_delete(api: respx.MockRouter) -> None:
    api.post("/domains/dom_1/verify").respond(200, json={"object": "domain", "id": "dom_1"})
    api.delete("/domains/dom_1").respond(200, json={"object": "domain", "id": "dom_1", "deleted": True})
    c = client()
    assert c.domains.verify("dom_1").id == "dom_1"
    deleted = c.domains.delete("dom_1")
    assert deleted.id == "dom_1" and deleted.deleted is True


def test_returns_pages_with_a_cursor_and_walks_them_with_list_all(api: respx.MockRouter) -> None:
    item = {k: v for k, v in DOMAIN_JSON.items() if k != "records"}
    route = api.get("/domains").mock(
        side_effect=[
            httpx.Response(200, json={"object": "list", "has_more": True, "data": [{**item, "id": "d1"}]}),
            httpx.Response(200, json={"object": "list", "has_more": False, "data": [{**item, "id": "d2"}]}),
            httpx.Response(200, json={"object": "list", "has_more": True, "data": [{**item, "id": "d1"}]}),
        ]
    )
    c = client()
    assert [d.id for d in c.domains.list_all(limit=1)] == ["d1", "d2"]
    assert dict(route.calls[0].request.url.params) == {"limit": "1"}
    assert dict(route.calls[1].request.url.params) == {"limit": "1", "after": "d1"}
    page = c.domains.list(limit=1)
    assert isinstance(page, Page) and page.has_more and page.next_cursor == "d1"


# Webhooks


def test_webhook_create_get_update_rotate_delete(api: respx.MockRouter) -> None:
    create = api.post("/webhooks").respond(200, json={"object": "webhook", "id": "wh_1", "signing_secret": "whsec_a"})
    api.get("/webhooks/wh_1").respond(200, json=WEBHOOK_JSON)
    update = api.patch("/webhooks/wh_1").respond(200, json={"object": "webhook", "id": "wh_1"})
    api.post("/webhooks/wh_1/signing-secret/rotate").respond(
        200, json={"object": "webhook", "id": "wh_1", "signing_secret": "whsec_b"}
    )
    api.delete("/webhooks/wh_1").respond(200, json={"object": "webhook", "id": "wh_1", "deleted": True})
    c = client()

    created = c.webhooks.create({"endpoint": "https://acme.com/hook", "events": ["email.delivered"]})
    assert created.signing_secret == "whsec_a"
    assert body(create.calls.last.request) == {"endpoint": "https://acme.com/hook", "events": ["email.delivered"]}
    webhook = c.webhooks.get("wh_1")
    assert webhook.signing_secret == "whsec_c2VjcmV0"
    assert webhook.events == ["email.delivered", "email.bounced"]
    assert webhook.created_at == datetime(2026, 10, 30, 9, 44, 49, tzinfo=timezone.utc)
    assert c.webhooks.update("wh_1", {"status": "disabled"}).id == "wh_1"
    assert body(update.calls.last.request) == {"status": "disabled"}
    assert c.webhooks.rotate_secret("wh_1").signing_secret == "whsec_b"
    assert c.webhooks.delete("wh_1").deleted is True


def test_webhook_deliveries(api: respx.MockRouter) -> None:
    api.get("/webhooks/wh_1/events").respond(
        200,
        json={
            "object": "list",
            "has_more": False,
            "data": [{"id": "ev_1", "type": "email.sent", "created_at": "2026-10-30T09:00:00Z", "status": "failed"}],
        },
    )
    api.get("/webhooks/wh_1/events/ev_1").respond(
        200,
        json={
            "id": "ev_1",
            "type": "email.sent",
            "created_at": "2026-10-30T09:00:00Z",
            "status": "attempting",
            "next_attempt_at": "2026-10-30T09:05:00Z",
            "payload": {"type": "email.sent"},
        },
    )
    attempts = api.get("/webhooks/wh_1/events/ev_1/attempts").respond(
        200,
        json={
            "object": "list",
            "has_more": False,
            "data": [{"id": "at_1", "http_status_code": 0, "response": "timeout", "sent_at": "2026-10-30T09:00:01Z"}],
        },
    )
    api.post("/webhooks/wh_1/events/ev_1/replay").respond(200, json={"object": "webhook_event", "id": "ev_2"})
    c = client()

    assert [d.status for d in c.webhooks.deliveries.list_all("wh_1")] == ["failed"]
    detail = c.webhooks.deliveries.get("wh_1", "ev_1")
    assert detail.next_attempt_at == datetime(2026, 10, 30, 9, 5, tzinfo=timezone.utc)
    assert detail.payload == {"type": "email.sent"}
    page = c.webhooks.deliveries.attempts("wh_1", "ev_1", limit=5)
    assert page.data[0].http_status == 0 and page.data[0].response == "timeout"
    assert attempts.calls.last.request.url.params["limit"] == "5"
    assert c.webhooks.deliveries.replay("wh_1", "ev_1").id == "ev_2"


def test_escapes_ids_in_paths(api: respx.MockRouter) -> None:
    route = api.get(url__regex=r".*").respond(200, json=WEBHOOK_JSON)
    client().webhooks.get("a/b?c")
    assert route.calls.last.request.url.raw_path == b"/webhooks/a%2Fb%3Fc"


# Errors and retries


def test_raises_agdir_email_error_with_code_status_and_message(api: respx.MockRouter) -> None:
    api.post("/emails").respond(422, json={"statusCode": 422, "name": "validation_error", "message": "Invalid `to`"})
    with pytest.raises(AgdirEmailError) as info:
        client().emails.send(EMAIL)
    assert (info.value.code, info.value.status, info.value.message) == ("validation_error", 422, "Invalid `to`")
    assert str(info.value) == "Invalid `to`"


def test_non_json_error_bodies(api: respx.MockRouter) -> None:
    api.get("/emails/x").respond(404, text="<html>not here</html>")
    with pytest.raises(AgdirEmailError) as info:
        client().emails.get("x")
    assert info.value.message == "Request failed with HTTP 404"
    assert info.value.code == "unknown_error"


def test_retries_rate_limits_after_retry_after_reusing_the_idempotency_key(
    api: respx.MockRouter, sleeps: List[float]
) -> None:
    limited = httpx.Response(
        429, json={"name": "rate_limit_exceeded", "message": "slow down"}, headers={"retry-after": "1"}
    )
    route = api.post("/emails").mock(side_effect=[limited, httpx.Response(200, json={"id": "em_1"})])
    assert client().emails.send(EMAIL).id == "em_1"
    assert sleeps == [1.0]
    keys = {call.request.headers["idempotency-key"] for call in route.calls}
    assert len(route.calls) == 2 and len(keys) == 1


def test_rate_limits_are_retried_even_without_an_idempotency_key(api: respx.MockRouter) -> None:
    limited = httpx.Response(429, json={"name": "rate_limit_exceeded", "message": "slow"})
    route = api.post("/domains").mock(side_effect=[limited, httpx.Response(200, json=DOMAIN_JSON)])
    assert client().domains.create({"name": "acme.com"}).id == "dom_1"
    assert route.call_count == 2


def test_gives_up_on_rate_limits_after_max_retries(api: respx.MockRouter, sleeps: List[float]) -> None:
    route = api.get("/emails/em_1").respond(
        429, json={"name": "rate_limit_exceeded", "message": "slow"}, headers={"retry-after": "2"}
    )
    with pytest.raises(AgdirEmailError) as info:
        client().emails.get("em_1")
    assert info.value.code == "rate_limit_exceeded" and info.value.retry_after == 2.0
    assert route.call_count == 3 and sleeps == [2.0, 2.0]


@pytest.mark.parametrize("code", ["daily_quota_exceeded", "monthly_quota_exceeded"])
def test_does_not_retry_quota_errors(api: respx.MockRouter, code: str) -> None:
    route = api.post("/emails").respond(429, json={"name": code, "message": "quota"})
    with pytest.raises(AgdirEmailError) as info:
        client().emails.send(EMAIL)
    assert info.value.code == code and route.call_count == 1


def test_retries_get_server_errors_and_network_failures_then_gives_up(
    api: respx.MockRouter, sleeps: List[float]
) -> None:
    route = api.get("/emails/em_1").mock(
        side_effect=[httpx.Response(503, text="busy"), httpx.ConnectError("refused"), httpx.Response(502)]
    )
    with pytest.raises(AgdirEmailError) as info:
        client().emails.get("em_1")
    assert info.value.code == "internal_server_error" and info.value.status == 502
    assert route.call_count == 3 and len(sleeps) == 2
    assert all(0 < s <= 8 for s in sleeps)


def test_retries_a_send_server_error_because_it_carries_an_idempotency_key(api: respx.MockRouter) -> None:
    route = api.post("/emails").mock(side_effect=[httpx.Response(500), httpx.Response(200, json={"id": "em_1"})])
    assert client().emails.send(EMAIL).id == "em_1"
    assert route.call_count == 2


def test_does_not_retry_a_non_idempotent_post_after_a_network_failure(api: respx.MockRouter) -> None:
    route = api.post("/domains").mock(side_effect=httpx.ConnectError("refused"))
    with pytest.raises(AgdirEmailError) as info:
        client().domains.create({"name": "acme.com"})
    assert info.value.code == "network_error" and info.value.status == 0
    assert isinstance(info.value.__cause__, httpx.ConnectError)
    assert route.call_count == 1


def test_timeouts_are_reported_as_timeout(api: respx.MockRouter) -> None:
    route = api.post("/emails").mock(side_effect=httpx.ReadTimeout("slow"))
    with pytest.raises(AgdirEmailError) as info:
        client(max_retries=0, timeout=5).emails.send(EMAIL)
    assert info.value.code == "timeout" and "5 s" in info.value.message
    assert route.call_count == 1


def test_parses_http_date_retry_after() -> None:
    from agdirlabs_email._http import to_error

    response = httpx.Response(
        429,
        json={"name": "rate_limit_exceeded", "message": "x"},
        headers={"retry-after": "Wed, 21 Oct 2015 07:28:00 GMT"},
    )
    assert to_error(response).retry_after == 0.0


# Client lifecycle


def test_uses_a_custom_http_client_and_leaves_it_open() -> None:
    seen: List[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"id": "em_1"})

    http = httpx.Client(transport=httpx.MockTransport(handler))
    with AgdirEmail("re_test", http_client=http) as c:
        c.emails.send(EMAIL)
    assert str(seen[0].url) == "https://api.agdir.email/emails"
    assert not http.is_closed
    http.close()


def test_context_manager_closes_its_own_client() -> None:
    with AgdirEmail("re_test") as c:
        pass
    assert c._transport._client.is_closed
