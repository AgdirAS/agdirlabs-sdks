# Changelog — agdirlabs-email (Ruby)

All notable changes to this gem are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the gem uses [Semantic Versioning](https://semver.org).

## [Unreleased]

## [0.1.0] — 2026-10-30

### Added

- `AgdirLabs::Email::Client` for the Agdir Email API, with no runtime dependencies (Ruby 3.1+).
- `emails`: `send`, `send_batch` (strict or permissive validation), `get`, `cancel`.
- `domains`: `create`, `get`, `list`, `list_all`, `verify`, `delete`.
- `webhooks`: `create`, `get`, `list`, `list_all`, `update`, `delete`, `rotate_secret`, and `deliveries` (`list`, `list_all`, `get`, `attempts`, `replay`).
- Frozen response objects with `Time` timestamps; `list_all` returns a lazily paging `Enumerator`.
- `AgdirLabs::Email::Webhook.verify` for signed webhook requests (Standard Webhooks).
- `AgdirLabs::Email::Error` with `code`, `status` and `retry_after`, and subclasses per error family.
- Automatic retries for rate limits, server errors and network failures, with generated idempotency keys for sends.
- `:agdir_email` ActionMailer delivery method (Railtie) and `AgdirLabs::Email::DeliveryMethod` for the `mail` gem.
- RBS signatures.
