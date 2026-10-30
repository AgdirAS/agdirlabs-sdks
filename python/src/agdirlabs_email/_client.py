from __future__ import annotations

from types import TracebackType
from typing import Optional, Type

import httpx

from ._http import DEFAULT_MAX_RETRIES, AsyncTransport, Config, SyncTransport
from ._resources import AsyncDomains, AsyncEmails, AsyncWebhooks, Domains, Emails, Webhooks

__all__ = ["AgdirEmail", "AsyncAgdirEmail"]


class AgdirEmail:
    """Agdir Email API client.

    .. code-block:: python

        client = AgdirEmail(api_key="re_...")
        client.emails.send({"from": "Acme <hi@acme.com>", "to": "user@example.com", "subject": "Hi", "html": "..."})

    :param api_key: Defaults to the ``AGDIR_EMAIL_API_KEY`` environment variable.
    :param base_url: Defaults to ``AGDIR_EMAIL_BASE_URL``, else ``https://api.agdir.email``.
    :param max_retries: Retries for rate limits, server errors and network failures. Default 2; 0 disables.
    :param timeout: Per-attempt timeout in seconds. Default 30.
    :param http_client: A custom ``httpx.Client`` (proxies, mTLS, tests). You remain responsible for closing it.
    """

    emails: Emails
    domains: Domains
    webhooks: Webhooks

    def __init__(
        self,
        api_key: Optional[str] = None,
        *,
        base_url: Optional[str] = None,
        max_retries: int = DEFAULT_MAX_RETRIES,
        timeout: Optional[float] = None,
        http_client: Optional[httpx.Client] = None,
    ) -> None:
        self._transport = SyncTransport(Config.resolve(api_key, base_url, max_retries, timeout), http_client)
        self.emails = Emails(self._transport)
        self.domains = Domains(self._transport)
        self.webhooks = Webhooks(self._transport)

    @property
    def base_url(self) -> str:
        return self._transport.config.base_url

    def close(self) -> None:
        """Closes the underlying connection pool (not a caller-supplied ``http_client``)."""
        self._transport.close()

    def __enter__(self) -> AgdirEmail:
        return self

    def __exit__(
        self,
        exc_type: Optional[Type[BaseException]],
        exc: Optional[BaseException],
        tb: Optional[TracebackType],
    ) -> None:
        self.close()

    def __repr__(self) -> str:
        return f"AgdirEmail(base_url={self.base_url!r})"


class AsyncAgdirEmail:
    """Async Agdir Email API client (``asyncio`` / ``trio`` via ``httpx``). Same options as :class:`AgdirEmail`.

    .. code-block:: python

        async with AsyncAgdirEmail() as client:
            await client.emails.send({...})
    """

    emails: AsyncEmails
    domains: AsyncDomains
    webhooks: AsyncWebhooks

    def __init__(
        self,
        api_key: Optional[str] = None,
        *,
        base_url: Optional[str] = None,
        max_retries: int = DEFAULT_MAX_RETRIES,
        timeout: Optional[float] = None,
        http_client: Optional[httpx.AsyncClient] = None,
    ) -> None:
        self._transport = AsyncTransport(Config.resolve(api_key, base_url, max_retries, timeout), http_client)
        self.emails = AsyncEmails(self._transport)
        self.domains = AsyncDomains(self._transport)
        self.webhooks = AsyncWebhooks(self._transport)

    @property
    def base_url(self) -> str:
        return self._transport.config.base_url

    async def aclose(self) -> None:
        """Closes the underlying connection pool (not a caller-supplied ``http_client``)."""
        await self._transport.aclose()

    async def __aenter__(self) -> AsyncAgdirEmail:
        return self

    async def __aexit__(
        self,
        exc_type: Optional[Type[BaseException]],
        exc: Optional[BaseException],
        tb: Optional[TracebackType],
    ) -> None:
        await self.aclose()

    def __repr__(self) -> str:
        return f"AsyncAgdirEmail(base_url={self.base_url!r})"
