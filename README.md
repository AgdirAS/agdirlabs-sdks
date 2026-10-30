# agdirlabs-sdks

Official SDKs and tools for [Agdir Email](https://agdir.email) — programmatic SMTP for developers, hosted in the EU. From [Agdir Labs](https://agdirlabs.com), the developer-tools arm of [Agdir](https://agdir.no).

[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

## Packages

| Language | Package | Folder | Status |
| --- | --- | --- | --- |
| Node.js / TypeScript | [`@agdirlabs/email`](https://www.npmjs.com/package/@agdirlabs/email) | [`packages/email`](packages/email) | [![npm](https://img.shields.io/npm/v/@agdirlabs/email.svg)](https://www.npmjs.com/package/@agdirlabs/email) |
| Node.js — Nodemailer | `@agdirlabs/nodemailer-transport` | [`packages/nodemailer-transport`](packages/nodemailer-transport) | in development |
| Python | `agdirlabs-email` (PyPI) | [`python`](python) | in development |
| PHP / Laravel | `agdirlabs/email` (Packagist) | [`php`](php) | in development |
| Go | `github.com/AgdirAS/agdirlabs-sdks/go` | [`go`](go) | in development |
| .NET | `AgdirLabs.Email` (NuGet) | [`dotnet`](dotnet) | in development |
| Java / Kotlin | `com.agdirlabs:agdir-email` (Maven Central) | [`java`](java) | in development |
| Ruby | `agdirlabs-email` (RubyGems) | [`ruby`](ruby) | in development |
| Rust | `agdirlabs-email` (crates.io) | [`rust`](rust) | in development |
| CLI | `agdir-email` (Homebrew, Scoop, winget, deb/rpm, `go install`) | [`cli`](cli) | in development |

All SDKs implement the same API, described by the [OpenAPI spec](openapi/agdir-email.yaml), and share the same conventions: a client preconfigured for `https://api.agdir.email`, the `AGDIR_EMAIL_API_KEY` / `AGDIR_EMAIL_BASE_URL` environment variables, typed errors with the API's error codes, safe retries with idempotency keys, cursor pagination, and webhook signature verification.

## Development

Each language lives in its own folder with its own README, CHANGELOG, tests, CI workflow (`.github/workflows/<lang>.yml`) and release workflow. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Community

- Bugs and feature requests: [GitHub issues](https://github.com/AgdirAS/agdirlabs-sdks/issues)
- Security: [SECURITY.md](SECURITY.md)
- Code of Conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)

## License

[MIT](LICENSE) © [Agdir AS](https://agdir.no)
