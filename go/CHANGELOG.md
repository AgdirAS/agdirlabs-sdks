# Changelog — Agdir Email Go SDK

All notable changes to this module are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the module uses [Semantic Versioning](https://semver.org). Releases are tagged `go/vX.Y.Z`.

## [Unreleased]

## [0.1.0] — 2026-10-30

### Added

- `NewClient` for the Agdir Email API, standard library only (Go 1.23+), with `WithAPIKey`, `WithBaseURL`, `WithHTTPClient`, `WithMaxRetries` and `WithTimeout`; reads `AGDIR_EMAIL_API_KEY` / `AGDIR_EMAIL_BASE_URL`.
- `Emails`: `Send`, `SendBatch` (strict or permissive validation), `Get`, `Cancel`.
- `Domains`: `Create`, `Get`, `List`, `ListAll`, `Verify`, `Delete`.
- `Webhooks`: `Create`, `Get`, `List`, `ListAll`, `Update`, `Delete`, `RotateSecret`, and `Deliveries` (`List`, `ListAll`, `Get`, `Attempts`, `Replay`).
- `ListAll` iterators (`iter.Seq2`) for cursor pagination.
- `VerifyWebhook` and `WebhookHandler` for signed webhook requests.
- `*Error` with `Code`, `Status`, `Message` and `RetryAfter`, and exported error code constants.
- Automatic retries for rate limits, server errors and network failures, with generated idempotency keys for sends.
