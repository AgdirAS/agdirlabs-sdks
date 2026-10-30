# Changelog — @agdirlabs/nodemailer-transport

All notable changes to this package are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the package uses [Semantic Versioning](https://semver.org).

## [Unreleased]

## [0.1.0] — 2026-10-30

### Added

- `agdirTransport()` / `AgdirTransport`: a Nodemailer transport that sends through the Agdir Email API with `@agdirlabs/email`.
- Accepts every Nodemailer address form (strings, `{ name, address }`, arrays, groups) and `html`/`text` as strings, Buffers, streams, files or URLs.
- Custom headers (object or array form), `inReplyTo`, `references`, `list` and `priority`.
- Tags, scheduled sending and idempotency keys via message options (`tags`, `scheduledAt`, `idempotencyKey`) or `X-Agdir-*` headers.
- `sendMail` info with the Agdir email id as `messageId`; API failures reported as `AgdirEmailError` (`code`, `status`, `responseCode`).
- Clear errors for features the API does not support yet (attachments, `raw`, `alternatives`, `icalEvent`, custom `envelope`).
