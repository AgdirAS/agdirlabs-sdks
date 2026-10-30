# Changelog — agdirlabs/email

All notable changes to this package are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the package uses [Semantic Versioning](https://semver.org).

## [Unreleased]

## [0.1.0] — 2026-10-30

### Added

- `AgdirEmail` client for the Agdir Email API over any PSR-18 HTTP client (Guzzle, Symfony HttpClient, … via `php-http/discovery`). PHP 8.1+.
- `emails`: `send`, `sendBatch` (strict or permissive validation), `get`, `cancel`.
- `domains`: `create`, `get`, `list`, `listAll`, `verify`, `delete`.
- `webhooks`: `create`, `get`, `list`, `listAll`, `update`, `delete`, `rotateSecret`, and `deliveries` (`list`, `listAll`, `get`, `attempts`, `replay`).
- Typed, readonly result objects and enums (`EmailStatus`, `DomainStatus`, `WebhookEventType`, …); `listAll()` returns a lazy `Generator`.
- `Webhook::verify()` for signed webhook requests (Standard Webhooks).
- `AgdirEmailException` with `errorCode`, `status` and `retryAfter`.
- Automatic retries for rate limits, server errors and network failures, with generated idempotency keys for sends.
- Symfony Mailer transport (`agdir+api://KEY@default`) and Laravel integration: auto-discovered service provider, `AgdirEmail` facade, `config/agdir-email.php` and the `agdir` mail transport (`MAIL_MAILER=agdir`).
