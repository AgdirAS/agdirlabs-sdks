# Changelog — agdirlabs-email (Rust)

All notable changes to this crate are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the crate uses [Semantic Versioning](https://semver.org).

## [Unreleased]

## [0.1.0] — 2026-10-30

### Added

- `AgdirEmail` async client for the Agdir Email API (Tokio + `reqwest`, rustls by default, optional `native-tls`), configured with `new`, `from_env` or `builder()` (API key, base URL, max retries, per-attempt timeout, custom `reqwest::Client`).
- `emails()`: `send`, `send_with`, `send_batch`, `send_batch_with` (strict or permissive validation), `get`, `cancel`, with builder-style `SendEmail`.
- `domains()`: `create`, `get`, `list`, `list_all`, `verify`, `delete`.
- `webhooks()`: `create`, `get`, `list`, `list_all`, `update`, `delete`, `rotate_secret`, and `deliveries()` (`list`, `list_all`, `get`, `attempts`, `replay`).
- Cursor pagination: `Page<T>` with `next_cursor`, and `list_all` streams (`impl Stream<Item = Result<T>>`).
- `verify_webhook` / `verify_webhook_with` for signed webhook requests (Standard Webhooks, constant-time comparison), working with `http::HeaderMap`, maps and header pairs; `sign_webhook` for tests.
- `Error` with `Api { code, status, message, retry_after }`, `Network`, `Timeout`, `Decode` and `Config` variants, and a non-exhaustive `ErrorCode`.
- Automatic retries for rate limits, server errors and network failures, with generated idempotency keys for sends.
- `blocking` feature: a synchronous client in `agdirlabs_email::blocking`.
- MSRV: Rust 1.75.
