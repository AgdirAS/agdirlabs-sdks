# Contributing

Thanks for helping improve Agdir's public packages! This repo is maintained by [Agdir Labs](https://agdirlabs.com), part of [Agdir](https://agdir.no).

## Ground rules

- Be kind — see the [Code of Conduct](CODE_OF_CONDUCT.md).
- Security issues go through [SECURITY.md](SECURITY.md), never public issues.
- For anything bigger than a small fix, open an issue first so we can agree on the approach.

## Layout

| Folder | Package | Toolchain |
| --- | --- | --- |
| `packages/*` | npm packages (`@agdirlabs/email`, …) | Node.js ≥ 22.13 + pnpm |
| `python` | PyPI | Python ≥ 3.9 |
| `php` | Packagist | PHP ≥ 8.1 + Composer |
| `go` | Go module | Go ≥ 1.22 |
| `dotnet` | NuGet | .NET 8+ |
| `java` | Maven Central | JDK 11+ (Maven Wrapper included) |
| `ruby` | RubyGems | Ruby ≥ 3.1 |
| `rust` | crates.io | stable Rust |
| `cli` | `agdir-email` binary (Homebrew, Scoop, winget) | Go |
| `openapi` | API description shared by all SDKs | — |

Each folder has its own README, CHANGELOG, tests and `.github/workflows/<lang>.yml`. If you don't have a toolchain installed, every folder documents a `docker run` command that runs its tests in the official image.

### Node.js

```bash
pnpm install
pnpm typecheck && pnpm test && pnpm build
```

## Pull requests

1. Fork, branch from `main`.
2. Add or update tests for every behaviour change; tests must not call real network services (stub `fetch`).
3. Update the package `CHANGELOG.md` under **Unreleased**.
4. Make sure `pnpm typecheck && pnpm test && pnpm build` passes — CI runs it on Node 20, 22 and 24.
5. Use [Conventional Commits](https://www.conventionalcommits.org) for titles, e.g. `fix(email): …`.

By contributing you agree your work is licensed under the [MIT License](LICENSE).

## Releasing (maintainers)

npm packages are tagged `<package dir>@<version>` (e.g. `email@0.2.0`); every other language is tagged `<folder>/v<version>` (e.g. `python/v0.1.0`, `go/v0.1.0` — the form Go requires for a module in a subfolder) and has its own release workflow.

For npm: bump `version` in the package's `package.json`, move **Unreleased** in its `CHANGELOG.md` to the new version, commit, then push a tag `<package dir>@<version>` (e.g. `email@0.1.0`). The release workflow verifies the tag, runs the tests and publishes to npm through [Trusted Publishing](https://docs.npmjs.com/trusted-publishers) (no tokens; provenance is attached automatically). A new package must be published once by hand and then have this repo's `release.yml` added as its trusted publisher on npmjs.com.
