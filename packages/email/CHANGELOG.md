# Changelog — @agdirlabs/email

All notable changes to this package are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the package uses [Semantic Versioning](https://semver.org).

## [Unreleased]

## [0.1.1] — 2026-10-30

### Changed

- Published from GitHub Actions via npm Trusted Publishing, with build provenance.

## [0.1.0] — 2026-10-30

### Added

- `AgdirEmail` client for the Agdir Email API, with no runtime dependencies.
- `emails`: `send`, `sendBatch` (strict or permissive validation), `get`, `cancel`.
- `domains`: `create`, `get`, `list`, `listAll`, `verify`, `delete`.
- `webhooks`: `create`, `get`, `list`, `listAll`, `update`, `delete`, `rotateSecret`, and `deliveries` (`list`, `listAll`, `get`, `attempts`, `replay`).
- `verifyWebhook` for signed webhook requests (Web Crypto; Node.js, Bun, Deno, edge).
- `AgdirEmailError` with `code`, `status` and `retryAfter`.
- Automatic retries for rate limits, server errors and network failures, with generated idempotency keys for sends.
