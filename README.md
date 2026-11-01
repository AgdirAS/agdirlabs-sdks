# agdirlabs-sdks

Official SDKs and tools for [Agdir Email](https://agdir.email) — programmatic SMTP for developers, hosted in the EU. From [Agdir Labs](https://agdirlabs.com), the developer-tools arm of [Agdir](https://agdir.no).

[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

## Packages

| Language | Package | Folder | Status |
| --- | --- | --- | --- |
| Node.js / TypeScript | [`@agdirlabs/email`](https://www.npmjs.com/package/@agdirlabs/email) | [`packages/email`](packages/email) | [![npm](https://img.shields.io/npm/v/@agdirlabs/email.svg)](https://www.npmjs.com/package/@agdirlabs/email) |
| Node.js — Nodemailer | `@agdirlabs/nodemailer-transport` | [`packages/nodemailer-transport`](packages/nodemailer-transport) | coming soon |
| Python | [`agdirlabs-email`](https://pypi.org/project/agdirlabs-email/) (PyPI) | [`python`](python) | [![PyPI](https://img.shields.io/pypi/v/agdirlabs-email.svg)](https://pypi.org/project/agdirlabs-email/) |
| PHP / Laravel | [`agdirlabs/email`](https://packagist.org/packages/agdirlabs/email) (Packagist) | [`php`](php) | [![Packagist](https://img.shields.io/packagist/v/agdirlabs/email.svg)](https://packagist.org/packages/agdirlabs/email) |
| Go | [`github.com/AgdirAS/agdirlabs-sdks/go`](https://pkg.go.dev/github.com/AgdirAS/agdirlabs-sdks/go) | [`go`](go) | [![Go Reference](https://pkg.go.dev/badge/github.com/AgdirAS/agdirlabs-sdks/go.svg)](https://pkg.go.dev/github.com/AgdirAS/agdirlabs-sdks/go) |
| .NET | [`AgdirLabs.Email`](https://www.nuget.org/packages/AgdirLabs.Email) (NuGet) | [`dotnet`](dotnet) | [![NuGet](https://img.shields.io/nuget/v/AgdirLabs.Email.svg)](https://www.nuget.org/packages/AgdirLabs.Email) |
| Java / Kotlin | `com.agdirlabs:agdir-email` (Maven Central) | [`java`](java) | coming soon |
| Ruby | [`agdirlabs-email`](https://rubygems.org/gems/agdirlabs-email) (RubyGems) | [`ruby`](ruby) | [![Gem](https://img.shields.io/gem/v/agdirlabs-email.svg)](https://rubygems.org/gems/agdirlabs-email) |
| Rust | [`agdirlabs-email`](https://crates.io/crates/agdirlabs-email) (crates.io) | [`rust`](rust) | [![crates.io](https://img.shields.io/crates/v/agdirlabs-email.svg)](https://crates.io/crates/agdirlabs-email) |
| CLI | `agdir-email` — `brew install agdiras/tap/agdir-email`, Scoop, deb/rpm, [binaries](https://github.com/AgdirAS/agdirlabs-sdks/releases/tag/cli/v0.1.0) | [`cli`](cli) | 0.1.0 |

All SDKs implement the same API, described by the [OpenAPI spec](openapi/agdir-email.yaml), and share the same conventions: a client preconfigured for `https://api.agdir.email`, the `AGDIR_EMAIL_API_KEY` / `AGDIR_EMAIL_BASE_URL` environment variables, typed errors with the API's error codes, safe retries with idempotency keys, cursor pagination, and webhook signature verification.

## Development

Each language lives in its own folder with its own README, CHANGELOG, tests, CI workflow (`.github/workflows/<lang>.yml`) and release workflow. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Community

- Bugs and feature requests: [GitHub issues](https://github.com/AgdirAS/agdirlabs-sdks/issues)
- Security: [SECURITY.md](SECURITY.md)
- Code of Conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)

## License

[MIT](LICENSE) © [Agdir AS](https://agdir.no)
