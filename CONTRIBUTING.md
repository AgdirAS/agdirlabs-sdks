# Contributing

Thanks for helping improve Agdir's public packages! This repo is maintained by [Agdir Labs](https://agdirlabs.com), part of [Agdir](https://agdir.no).

## Ground rules

- Be kind — see the [Code of Conduct](CODE_OF_CONDUCT.md).
- Security issues go through [SECURITY.md](SECURITY.md), never public issues.
- For anything bigger than a small fix, open an issue first so we can agree on the approach.

## Setup

Requires Node.js ≥ 20 and [pnpm](https://pnpm.io) (`corepack enable` picks the version from `package.json`).

```bash
pnpm install
pnpm typecheck
pnpm test
pnpm build
```

## Layout

| Path | Package |
| --- | --- |
| `packages/email` | [`@agdirlabs/email`](packages/email) — Node.js SDK for Agdir Email |

Each package has its own `README.md`, `CHANGELOG.md`, tests (`src/*.test.ts`, [Vitest](https://vitest.dev)) and build ([tsup](https://tsup.egoist.dev)).

## Pull requests

1. Fork, branch from `main`.
2. Add or update tests for every behaviour change; tests must not call real network services (stub `fetch`).
3. Update the package `CHANGELOG.md` under **Unreleased**.
4. Make sure `pnpm typecheck && pnpm test && pnpm build` passes — CI runs it on Node 20, 22 and 24.
5. Use [Conventional Commits](https://www.conventionalcommits.org) for titles, e.g. `fix(email): …`.

By contributing you agree your work is licensed under the [MIT License](LICENSE).

## Releasing (maintainers)

Bump `version` in the package's `package.json`, move **Unreleased** in its `CHANGELOG.md` to the new version, commit, then push a tag `<package dir>@<version>` (e.g. `email@0.1.0`). The release workflow verifies the tag, runs the tests and publishes to npm through [Trusted Publishing](https://docs.npmjs.com/trusted-publishers) (no tokens; provenance is attached automatically). A new package must be published once by hand and then have this repo's `release.yml` added as its trusted publisher on npmjs.com.
