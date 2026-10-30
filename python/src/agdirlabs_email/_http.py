"""HTTP transport shared by the sync and async clients: request building, error mapping and retries."""

from __future__ import annotations

import asyncio
import os
import random
import time
from dataclasses import dataclass, field
from email.utils import parsedate_to_datetime
from typing import Any, Callable, Dict, Generic, Mapping, Optional, TypeVar, Union
from urllib.parse import quote

import httpx

from ._errors import AgdirEmailError
from ._version import USER_AGENT

T = TypeVar("T")

DEFAULT_BASE_URL = "https://api.agdir.email"
DEFAULT_TIMEOUT = 30.0
DEFAULT_MAX_RETRIES = 2

_RETRY_STATUSES = frozenset({408, 500, 502, 503, 504})


def seg(value: str) -> str:
    """Escapes an id for use as one URL path segment."""
    return quote(value, safe="")


@dataclass(frozen=True)
class Operation(Generic[T]):
    """One API call: what to send and how to map the JSON response."""

    method: str
    path: str
    parse: Callable[[Any], T]
    retryable: bool
    """Safe to repeat: GETs, and POSTs that carry an idempotency key."""
    query: Mapping[str, Union[str, int, None]] = field(default_factory=dict)
    body: Any = None
    headers: Mapping[str, Optional[str]] = field(default_factory=dict)


def _env(name: str) -> Optional[str]:
    # Empty values (e.g. `AGDIR_EMAIL_BASE_URL=` in a .env file) count as unset.
    return os.environ.get(name) or None


@dataclass(frozen=True)
class Config:
    api_key: str
    base_url: str
    max_retries: int
    timeout: float

    @classmethod
    def resolve(
        cls,
        api_key: Optional[str],
        base_url: Optional[str],
        max_retries: int,
        timeout: Optional[float],
    ) -> Config:
        key = api_key or _env("AGDIR_EMAIL_API_KEY")
        if not key:
            raise AgdirEmailError(
                "Missing API key: pass api_key=... or set AGDIR_EMAIL_API_KEY. Create one at https://app.agdir.email.",
                "missing_api_key",
            )
        if isinstance(max_retries, bool) or not isinstance(max_retries, int) or max_retries < 0:
            raise ValueError("max_retries must be a non-negative integer.")
        if timeout is not None and timeout <= 0:
            raise ValueError("timeout must be a positive number of seconds.")
        return cls(
            api_key=key,
            base_url=(base_url or _env("AGDIR_EMAIL_BASE_URL") or DEFAULT_BASE_URL).rstrip("/"),
            max_retries=max_retries,
            timeout=DEFAULT_TIMEOUT if timeout is None else timeout,
        )


def backoff_seconds(attempt: int) -> float:
    """Exponential backoff with jitter: ~0.5 s, 1 s, 2 s … capped at 8 s."""
    base = min(8.0, 0.5 * 2.0**attempt)
    return base / 2 + random.random() * (base / 2)


def _retry_after(response: httpx.Response) -> Optional[float]:
    raw = response.headers.get("retry-after")
    if not raw:
        return None
    try:
        return max(0.0, float(raw))
    except ValueError:
        pass
    try:
        when = parsedate_to_datetime(raw)
    except (TypeError, ValueError):
        return None
    return max(0.0, when.timestamp() - time.time())


def to_error(response: httpx.Response) -> AgdirEmailError:
    body: Dict[str, Any] = {}
    try:
        parsed = response.json()
        if isinstance(parsed, dict):
            body = parsed
    except ValueError:
        pass  # Not JSON (e.g. a proxy error page): fall back to the status line.
    message = body.get("message")
    code = body.get("name")
    return AgdirEmailError(
        message if isinstance(message, str) and message else f"Request failed with HTTP {response.status_code}",
        code
        if isinstance(code, str) and code
        else ("internal_server_error" if response.status_code >= 500 else "unknown_error"),
        response.status_code,
        _retry_after(response),
    )


