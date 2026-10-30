from __future__ import annotations

from typing import Optional

__all__ = ["AgdirEmailError", "WebhookVerificationError"]


class AgdirEmailError(Exception):
    """Raised by every client method when a request fails.

    ``code`` is the API's machine-readable error name (``validation_error``, ``rate_limit_exceeded``,
    ``daily_quota_exceeded``, …) or a client-side ``network_error`` / ``timeout``.
    """

    code: str
    """Machine-readable error code, e.g. ``validation_error``."""
    status: int
    """HTTP status, or 0 when no response was received."""
    message: str
    """Human-readable explanation."""
    retry_after: Optional[float]
    """Seconds the API asked us to wait (from ``retry-after``), if any."""

    def __init__(self, message: str, code: str, status: int = 0, retry_after: Optional[float] = None) -> None:
        super().__init__(message)
        self.message = message
        self.code = code
        self.status = status
        self.retry_after = retry_after

    def __repr__(self) -> str:
        return f"AgdirEmailError(code={self.code!r}, status={self.status}, message={self.message!r})"


class WebhookVerificationError(Exception):
    """Raised by :func:`verify_webhook` when a payload's signature or timestamp is invalid."""
