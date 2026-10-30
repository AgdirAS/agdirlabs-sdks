# Changelog — com.agdirlabs:agdir-email

All notable changes to the Java SDK are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the package uses [Semantic Versioning](https://semver.org).

## [Unreleased]

## [0.1.0] — 2026-10-30

### Added

- `AgdirEmail` client for the Agdir Email API (Java 11+, no runtime dependencies), configured with `AgdirEmail.builder()` or `AgdirEmail.fromEnv()`.
- `emails()`: `send`, `sendBatch` (strict or permissive validation), `get`, `cancel`.
- `domains()`: `create`, `get`, `list`, `listAll`, `verify`, `delete`.
- `webhooks()`: `create`, `get`, `list`, `listAll`, `update`, `delete`, `rotateSecret`, and `deliveries()` (`list`, `listAll`, `get`, `attempts`, `replay`).
- `CompletableFuture` variants of every call (`sendAsync`, `getAsync`, …); cancelling the future stops retries.
- `listAll()` returns a lazily paging `Iterable` with `stream()`.
- `Webhooks.verify` for signed webhook requests (Standard Webhooks, HMAC-SHA256).
- `AgdirEmailException` with `code`, `status` and `retryAfter`; `WebhookVerificationException`.
- Automatic retries for rate limits, server errors and network failures, with generated idempotency keys for sends.