def _transport_error(exc: httpx.TransportError, timeout: float) -> AgdirEmailError:
    if isinstance(exc, httpx.TimeoutException):
        error = AgdirEmailError(f"Request timed out after {timeout:g} s", "timeout")
    else:
        error = AgdirEmailError(f"Could not reach the Agdir Email API: {exc}", "network_error")
    error.__cause__ = exc
    return error


def _should_retry(error: AgdirEmailError, op: Operation[Any]) -> bool:
    # Rate limits are refused before any work is done, so they are always safe to retry.
    return error.code == "rate_limit_exceeded" or (op.retryable and error.status in _RETRY_STATUSES)


def _delay(error: AgdirEmailError, attempt: int) -> float:
    return error.retry_after if error.retry_after is not None else backoff_seconds(attempt)


def _build(config: Config, op: Operation[Any]) -> Dict[str, Any]:
    headers = {
        "Authorization": f"Bearer {config.api_key}",
        "User-Agent": USER_AGENT,
        "Accept": "application/json",
    }
    headers.update({k: v for k, v in op.headers.items() if v is not None})
    request: Dict[str, Any] = {
        "method": op.method,
        "url": config.base_url + op.path,
        "headers": headers,
        "params": {k: str(v) for k, v in op.query.items() if v is not None},
        "timeout": config.timeout,
    }
    if op.body is not None:
        request["json"] = op.body
    return request


def _parse(op: Operation[T], response: httpx.Response) -> T:
    try:
        payload = response.json()
    except ValueError:
        raise AgdirEmailError(
            f"Expected JSON from the Agdir Email API (HTTP {response.status_code})",
            "invalid_response",
            response.status_code,
        ) from None
    return op.parse(payload)


# Indirections so tests can skip the waits.
def _sleep(seconds: float) -> None:
    time.sleep(seconds)


async def _async_sleep(seconds: float) -> None:
    await asyncio.sleep(seconds)


class SyncTransport:
    def __init__(self, config: Config, http_client: Optional[httpx.Client]) -> None:
        self.config = config
        self._owns_client = http_client is None
        self._client = http_client if http_client is not None else httpx.Client()

    def close(self) -> None:
        if self._owns_client:
            self._client.close()

    def call(self, op: Operation[T]) -> T:
        request = _build(self.config, op)
        attempt = 0
        while True:
            can_retry = attempt < self.config.max_retries
            try:
                response = self._client.request(**request)
            except httpx.TransportError as exc:
                # A timed-out POST may have been accepted: only retry it when it is idempotent.
                if can_retry and op.retryable:
                    _sleep(backoff_seconds(attempt))
                    attempt += 1
                    continue
                raise _transport_error(exc, self.config.timeout) from exc
            if response.is_success:
                return _parse(op, response)
            error = to_error(response)
            if not (can_retry and _should_retry(error, op)):
                raise error
            _sleep(_delay(error, attempt))
            attempt += 1


class AsyncTransport:
    def __init__(self, config: Config, http_client: Optional[httpx.AsyncClient]) -> None:
        self.config = config
        self._owns_client = http_client is None
        self._client = http_client if http_client is not None else httpx.AsyncClient()

    async def aclose(self) -> None:
        if self._owns_client:
            await self._client.aclose()

    async def call(self, op: Operation[T]) -> T:
        request = _build(self.config, op)
        attempt = 0
        while True:
            can_retry = attempt < self.config.max_retries
            try:
                response = await self._client.request(**request)
            except httpx.TransportError as exc:
                if can_retry and op.retryable:
                    await _async_sleep(backoff_seconds(attempt))
                    attempt += 1
                    continue
                raise _transport_error(exc, self.config.timeout) from exc
            if response.is_success:
                return _parse(op, response)
            error = to_error(response)
            if not (can_retry and _should_retry(error, op)):
                raise error
            await _async_sleep(_delay(error, attempt))
            attempt += 1
