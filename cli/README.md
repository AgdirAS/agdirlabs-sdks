# agdir-email — the Agdir Email CLI

[![CI](https://github.com/AgdirAS/agdirlabs-sdks/actions/workflows/cli.yml/badge.svg)](https://github.com/AgdirAS/agdirlabs-sdks/actions/workflows/cli.yml)

The official command-line interface for [Agdir Email](https://agdir.email) — programmatic SMTP for developers, hosted in the EU. Built by [Agdir Labs](https://agdirlabs.com) at [Agdir](https://agdir.no) on the [Go SDK](../go).

- Send and schedule email from your terminal or CI
- Add sending domains and **check their DNS records live** (`domains check`)
- Manage webhooks, inspect and replay deliveries, verify signatures of captured requests
- `--json` on every command, stable exit codes, shell completions
- Single static binary for macOS, Linux and Windows

Keywords: email, smtp, transactional-email, agdir.

## Install

**Homebrew** (macOS, Linux)

```bash
brew install agdiras/tap/agdir-email
```

**Scoop** (Windows)

```powershell
scoop bucket add agdiras https://github.com/AgdirAS/scoop-bucket
scoop install agdir-email
```

**winget** (Windows)

```powershell
winget install AgdirLabs.AgdirEmail
```

**Debian/Ubuntu, Fedora/RHEL, Alpine** — download the `.deb`, `.rpm` or `.apk` from the [releases page](https://github.com/AgdirAS/agdirlabs-sdks/releases?q=cli%2Fv) and install it with `sudo dpkg -i`, `sudo rpm -i` or `sudo apk add --allow-untrusted`.

**Go** (1.23+)

```bash
go install github.com/AgdirAS/agdirlabs-sdks/cli/cmd/agdir-email@latest
```

**Binary download** — every [release](https://github.com/AgdirAS/agdirlabs-sdks/releases?q=cli%2Fv) has archives for macOS (arm64, amd64), Linux (amd64, arm64) and Windows (amd64, arm64), plus `checksums.txt`:

```bash
curl -sSL https://github.com/AgdirAS/agdirlabs-sdks/releases/download/cli/v0.1.0/agdir-email_0.1.0_linux_amd64.tar.gz | tar xz agdir-email
sudo mv agdir-email /usr/local/bin/
```

Archives also contain shell completion scripts (`completions/`).

## Quick start

Create an API key at [app.agdir.email](https://app.agdir.email), then:

```bash
# 1. Store the key (checked against the API, saved with 0600 permissions)
agdir-email login

# 2. Add your sending domain: prints the DNS records to publish
agdir-email domains add acme.com

# 3. After adding the records at your DNS provider, check them in live DNS
agdir-email domains check acme.com
#    ✓  DKIM     TXT  agdir._domainkey.acme.com  ok
#    ✓  SPF      MX   send.acme.com              ok
#    ✗  SPF      TXT  send.acme.com              SPF record does not contain include:amazonses.com; …
#    !  DMARC (recommended) …

# 4. Verify, then send
agdir-email domains verify acme.com
agdir-email send --from "Acme <hi@acme.com>" --to you@example.com \
  --subject "Hello from the terminal" --text "It works!"
```

## Commands

The full reference, generated from the CLI itself, is in [`docs/`](docs/agdir-email.md). `agdir-email <command> --help` shows the same.

| Command | What it does |
| --- | --- |
| [`send`](docs/agdir-email_send.md) | Send (or `--schedule`) an email |
| [`emails get` / `cancel`](docs/agdir-email_emails.md) | Show an email's delivery status; cancel a scheduled email |
| [`batch send <file.json>`](docs/agdir-email_batch_send.md) | Send up to 100 emails in one request (`--permissive` to accept the valid ones) |
| [`domains add` / `list` / `get` / `verify` / `delete`](docs/agdir-email_domains.md) | Manage sending domains (by ID or name) |
| [`domains check <domain>`](docs/agdir-email_domains_check.md) | Look up every required DNS record and show ✓/✗ against the expected value |
| [`webhooks create` / `list` / `get` / `update` / `delete` / `rotate-secret`](docs/agdir-email_webhooks.md) | Manage webhook endpoints |
| [`webhooks deliveries list` / `get` / `attempts` / `replay`](docs/agdir-email_webhooks_deliveries.md) | Inspect what was sent to an endpoint, each HTTP attempt, and replay deliveries |
| [`webhooks verify`](docs/agdir-email_webhooks_verify.md) | Check the signature of a captured webhook request |
| [`login` / `logout` / `whoami` / `profiles`](docs/agdir-email_login.md) | Manage stored API keys and profiles |
| [`completion bash\|zsh\|fish\|powershell`](docs/agdir-email_completion.md) | Shell completion scripts |
| [`version`](docs/agdir-email_version.md) | Print the version |

### Sending

```bash
# HTML from a file, text part from stdin, several recipients, tags and a custom header
./render-report | agdir-email send --from reports@acme.com --to ana@example.com --to ben@example.com \
  --subject "Weekly report" --html-file report.html --text-file - \
  --tag kind=report --header "X-Entity-Ref-ID: week-44" --reply-to support@acme.com

# Schedule (RFC 3339 or relative), then cancel
id=$(agdir-email send -q --from hi@acme.com --to ana@example.com --subject Reminder --text "Tomorrow!" --schedule "in 1 day")
agdir-email emails get "$id"
agdir-email emails cancel "$id"

# Safe to re-run: the same idempotency key never sends twice
agdir-email send --idempotency-key "order-1042/receipt" --from billing@acme.com --to ana@example.com \
  --subject "Your receipt" --html-file receipt.html
```

With no body flag, the text body is read from stdin when it is piped: `echo "Backup done" | agdir-email send --from … --to … --subject …`.

Batch files are a JSON array of emails in the API's format:

```json
[
  { "from": "hi@acme.com", "to": "ana@example.com", "subject": "Hi Ana", "text": "Hello!" },
  { "from": "hi@acme.com", "to": ["ben@example.com"], "subject": "Hi Ben", "html": "<p>Hello!</p>",
    "tags": [{ "name": "kind", "value": "welcome" }], "scheduled_at": "2026-11-02T09:00:00+01:00" }
]
```

### Domains and DNS

`domains add` prints the records to publish (names are relative to your registrable domain, e.g. `send` means `send.acme.com`; for `mail.acme.com` it is `send.mail`). `domains check` resolves each record from your machine and compares it with what Agdir Email expects: DKIM must contain the expected public key, the SPF TXT record must include `amazonses.com` (other `include:`s are fine; two SPF records are flagged), the MX record must point at the feedback host, and DMARC is recommended. Use `--nameserver 1.1.1.1` to bypass a stale local cache. It exits with code 1 while a required record is missing, so it works in scripts:

```bash
until agdir-email domains check acme.com -q; do sleep 60; done && agdir-email domains verify acme.com
```

### Webhooks

```bash
agdir-email webhooks create --endpoint https://acme.com/webhooks/email --event delivered,bounced,complained
agdir-email webhooks deliveries list <webhook-id>
agdir-email webhooks deliveries get <webhook-id> <delivery-id>      # includes the exact payload
agdir-email webhooks deliveries replay <webhook-id> <delivery-id>

# Debug your endpoint's signature check with a captured request
agdir-email webhooks verify --secret "$WEBHOOK_SECRET" --headers-file headers.txt body.json
```

`--headers-file` takes `Name: value` lines (a pasted request dump works; the request line is skipped) or a JSON object. On a mismatch the CLI prints the signature it expected, so you can tell a wrong secret from a modified body. `--ignore-timestamp` checks old captures.

## Authentication and configuration

| Setting | Precedence (first wins) |
| --- | --- |
| API key | `--api-key` → `AGDIR_EMAIL_API_KEY` → the profile's key |
| Base URL | `--base-url` → `AGDIR_EMAIL_BASE_URL` → the profile's base URL → `https://api.agdir.email` |
| Profile | `--profile` → `AGDIR_EMAIL_PROFILE` → the current profile (set by `login` / `profiles use`) → `default` |

`login` stores keys in `agdir-email/config.json` in the OS config directory (`~/.config` on Linux, `~/Library/Application Support` on macOS, `%AppData%` on Windows), created with `0600` permissions; `AGDIR_EMAIL_CONFIG` points elsewhere. Keep several teams or environments apart with profiles:

```bash
agdir-email login --profile staging --base-url https://api.staging.example
agdir-email profiles list
agdir-email whoami --profile staging     # shows the key (masked), its source, base URL and access level
agdir-email profiles use default
```

A sending-only key works for `send` and `batch send`; `whoami` reports it as "sending access". Other commands need a full-access key.

## JSON output

Every command accepts `--json` and then prints a single JSON document to stdout with the API's snake_case field names (`id`, `status`, `created_at`, `reply_to`, `records[].purpose`, …). Lists print `{"data": [...], "has_more": false, "next_cursor": "…"}`; list commands fetch every page unless `--limit`/`--after` is given. Errors go to stderr as `{"error": {"code": "validation_error", "status": 422, "message": "…"}}`. `--quiet` (`-q`) prints only IDs, one per line.

```bash
domain_id=$(agdir-email domains add acme.com --json | jq -r .id)
agdir-email domains check acme.com --json | jq '.records[] | select(.ok | not) | .fqdn'
```

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | API error (validation, not found, quota, network…), failed `domains check`, invalid webhook signature, or emails rejected by a permissive batch |
| 2 | Usage error: unknown command or flag, missing argument, unreadable input file |
| 3 | Authentication: no API key configured, or the key is invalid, revoked or not allowed for the command |

Error messages include the API's error code, e.g. `Error: daily_quota_exceeded: … (HTTP 429)`. Rate limits and transient server errors are retried automatically, and sends carry an idempotency key so a retry never delivers twice.

## CI usage

Send a deploy notification from GitHub Actions (store a sending-only key as the `AGDIR_EMAIL_API_KEY` secret):

```yaml
- name: Notify the team
  env:
    AGDIR_EMAIL_API_KEY: ${{ secrets.AGDIR_EMAIL_API_KEY }}
  run: |
    go install github.com/AgdirAS/agdirlabs-sdks/cli/cmd/agdir-email@latest
    agdir-email send \
      --from "Deploy bot <deploys@acme.com>" --to team@acme.com \
      --subject "Deployed ${GITHUB_REPOSITORY}@${GITHUB_SHA::7}" \
      --text "Run: ${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}" \
      --idempotency-key "deploy/${GITHUB_RUN_ID}/${GITHUB_RUN_ATTEMPT}" \
      --tag kind=deploy
```

Without Go on the runner, download the Linux binary from the releases page instead (see [Install](#install)).

## Development

```bash
cd cli
go test -race ./...                      # command tests against a fake API and a fake DNS resolver
go run ./cmd/agdir-email --help
go run ./internal/tools/gendocs docs     # regenerate docs/ after changing commands or flags

# Opt-in end-to-end test against a running Agdir Email API (never production):
AGDIR_EMAIL_LIVE_BASE_URL=https://your-test-instance.example \
AGDIR_EMAIL_LIVE_API_KEY=re_… \
AGDIR_EMAIL_LIVE_FROM=hi@your-verified-domain.example \
go test -run TestLive -v ./internal/app
```

The module `github.com/AgdirAS/agdirlabs-sdks/cli` requires the Go SDK `github.com/AgdirAS/agdirlabs-sdks/go`. `go.mod` has no `replace` directive; instead `cli/go.work` points the SDK at `../go`, so local builds, tests and GoReleaser always use the SDK from the same commit. `go install …@version` ignores `go.work` and uses the published SDK tag. `./tidy.sh` runs `go mod tidy` while the SDK version required by `go.mod` is not tagged yet.

## Releasing

Releases are built by [GoReleaser](https://goreleaser.com) from this repository (`cli/.goreleaser.yaml`) by `.github/workflows/cli-release.yml`, triggered by tags **`cli/vX.Y.Z`**. The workflow checks the tag against `Version` in `internal/app/version.go` and `CHANGELOG.md`, runs the tests, builds every target, creates the GitHub release `cli/vX.Y.Z` with archives, `.deb`/`.rpm`/`.apk` packages and `checksums.txt`, and only then publishes the Homebrew cask, the Scoop manifest and the winget pull request (so their download URLs already work).

To release:

1. If the CLI needs a newer SDK, release the SDK first (`go/vX.Y.Z`), then in `cli/` run `GOWORK=off go get github.com/AgdirAS/agdirlabs-sdks/go@vX.Y.Z && GOWORK=off go mod tidy`, so `go.sum` records the published SDK and `go install` works.
2. Update `Version` in `internal/app/version.go` and `CHANGELOG.md`, regenerate `docs/`, merge to `main`.
3. `git tag cli/vX.Y.Z && git push origin cli/vX.Y.Z`.

**The first release needs the SDK tag `go/v0.1.0` to exist** before `go install …/cli/cmd/agdir-email@latest` can work, and step 1 must be done once after tagging it (`go.sum` currently has no entry for the SDK because it isn't published yet).

Dry run locally (builds all six targets, packages and manifests into `cli/dist/` without publishing):

```bash
docker run --rm -v "$PWD":/src -w /src/cli -e GORELEASER_CURRENT_TAG=v0.1.0 goreleaser/goreleaser release --snapshot --clean
```

### One-time setup (owner)

1. **Homebrew tap**: create the public repo `AgdirAS/homebrew-tap` (empty, default branch `main`). Users then run `brew install agdiras/tap/agdir-email`. The workflow writes `Casks/agdir-email.rb`.
2. **Scoop bucket**: create the public repo `AgdirAS/scoop-bucket` (default branch `main`). The workflow writes `agdir-email.json`.
3. **winget**: fork [`microsoft/winget-pkgs`](https://github.com/microsoft/winget-pkgs) to `AgdirAS/winget-pkgs`. Each release pushes the manifests for `AgdirLabs.AgdirEmail` to a branch of the fork and opens a pull request upstream; the first one is reviewed by winget moderators (later ones are usually auto-merged). Accept the Microsoft CLA on the first PR.
4. **Credentials**: the Homebrew tap and the Scoop bucket are pushed with write **deploy keys** (secrets `HOMEBREW_TAP_DEPLOY_KEY`, `SCOOP_BUCKET_DEPLOY_KEY` in `AgdirAS/agdirlabs-sdks`; public halves on each repo). The winget pull request needs a token that can push to the fork `AgdirAS/winget-pkgs` and open PRs on `microsoft/winget-pkgs` (a classic token with `public_repo`, ideally from a bot account), saved as **`WINGET_GITHUB_TOKEN`**; without it the release skips winget with a notice. The release's own assets use the workflow's `GITHUB_TOKEN`.
5. **Optional, later**: macOS code signing and notarization (Apple Developer ID, GoReleaser `notarize`), Windows Authenticode signing, signed checksums (cosign) and SBOMs (`sboms:` with syft). Until binaries are notarized, the Homebrew cask removes the quarantine attribute on install; a manually downloaded macOS binary may need `xattr -d com.apple.quarantine agdir-email`.

## Contributing

Issues and pull requests are welcome — see [CONTRIBUTING.md](../CONTRIBUTING.md). Changes are listed in [CHANGELOG.md](CHANGELOG.md).

## License

[MIT](LICENSE) © [Agdir AS](https://agdir.no)
