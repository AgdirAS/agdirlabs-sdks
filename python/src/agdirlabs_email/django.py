"""Django email backend that sends through the Agdir Email API.

.. code-block:: python

    # settings.py — Django 6.1+ (MAILERS)
    MAILERS = {"default": {"BACKEND": "agdirlabs_email.django.EmailBackend", "OPTIONS": {"api_key": "re_..."}}}

    # settings.py — earlier Django versions
    EMAIL_BACKEND = "agdirlabs_email.django.EmailBackend"
    AGDIR_EMAIL_API_KEY = "re_..."           # or the AGDIR_EMAIL_API_KEY environment variable

Install with ``pip install "agdirlabs-email[django]"``.
"""

from __future__ import annotations

from typing import Any, Dict, List, Optional, Sequence

from django.conf import settings
from django.core.mail.backends.base import BaseEmailBackend

from ._client import AgdirEmail
from ._errors import AgdirEmailError
from ._http import DEFAULT_MAX_RETRIES
from ._types import SendEmailParams

__all__ = ["AttachmentsNotSupportedError", "EmailBackend"]

# Set by the API from the message itself; Django's display overrides for these can't be honoured separately.
_RESERVED_HEADERS = frozenset({"from", "to", "cc", "bcc", "reply-to", "subject"})


class AttachmentsNotSupportedError(ValueError):
    """The Agdir Email API does not accept attachments yet."""


class EmailBackend(BaseEmailBackend):
    """Sends Django ``EmailMessage`` / ``EmailMultiAlternatives`` through Agdir Email.

    Options come from ``MAILERS[alias]["OPTIONS"]`` (Django 6.1+) or ``get_connection()`` keyword arguments
    (``api_key``, ``base_url``, ``max_retries``, ``timeout``), falling back to these settings:

    - ``AGDIR_EMAIL_API_KEY`` — defaults to the ``AGDIR_EMAIL_API_KEY`` environment variable
    - ``AGDIR_EMAIL_BASE_URL`` — defaults to ``AGDIR_EMAIL_BASE_URL``, else ``https://api.agdir.email``
    - ``AGDIR_EMAIL_MAX_RETRIES`` (default 2), ``AGDIR_EMAIL_TIMEOUT`` (seconds, default 30)

    After a successful send, each message gets an ``agdir_email_id`` attribute with the API's email id. Set
    ``message.tags = [{"name": ..., "value": ...}]`` or ``message.scheduled_at`` to use those API features.
    """

    def __init__(
        self,
        fail_silently: bool = False,
        *,
        api_key: Optional[str] = None,
        base_url: Optional[str] = None,
        max_retries: Optional[int] = None,
        timeout: Optional[float] = None,
        client: Optional[AgdirEmail] = None,
        **kwargs: Any,
    ) -> None:
        # `fail_silently` is handled here: newer Django deprecates passing it to BaseEmailBackend.
        super().__init__(**kwargs)
        self.fail_silently = fail_silently
        self._api_key = api_key or getattr(settings, "AGDIR_EMAIL_API_KEY", None)
        self._base_url = base_url or getattr(settings, "AGDIR_EMAIL_BASE_URL", None)
        self._max_retries = (
            max_retries
            if max_retries is not None
            else getattr(settings, "AGDIR_EMAIL_MAX_RETRIES", DEFAULT_MAX_RETRIES)
        )
        self._timeout = timeout if timeout is not None else getattr(settings, "AGDIR_EMAIL_TIMEOUT", None)
        self._client = client
        self._owns_client = client is None

    def open(self) -> bool:
        if self._client is not None:
            return False
        self._client = AgdirEmail(
            self._api_key, base_url=self._base_url, max_retries=self._max_retries, timeout=self._timeout
        )
        self._owns_client = True
        return True

    def close(self) -> None:
        if self._client is not None and self._owns_client:
            self._client.close()
            self._client = None

    def send_messages(self, email_messages: Sequence[Any]) -> int:
        if not email_messages:
            return 0
        try:
            opened = self.open()
        except AgdirEmailError:
            if not self.fail_silently:
                raise
            return 0
        assert self._client is not None
        sent = 0
        try:
            for message in email_messages:
                params = message_to_params(message)
                if params is None:
                    continue
                try:
                    result = self._client.emails.send(params)
                except AgdirEmailError:
                    if not self.fail_silently:
                        raise
                    continue
                message.agdir_email_id = result.id
                sent += 1
        finally:
            if opened:
                self.close()
        return sent


def message_to_params(message: Any) -> Optional[SendEmailParams]:
    """Maps a Django ``EmailMessage`` to the API's send parameters (``None`` if it has no recipients)."""
    if not message.recipients():
        return None
    if message.attachments:
        raise AttachmentsNotSupportedError(
            "Agdir Email does not support attachments yet: link to the file instead, or remove the attachment."
        )

    extra: Dict[str, str] = {str(k): str(v) for k, v in (message.extra_headers or {}).items()}
    lowered = {k.lower(): v for k, v in extra.items()}

    params: SendEmailParams = {
        "from": lowered.get("from") or message.from_email or settings.DEFAULT_FROM_EMAIL,
        "to": list(message.to),
        "subject": str(message.subject),
    }
    if message.cc:
        params["cc"] = list(message.cc)
    if message.bcc:
        params["bcc"] = list(message.bcc)
    reply_to: List[str] = list(message.reply_to or [])
    if not reply_to and lowered.get("reply-to"):
        reply_to = [addr.strip() for addr in lowered["reply-to"].split(",") if addr.strip()]
    if reply_to:
        params["reply_to"] = reply_to

    body = str(message.body or "")
    if getattr(message, "content_subtype", "plain") == "html":
        params["html"] = body
    elif body:
        params["text"] = body
    for alternative in getattr(message, "alternatives", None) or []:
        content, mimetype = alternative[0], alternative[1]
        if mimetype == "text/html" and "html" not in params:
            params["html"] = str(content)
        elif mimetype == "text/plain" and "text" not in params:
            params["text"] = str(content)
        else:
            raise ValueError(f"Agdir Email supports only text/plain and text/html bodies, not {mimetype}")
    if "html" not in params and "text" not in params:
        params["text"] = ""

    headers = {k: v for k, v in extra.items() if k.lower() not in _RESERVED_HEADERS}
    if headers:
        params["headers"] = headers
    tags = getattr(message, "tags", None)
    if tags:
        params["tags"] = list(tags)
    scheduled_at = getattr(message, "scheduled_at", None)
    if scheduled_at:
        params["scheduled_at"] = scheduled_at
    return params
