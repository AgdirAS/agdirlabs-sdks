# Changelog — agdirlabs-email (Python)

All notable changes to this package are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the package uses [Semantic Versioning](https://semver.org).

## [Unreleased]

## [0.1.0] — 2026-10-30

### Added

- `AgdirEmail` (sync) and `AsyncAgdirEmail` (async) clients for the Agdir Email API, built on `httpx`.
- `emails`: `send`, `send_batch` (strict or permissive validation), `get`, `cancel`.
- `domains`: `create`, `get`, `list`, `list_all`, `verify`, `delete`.
- `webhooks`: `create`, `get`, `list`, `list_all`, `update`, `delete`, `rotate_secret`, and `deliveries` (`list`, `list_all`, `get`, `attempts`, `replay`).
- `verify_webhook` for signed webhook requests (Standard Webhooks), returning a typed `WebhookEvent`.
- `AgdirEmailError` with `code`, `status`, `message` and `retry_after`; `WebhookVerificationError`.
- Automatic retries for rate limits, server errors and network failures, with generated idempotency keys for sends.
- Django email backend `agdirlabs_email.django.EmailBackend` (`pip install "agdirlabs-email[django]"`).
- Type hints throughout (`py.typed`).
