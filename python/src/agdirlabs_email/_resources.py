"""API resources: ``emails``, ``domains``, ``webhooks`` (sync and async)."""

from __future__ import annotations

from typing import AsyncIterator, Awaitable, Callable, Iterator, List, Optional, TypeVar

from . import _operations as ops
from ._http import AsyncTransport, SyncTransport
from ._operations import BatchValidation
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

T = TypeVar("T")


def _paginate(fetch: Callable[[Optional[str]], Page[T]], after: Optional[str]) -> Iterator[T]:
    while True:
        page = fetch(after)
        yield from page.data
        if not page.next_cursor:
            return
        after = page.next_cursor


async def _apaginate(fetch: Callable[[Optional[str]], Awaitable[Page[T]]], after: Optional[str]) -> AsyncIterator[T]:
    while True:
        page = await fetch(after)
        for item in page.data:
            yield item
        if not page.next_cursor:
            return
        after = page.next_cursor


# ---------------------------------------------------------------------------------------------------------------------
# Sync


class Emails:
    def __init__(self, transport: SyncTransport) -> None:
        self._t = transport

    def send(self, params: SendEmailParams, *, idempotency_key: Optional[str] = None) -> SendResult:
        """Accepts one email for delivery (or for ``scheduled_at``).

        ``idempotency_key`` makes the send safe to retry: the same key with the same body returns the original
        result. One is generated automatically when retries are enabled.
        """
        key = ops.idempotency_key(idempotency_key, self._t.config.max_retries)
        return self._t.call(ops.send_email(params, key))

    def send_batch(
        self,
        emails: List[SendEmailParams],
        *,
        validation: BatchValidation = "strict",
        idempotency_key: Optional[str] = None,
    ) -> SendBatchResult:
        """Accepts up to 100 emails in one request.

        ``strict`` (default): one invalid email rejects the whole batch. ``permissive``: valid emails are accepted,
        invalid ones are reported in ``errors``.
        """
        key = ops.idempotency_key(idempotency_key, self._t.config.max_retries)
        return self._t.call(ops.send_batch(emails, key, validation))

    def get(self, email_id: str) -> Email:
        """Reads an email and its current delivery status. Requires a full-access key."""
        return self._t.call(ops.get_email(email_id))

    def cancel(self, email_id: str) -> IdResult:
        """Cancels a scheduled email that has not been sent yet."""
        return self._t.call(ops.cancel_email(email_id))


class Domains:
    def __init__(self, transport: SyncTransport) -> None:
        self._t = transport

    def create(self, params: CreateDomainParams) -> DomainWithRecords:
        """Adds a sending domain. Publish the returned ``records`` in DNS, then call :meth:`verify`."""
        return self._t.call(ops.create_domain(params))

    def get(self, domain_id: str) -> DomainWithRecords:
        return self._t.call(ops.get_domain(domain_id))

    def list(self, *, limit: Optional[int] = None, after: Optional[str] = None) -> Page[Domain]:
        """One page of domains. Pass ``page.next_cursor`` as ``after`` for the next one."""
        return self._t.call(ops.list_domains(limit, after))

    def list_all(self, *, limit: Optional[int] = None, after: Optional[str] = None) -> Iterator[Domain]:
        """Every domain, fetching further pages (of ``limit`` items) as you iterate."""
        return _paginate(lambda cursor: self.list(limit=limit, after=cursor), after)

    def verify(self, domain_id: str) -> IdResult:
        """Starts a DNS check now instead of waiting for the next automatic one."""
        return self._t.call(ops.verify_domain(domain_id))

    def delete(self, domain_id: str) -> DeleteResult:
        return self._t.call(ops.delete_domain(domain_id))


class WebhookDeliveries:
    """Messages sent (or queued) to one webhook endpoint, with their HTTP attempts."""

    def __init__(self, transport: SyncTransport) -> None:
        self._t = transport

    def list(
        self, webhook_id: str, *, limit: Optional[int] = None, after: Optional[str] = None
    ) -> Page[WebhookDelivery]:
        return self._t.call(ops.list_deliveries(webhook_id, limit, after))

    def list_all(
        self, webhook_id: str, *, limit: Optional[int] = None, after: Optional[str] = None
    ) -> Iterator[WebhookDelivery]:
        return _paginate(lambda cursor: self.list(webhook_id, limit=limit, after=cursor), after)

    def get(self, webhook_id: str, delivery_id: str) -> WebhookDeliveryDetail:
        return self._t.call(ops.get_delivery(webhook_id, delivery_id))

    def attempts(
        self, webhook_id: str, delivery_id: str, *, limit: Optional[int] = None, after: Optional[str] = None
    ) -> Page[WebhookAttempt]:
        """HTTP attempts for one delivery, newest first."""
        return self._t.call(ops.list_attempts(webhook_id, delivery_id, limit, after))

    def replay(self, webhook_id: str, delivery_id: str) -> IdResult:
        """Sends the identical message again, once, right now (also to disabled endpoints)."""
        return self._t.call(ops.replay_delivery(webhook_id, delivery_id))


