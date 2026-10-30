# Changelog — AgdirLabs.Email (.NET)

All notable changes to the .NET packages (`AgdirLabs.Email`, `AgdirLabs.Email.Extensions.DependencyInjection`) are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the packages use [Semantic Versioning](https://semver.org).

## [Unreleased]

## [0.1.0] — 2026-10-30

### Added

- `AgdirEmailClient` for the Agdir Email API, targeting .NET 8 and .NET Standard 2.0, with no dependencies beyond System.Text.Json on .NET Standard.
- `Emails`: `SendAsync`, `SendBatchAsync` (strict or permissive validation), `GetAsync`, `CancelAsync`.
- `Domains`: `CreateAsync`, `GetAsync`, `ListAsync`, `ListAllAsync`, `VerifyAsync`, `DeleteAsync`.
- `Webhooks`: `CreateAsync`, `GetAsync`, `ListAsync`, `ListAllAsync`, `UpdateAsync`, `DeleteAsync`, `RotateSecretAsync`, and `Deliveries` (`ListAsync`, `ListAllAsync`, `GetAsync`, `ListAttemptsAsync`, `ReplayAsync`).
- `WebhookVerifier.Verify` for signed webhook requests (`IDictionary<string, string>`, `HttpHeaders` or a header callback).
- `AgdirEmailException` with `Code`, `StatusCode` and `RetryAfter`; `AgdirEmailErrorCodes`.
- Automatic retries for rate limits, server errors and network failures, with generated idempotency keys for sends.
- Interfaces (`IAgdirEmailClient`, `IEmailsResource`, …) for mocking; reflection-free JSON (trimming/AOT-friendly on .NET 8).
- `AgdirLabs.Email.Extensions.DependencyInjection`: `services.AddAgdirEmail()` with `IHttpClientFactory` and the `AgdirEmail` configuration section.
