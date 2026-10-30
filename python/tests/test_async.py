from __future__ import annotations

import json
from typing import Any, List

import httpx
import pytest
import respx

from agdirlabs_email import AgdirEmailError, AsyncAgdirEmail, SendEmailParams

from .conftest import BASE_URL, DOMAIN_JSON, EMAIL_JSON, WEBHOOK_JSON

EMAIL: SendEmailParams = {"from": "a@acme.com", "to": "b@example.com", "subject": "s", "html": "<p>t</p>"}


def client(**kwargs: Any) -> AsyncAgdirEmail:
    return AsyncAgdirEmail("re_test", base_url=BASE_URL, **kwargs)


async def test_send_get_cancel(api: respx.MockRouter) -> None:
    send = api.post("/emails").respond(200, json={"id": "em_1"})
    api.get("/emails/em_1").respond(200, json=EMAIL_JSON)
    api.post("/emails/em_1/cancel").respond(200, json={"object": "email", "id": "em_1"})
    async with client() as c:
        assert (await c.emails.send(EMAIL)).id == "em_1"
        assert (await c.emails.get("em_1")).status == "scheduled"
        assert (await c.emails.cancel("em_1")).id == "em_1"
    request = send.calls.last.request
    assert json.loads(request.content)["html"] == "<p>t</p>"
    assert request.headers["authorization"] == "Bearer re_test"
    assert len(request.headers["idempotency-key"]) == 36


async def test_batch(api: respx.MockRouter) -> None:
    route = api.post("/emails/batch").respond(200, json={"data": [{"id": "a"}, {"id": "b"}]})
    async with client() as c:
        result = await c.emails.send_batch([EMAIL, EMAIL], idempotency_key="batch-1")
    assert [r.id for r in result.data] == ["a", "b"]
    assert route.calls.last.request.headers["idempotency-key"] == "batch-1"


async def test_list_all_is_an_async_iterator(api: respx.MockRouter) -> None:
    item = {k: v for k, v in DOMAIN_JSON.items() if k != "records"}
    route = api.get("/domains").mock(
        side_effect=[
            httpx.Response(200, json={"object": "list", "has_more": True, "data": [{**item, "id": "d1"}]}),
            httpx.Response(200, json={"object": "list", "has_more": False, "data": [{**item, "id": "d2"}]}),
        ]
    )
    async with client() as c:
        assert [d.id async for d in c.domains.list_all(limit=1)] == ["d1", "d2"]
    assert route.calls[1].request.url.params["after"] == "d1"


async def test_domains_and_webhooks(api: respx.MockRouter) -> None:
    api.post("/domains").respond(200, json=DOMAIN_JSON)
    api.post("/domains/dom_1/verify").respond(200, json={"id": "dom_1"})
    api.delete("/domains/dom_1").respond(200, json={"id": "dom_1", "deleted": True})
    api.get("/webhooks").respond(200, json={"object": "list", "has_more": False, "data": [WEBHOOK_JSON]})
    api.get("/webhooks/wh_1/events").respond(200, json={"object": "list", "has_more": False, "data": []})
    async with client() as c:
        domain = await c.domains.create({"name": "acme.com", "region": "eu-north-1"})
        assert len(domain.records) == 2
        assert (await c.domains.verify("dom_1")).id == "dom_1"
        assert (await c.domains.delete("dom_1")).deleted
        assert [w.id async for w in c.webhooks.list_all()] == ["wh_1"]
        assert (await c.webhooks.deliveries.list("wh_1")).data == []


async def test_retries_rate_limits_and_maps_errors(api: respx.MockRouter, sleeps: List[float]) -> None:
    route = api.get("/emails/em_1").mock(
        side_effect=[
            httpx.Response(429, json={"name": "rate_limit_exceeded", "message": "slow"}, headers={"retry-after": "1"}),
            httpx.ConnectError("refused"),
            httpx.Response(404, json={"name": "not_found", "message": "Email not found"}),
        ]
    )
    async with client() as c:
        with pytest.raises(AgdirEmailError) as info:
            await c.emails.get("em_1")
    assert info.value.code == "not_found" and info.value.status == 404
    assert route.call_count == 3 and sleeps[0] == 1.0


async def test_does_not_retry_non_idempotent_posts_on_network_errors(api: respx.MockRouter) -> None:
    route = api.post("/webhooks").mock(side_effect=httpx.ConnectError("refused"))
    async with client() as c:
        with pytest.raises(AgdirEmailError) as info:
            await c.webhooks.create({"endpoint": "https://acme.com/hook", "events": ["email.sent"]})
    assert info.value.code == "network_error" and route.call_count == 1


async def test_uses_a_custom_async_http_client() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, json={"id": "em_9"})

    http = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    async with AsyncAgdirEmail("re_test", http_client=http) as c:
        assert (await c.emails.send(EMAIL)).id == "em_9"
    assert not http.is_closed
    await http.aclose()