class Webhooks:
    deliveries: WebhookDeliveries

    def __init__(self, transport: SyncTransport) -> None:
        self._t = transport
        self.deliveries = WebhookDeliveries(transport)

    def create(self, params: CreateWebhookParams) -> SecretResult:
        """Registers an endpoint. Keep the returned ``signing_secret`` to verify incoming events."""
        return self._t.call(ops.create_webhook(params))

    def get(self, webhook_id: str) -> WebhookWithSecret:
        return self._t.call(ops.get_webhook(webhook_id))

    def list(self, *, limit: Optional[int] = None, after: Optional[str] = None) -> Page[Webhook]:
        return self._t.call(ops.list_webhooks(limit, after))

    def list_all(self, *, limit: Optional[int] = None, after: Optional[str] = None) -> Iterator[Webhook]:
        return _paginate(lambda cursor: self.list(limit=limit, after=cursor), after)

    def update(self, webhook_id: str, params: UpdateWebhookParams) -> IdResult:
        return self._t.call(ops.update_webhook(webhook_id, params))

    def delete(self, webhook_id: str) -> DeleteResult:
        return self._t.call(ops.delete_webhook(webhook_id))

    def rotate_secret(self, webhook_id: str) -> SecretResult:
        """Issues a new signing secret; the old one stops working immediately."""
        return self._t.call(ops.rotate_secret(webhook_id))


# ---------------------------------------------------------------------------------------------------------------------
# Async


class AsyncEmails:
    def __init__(self, transport: AsyncTransport) -> None:
        self._t = transport

    async def send(self, params: SendEmailParams, *, idempotency_key: Optional[str] = None) -> SendResult:
        """Accepts one email for delivery (or for ``scheduled_at``). See :meth:`Emails.send`."""
        key = ops.idempotency_key(idempotency_key, self._t.config.max_retries)
        return await self._t.call(ops.send_email(params, key))

    async def send_batch(
        self,
        emails: List[SendEmailParams],
        *,
        validation: BatchValidation = "strict",
        idempotency_key: Optional[str] = None,
    ) -> SendBatchResult:
        """Accepts up to 100 emails in one request. See :meth:`Emails.send_batch`."""
        key = ops.idempotency_key(idempotency_key, self._t.config.max_retries)
        return await self._t.call(ops.send_batch(emails, key, validation))

    async def get(self, email_id: str) -> Email:
        return await self._t.call(ops.get_email(email_id))

    async def cancel(self, email_id: str) -> IdResult:
        return await self._t.call(ops.cancel_email(email_id))


class AsyncDomains:
    def __init__(self, transport: AsyncTransport) -> None:
        self._t = transport

    async def create(self, params: CreateDomainParams) -> DomainWithRecords:
        return await self._t.call(ops.create_domain(params))

    async def get(self, domain_id: str) -> DomainWithRecords:
        return await self._t.call(ops.get_domain(domain_id))

    async def list(self, *, limit: Optional[int] = None, after: Optional[str] = None) -> Page[Domain]:
        return await self._t.call(ops.list_domains(limit, after))

    def list_all(self, *, limit: Optional[int] = None, after: Optional[str] = None) -> AsyncIterator[Domain]:
        """Every domain: ``async for domain in client.domains.list_all(): ...``."""
        return _apaginate(lambda cursor: self.list(limit=limit, after=cursor), after)

    async def verify(self, domain_id: str) -> IdResult:
        return await self._t.call(ops.verify_domain(domain_id))

    async def delete(self, domain_id: str) -> DeleteResult:
        return await self._t.call(ops.delete_domain(domain_id))


class AsyncWebhookDeliveries:
    def __init__(self, transport: AsyncTransport) -> None:
        self._t = transport

    async def list(
        self, webhook_id: str, *, limit: Optional[int] = None, after: Optional[str] = None
    ) -> Page[WebhookDelivery]:
        return await self._t.call(ops.list_deliveries(webhook_id, limit, after))

    def list_all(
        self, webhook_id: str, *, limit: Optional[int] = None, after: Optional[str] = None
    ) -> AsyncIterator[WebhookDelivery]:
        return _apaginate(lambda cursor: self.list(webhook_id, limit=limit, after=cursor), after)

    async def get(self, webhook_id: str, delivery_id: str) -> WebhookDeliveryDetail:
        return await self._t.call(ops.get_delivery(webhook_id, delivery_id))

    async def attempts(
        self, webhook_id: str, delivery_id: str, *, limit: Optional[int] = None, after: Optional[str] = None
    ) -> Page[WebhookAttempt]:
        return await self._t.call(ops.list_attempts(webhook_id, delivery_id, limit, after))

    async def replay(self, webhook_id: str, delivery_id: str) -> IdResult:
        return await self._t.call(ops.replay_delivery(webhook_id, delivery_id))


class AsyncWebhooks:
    deliveries: AsyncWebhookDeliveries

    def __init__(self, transport: AsyncTransport) -> None:
        self._t = transport
        self.deliveries = AsyncWebhookDeliveries(transport)

    async def create(self, params: CreateWebhookParams) -> SecretResult:
        return await self._t.call(ops.create_webhook(params))

    async def get(self, webhook_id: str) -> WebhookWithSecret:
        return await self._t.call(ops.get_webhook(webhook_id))

    async def list(self, *, limit: Optional[int] = None, after: Optional[str] = None) -> Page[Webhook]:
        return await self._t.call(ops.list_webhooks(limit, after))

    def list_all(self, *, limit: Optional[int] = None, after: Optional[str] = None) -> AsyncIterator[Webhook]:
        return _apaginate(lambda cursor: self.list(limit=limit, after=cursor), after)

    async def update(self, webhook_id: str, params: UpdateWebhookParams) -> IdResult:
        return await self._t.call(ops.update_webhook(webhook_id, params))

    async def delete(self, webhook_id: str) -> DeleteResult:
        return await self._t.call(ops.delete_webhook(webhook_id))

    async def rotate_secret(self, webhook_id: str) -> SecretResult:
        return await self._t.call(ops.rotate_secret(webhook_id))
