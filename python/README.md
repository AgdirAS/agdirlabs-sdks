# agdirlabs-email

[![PyPI](https://img.shields.io/pypi/v/agdirlabs-email.svg)](https://pypi.org/project/agdirlabs-email/)
[![Python](https://img.shields.io/pypi/pyversions/agdirlabs-email.svg)](https://pypi.org/project/agdirlabs-email/)
[![CI](https://github.com/AgdirAS/agdirlabs-sdks/actions/workflows/python.yml/badge.svg)](https://github.com/AgdirAS/agdirlabs-sdks/actions/workflows/python.yml)

The official Python SDK for [Agdir Email](https://agdir.email) — programmatic SMTP for developers, hosted in the EU. Built by [Agdir Labs](https://agdirlabs.com) at [Agdir](https://agdir.no).

- Sync and async clients (`AgdirEmail`, `AsyncAgdirEmail`) with the same API
- Fully typed (`py.typed`): `TypedDict` inputs, frozen dataclass results
- One dependency ([httpx](https://www.python-httpx.org)); Python 3.9+
- Safe automatic retries with idempotency keys
- Cursor pagination with `for` / `async for`
- Webhook signature verification built in
- Optional Django email backend

## Install

```bash
pip install agdirlabs-email
# with the Django email backend
pip install "agdirlabs-email[django]"
```

## Quick start

Create an API key at [app.agdir.email](https://app.agdir.email) and verify a sending domain, then:

```python
from agdirlabs_email import AgdirEmail

client = AgdirEmail(api_key="re_...")  # or set AGDIR_EMAIL_API_KEY and call AgdirEmail()

result = client.emails.send(
    {
        "from": "Acme <hello@acme.com>",
        "to": "user@example.com",
        "subject": "Welcome to Acme",
        "html": "<p>Thanks for signing up!</p>",
    }
)
print(result.id)
```

Async code uses `AsyncAgdirEmail` — every method is the same, awaited:

```python
from agdirlabs_email import AsyncAgdirEmail

async with AsyncAgdirEmail() as client:
    result = await client.emails.send({...})
```

Clients hold a connection pool: create one and reuse it, and close it with `client.close()` /
`await client.aclose()` or a `with` / `async with` block.

## Emails

Email parameters are plain dicts (typed as `SendEmailParams`) with the API's field names, so `from` works as a key:

```python
from datetime import datetime, timedelta, timezone

# Plain text, several recipients, reply-to, tags, custom headers
client.emails.send(
    {
        "from": "Acme <billing@acme.com>",
        "to": ["ana@example.com", "ben@example.com"],
        "cc": "finance@acme.com",
        "reply_to": "support@acme.com",
        "subject": "Your invoice",
        "text": "Invoice #1042 is ready in your dashboard.",
        "tags": [{"name": "kind", "value": "invoice"}],
        "headers": {"X-Entity-Ref-ID": "inv-1042"},
    }
)

# Schedule for later (a timezone-aware datetime or an ISO 8601 string), then cancel if plans change
sent = client.emails.send({**email, "scheduled_at": datetime.now(timezone.utc) + timedelta(hours=1)})
client.emails.cancel(sent.id)

# Delivery status
email = client.emails.get(sent.id)
email.status  # 'queued' | 'scheduled' | 'sent' | 'delivered' | 'bounced' | 'complained' | …
email.from_, email.to, email.created_at  # datetimes are timezone-aware; cc/bcc/reply_to are always lists

# Up to 100 emails per request (MAX_BATCH_SIZE)
batch = client.emails.send_batch([email1, email2])
[r.id for r in batch.data]

# Accept the valid ones, report the rest
batch = client.emails.send_batch(emails, validation="permissive")
for error in batch.errors:
    print(error.index, error.message)
```

## Domains

```python
domain = client.domains.create({"name": "acme.com"})
for record in domain.records:  # add these to your DNS
    print(record.purpose, record.type, record.name, record.value)

client.domains.verify(domain.id)  # check DNS now instead of waiting
client.domains.get(domain.id).status  # 'pending' → 'verified'

for d in client.domains.list_all():  # fetches further pages as you iterate
    print(d.name, d.status)

page = client.domains.list(limit=10)  # or page by hand
next_page = client.domains.list(limit=10, after=page.next_cursor) if page.has_more else None

client.domains.delete(domain.id)
```

With the async client, `list_all()` is an async iterator: `async for d in client.domains.list_all(): ...`.

## Webhooks

```python
webhook = client.webhooks.create(
    {
        "endpoint": "https://acme.com/webhooks/email",
        "events": ["email.delivered", "email.bounced", "email.complained"],
    }
)
webhook.signing_secret  # 'whsec_…' — store it to verify incoming events

client.webhooks.update(webhook.id, {"status": "disabled"})
client.webhooks.rotate_secret(webhook.id)

# What was sent to your endpoint, and each HTTP attempt
deliveries = client.webhooks.deliveries.list(webhook.id).data
attempts = client.webhooks.deliveries.attempts(webhook.id, deliveries[0].id).data
client.webhooks.deliveries.replay(webhook.id, deliveries[0].id)
```

### Verifying incoming events

Every request to your endpoint is signed ([Standard Webhooks](https://www.standardwebhooks.com)). Verify it against
the **raw** body before trusting it — `verify_webhook` returns the parsed event or raises `WebhookVerificationError`:

```python
import os
from agdirlabs_email import verify_webhook, WebhookVerificationError

SECRET = os.environ["AGDIR_EMAIL_WEBHOOK_SECRET"]

# Django
from django.http import HttpResponse, HttpResponseBadRequest
from django.views.decorators.csrf import csrf_exempt
from django.views.decorators.http import require_POST


@csrf_exempt
@require_POST
def email_webhook(request):
    try:
        event = verify_webhook(request.body, request.headers, SECRET)
    except WebhookVerificationError:
        return HttpResponseBadRequest()
    if event.type == "email.bounced":
        mark_bounced(event.data.to, event.data.bounce)
    return HttpResponse(status=204)


# Flask
@app.post("/webhooks/email")
def email_webhook():
    try:
        event = verify_webhook(request.get_data(), request.headers, SECRET)
    except WebhookVerificationError:
        abort(400)
    ...
    return "", 204


# FastAPI
@app.post("/webhooks/email", status_code=204)
async def email_webhook(request: Request):
    try:
        event = verify_webhook(await request.body(), request.headers, SECRET)
    except WebhookVerificationError:
        raise HTTPException(400)
    ...
```

Messages older than 5 minutes are rejected (`tolerance_seconds=` to change). Failed deliveries are retried for about
18 hours; use `event.data.email_id` to deduplicate.

## Django

Send Django mail (`send_mail`, `EmailMessage`, `EmailMultiAlternatives`, password-reset emails, …) through Agdir Email:

```python
# settings.py
EMAIL_BACKEND = "agdirlabs_email.django.EmailBackend"
AGDIR_EMAIL_API_KEY = os.environ["AGDIR_EMAIL_API_KEY"]  # optional: falls back to the environment variable
DEFAULT_FROM_EMAIL = "Acme <hello@acme.com>"
# Optional: AGDIR_EMAIL_BASE_URL, AGDIR_EMAIL_MAX_RETRIES (default 2), AGDIR_EMAIL_TIMEOUT (seconds, default 30)
```

```python
from django.core.mail import EmailMultiAlternatives

message = EmailMultiAlternatives(
    "Your invoice",
    "Plain-text body",
    to=["ana@example.com"],
    cc=["finance@acme.com"],
    reply_to=["support@acme.com"],
    headers={"X-Entity-Ref-ID": "inv-1042"},
)
message.attach_alternative("<p>HTML body</p>", "text/html")
message.tags = [{"name": "kind", "value": "invoice"}]  # optional API extras: tags, scheduled_at
message.send()
message.agdir_email_id  # the API's email id
```

`from`, `to`, `cc`, `bcc`, `reply_to`, `subject`, the body (plain, or HTML with `content_subtype = "html"`), a
`text/html` alternative and extra headers are mapped. Display-only `To`/`Cc` header overrides are ignored, and messages
with attachments raise `AttachmentsNotSupportedError` (the API doesn't accept attachments yet). API failures raise
`AgdirEmailError` unless `fail_silently=True`.

## Errors

Every failed call raises an `AgdirEmailError`:

```python
from agdirlabs_email import AgdirEmailError

try:
    client.emails.send(email)
except AgdirEmailError as err:
    err.code  # 'validation_error' | 'daily_quota_exceeded' | 'rate_limit_exceeded' | 'network_error' | 'timeout' | …
    err.status  # HTTP status, 0 if no response was received
    err.message  # human-readable explanation
    err.retry_after  # seconds, when the API sent retry-after
```

| `code` | Status | Meaning |
| --- | --- | --- |
| `missing_api_key` | 401 / 0 | No API key (also raised by the constructor) |
| `invalid_api_key` | 403 | Unknown or revoked key |
| `restricted_api_key` | 401 | A sending-only key used on a read endpoint |
| `validation_error` | 422 / 403 | Invalid input, unverified domain, paused team, domain limit |
| `not_found` | 404 | No such email, domain or webhook |
| `invalid_idempotent_request` / `concurrent_idempotent_requests` | 409 | Idempotency key reused with a different body / in flight |
| `daily_quota_exceeded` / `monthly_quota_exceeded` | 429 | Plan limit reached (not retried) |
| `rate_limit_exceeded` | 429 | Too many requests per second (retried automatically) |
| `internal_server_error` | 5xx | Server error |
| `network_error` / `timeout` | 0 | No response |

## Retries and idempotency

Rate limits (`429 rate_limit_exceeded`), server errors (408/500/502/503/504) and network failures are retried up to
`max_retries` times (default 2) with exponential backoff, honouring `retry-after`. Requests that could have side
effects are only retried when that's safe: sends carry an idempotency key, so a retry can never deliver an email twice.
The SDK generates one per call, or pass your own to make sends safe across process restarts too:

```python
client.emails.send(email, idempotency_key=f"welcome/{user.id}")
```

Quota errors are never retried.

## Configuration

```python
AgdirEmail(
    api_key="re_...",  # default: AGDIR_EMAIL_API_KEY
    base_url="https://api.agdir.email",  # default: AGDIR_EMAIL_BASE_URL, else this
    max_retries=2,
    timeout=30.0,  # seconds, per attempt
    http_client=httpx.Client(proxy="http://proxy:8080"),  # optional; you close it
)
```

Empty environment variables count as unset. With `AsyncAgdirEmail`, pass an `httpx.AsyncClient`. Async calls are
cancelled the usual way (`asyncio.timeout`, task cancellation).

## Development

```bash
cd python
pip install -e . --group dev     # pip ≥ 25.1
ruff check . && ruff format --check . && mypy && pytest
```

Without a local Python, run the same in Docker:

```bash
docker run --rm -v "$PWD":/src -w /src python:3.13-slim \
  sh -c 'pip install -q -e . --group dev && ruff check . && mypy && pytest'
```

`tests/test_live.py` runs against a real API only when `AGDIR_EMAIL_LIVE_BASE_URL` and `AGDIR_EMAIL_LIVE_API_KEY`
(and optionally `AGDIR_EMAIL_LIVE_FROM`, an address on a verified domain) are set. Never point it at production.

## Releasing

Releases are published to PyPI from GitHub Actions with
[Trusted Publishing](https://docs.pypi.org/trusted-publishers/) — no API tokens are stored anywhere.

1. Bump `__version__` in `src/agdirlabs_email/_version.py` and move **Unreleased** in `CHANGELOG.md` to the new version.
2. Commit, then push a tag `python/v<version>` (e.g. `python/v0.1.0`). `.github/workflows/python-release.yml` checks
   that the tag matches `__version__`, runs the tests, builds the sdist and wheel with `python -m build` and publishes.

One-time setup (maintainers): on [pypi.org](https://pypi.org/manage/account/publishing/) add a *pending* trusted
publisher for project `agdirlabs-email` — owner `AgdirAS`, repository `agdirlabs-sdks`, workflow
`python-release.yml`, environment `pypi` — and create a `pypi` environment in the GitHub repository settings
(optionally with required reviewers).

## Contributing

Issues and pull requests are welcome — see [CONTRIBUTING.md](https://github.com/AgdirAS/agdirlabs-sdks/blob/main/CONTRIBUTING.md). Changes are listed in
[CHANGELOG.md](https://github.com/AgdirAS/agdirlabs-sdks/blob/main/python/CHANGELOG.md).

## License

[MIT](https://github.com/AgdirAS/agdirlabs-sdks/blob/main/python/LICENSE) © [Agdir AS](https://agdir.no)
