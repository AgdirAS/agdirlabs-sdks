# Changelog — agdir-email CLI

All notable changes to the CLI are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the CLI uses [Semantic Versioning](https://semver.org). Releases are tagged `cli/vX.Y.Z`.

## [Unreleased]

## [0.1.0] — 2026-10-30

### Added

- `send` with `--from/--to/--cc/--bcc/--reply-to`, HTML and text bodies from flags, files or stdin, `--tag`, `--header`, `--schedule` (RFC 3339 or relative, e.g. "in 1h") and `--idempotency-key`.
- `emails get` and `emails cancel`; `batch send <file.json>` (strict or `--permissive`).
- `domains add|list|get|verify|delete` (domains by ID or name) and `domains check`, which looks up every required DNS record in live DNS and shows ✓/✗ against the expected value.
- `webhooks create|list|get|update|delete|rotate-secret`, `webhooks deliveries list|get|attempts|replay`, and `webhooks verify` to check signatures of captured requests.
- `login` (key validated, stored with 0600 permissions, named profiles), `logout`, `whoami`/`status`, `profiles list|use`.
- `--json` output on every command, `--quiet`, exit codes 0/1/2/3, `completion bash|zsh|fish|powershell`, `version`.
- Release builds for macOS, Linux and Windows (amd64, arm64), Homebrew cask, Scoop manifest, winget manifest and deb/rpm/apk packages.
