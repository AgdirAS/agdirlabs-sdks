# Agdir Email Go SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/AgdirAS/agdirlabs-sdks/go.svg)](https://pkg.go.dev/github.com/AgdirAS/agdirlabs-sdks/go)
[![CI](https://github.com/AgdirAS/agdirlabs-sdks/actions/workflows/go.yml/badge.svg)](https://github.com/AgdirAS/agdirlabs-sdks/actions/workflows/go.yml)

The official Go SDK for [Agdir Email](https://agdir.email) — programmatic SMTP for developers, hosted in the EU. Built by [Agdir Labs](https://agdirlabs.com) at [Agdir](https://agdir.no).

- Standard library only, Go 1.23+
- `context.Context` on every call
- Safe automatic retries with idempotency keys
- Cursor pagination with range-over-func iterators
- Webhook signature verification and a ready-made `http.Handler`

Keywords: email, smtp, transactional-email, agdir.

## Install

```bash
go get github.com/AgdirAS/agdirlabs-sdks/go
```

The import path ends in `/go`, so give the package its name when importing it:

```go
import agdiremail "github.com/AgdirAS/agdirlabs-sdks/go"
```

## Quick start

Create an API key at [app.agdir.email](https://app.agdir.email) and verify a sending domain, then:

```go
client, err := agdiremail.NewClient(agdiremail.WithAPIKey(os.Getenv("AGDIR_EMAIL_API_KEY")))
if err != nil {
	log.Fatal(err)
}

sent, err := client.Emails.Send(ctx, &agdiremail.SendEmailRequest{
	From:    "Acme <hello@acme.com>",
	To:      []string{"user@example.com"},
	Subject: "Welcome to Acme",
	HTML:    "<p>Thanks for signing up!</p>",
})
if err != nil {
	log.Fatal(err)
}
fmt.Println(sent.ID)
```

`agdiremail.NewClient()` with no options reads `AGDIR_EMAIL_API_KEY` from the environment.

## Emails

```go
// Plain text, several recipients, reply-to, tags, custom headers
_, err := client.Emails.Send(ctx, &agdiremail.SendEmailRequest{
	From:    "Acme <billing@acme.com>",
	To:      []string{"ana@example.com", "ben@example.com"},
	Cc:      []string{"finance@acme.com"},
	ReplyTo: []string{"support@acme.com"},
	Subject: "Your invoice",
	Text:    "Invoice #1042 is attached in your dashboard.",
	Tags:    []agdiremail.Tag{{Name: "kind", Value: "invoice"}},
	Headers: map[string]string{"X-Entity-Ref-ID": "inv-1042"},
})

// Schedule for later, then cancel if plans change
email.ScheduledAt = time.Now().Add(time.Hour)
sent, err := client.Emails.Send(ctx, email)
_, err = client.Emails.Cancel(ctx, sent.ID)

// Delivery status
got, err := client.Emails.Get(ctx, sent.ID)
got.Status // agdiremail.EmailScheduled, EmailSent, EmailDelivered, EmailBounced, …

// Up to 100 emails per request (strict: one invalid email rejects the batch)
res, err := client.Emails.SendBatch(ctx, []agdiremail.SendEmailRequest{email1, email2})

// Accept the valid ones, report the rest
res, err = client.Emails.SendBatch(ctx, emails, agdiremail.WithBatchValidation(agdiremail.BatchPermissive))
for _, e := range res.Errors {
	log.Printf("email %d rejected: %s", e.Index, e.Message)
}
```

## Domains

```go
domain, err := client.Domains.Create(ctx, &agdiremail.CreateDomainRequest{Name: "acme.com"})
for _, r := range domain.Records {
	fmt.Println(r.Type, r.Name, r.Value) // add these to your DNS
}

_, err = client.Domains.Verify(ctx, domain.ID) // check DNS now instead of waiting
d, err := client.Domains.Get(ctx, domain.ID)  // d.Status: pending → verified

// One page at a time…
page, err := client.Domains.List(ctx, &agdiremail.ListParams{Limit: 50})
next, err := client.Domains.List(ctx, &agdiremail.ListParams{Limit: 50, After: page.NextCursor})

// …or every item, fetching pages as you go
for d, err := range client.Domains.ListAll(ctx, nil) {
	if err != nil {
		return err
	}
	fmt.Println(d.Name, d.Status)
}

_, err = client.Domains.Delete(ctx, domain.ID)
```

## Webhooks

```go
hook, err := client.Webhooks.Create(ctx, &agdiremail.CreateWebhookRequest{
	Endpoint: "https://acme.com/webhooks/email",
	Events:   []agdiremail.WebhookEventType{agdiremail.EventEmailDelivered, agdiremail.EventEmailBounced},
})
// hook.SigningSecret is "whsec_…": keep it to verify incoming events

_, err = client.Webhooks.Update(ctx, hook.ID, &agdiremail.UpdateWebhookRequest{Status: agdiremail.WebhookDisabled})
rotated, err := client.Webhooks.RotateSecret(ctx, hook.ID)

// What was sent to your endpoint, and each HTTP attempt
deliveries, err := client.Webhooks.Deliveries.List(ctx, hook.ID, nil)
attempts, err := client.Webhooks.Deliveries.Attempts(ctx, hook.ID, deliveries.Data[0].ID, nil)
_, err = client.Webhooks.Deliveries.Replay(ctx, hook.ID, deliveries.Data[0].ID)
```

### Verifying incoming events

Every request to your endpoint is signed ([Standard Webhooks](https://www.standardwebhooks.com)). The simplest way to receive events is `WebhookHandler`, which verifies each request and responds `204` (or `400` for a bad signature, `500` if your function returns an error so the event is retried):

```go
http.Handle("POST /webhooks/email", agdiremail.WebhookHandler(os.Getenv("AGDIR_EMAIL_WEBHOOK_SECRET"),
	func(ctx context.Context, event *agdiremail.WebhookEvent) error {
		if event.Type == agdiremail.EventEmailBounced {
			return markBounced(ctx, event.Data.To, event.Data.Bounce)
		}
		return nil
	}))
```

Or verify the **raw** body yourself, in any framework:

```go
payload, err := io.ReadAll(r.Body)
// …
event, err := agdiremail.VerifyWebhook(payload, r.Header, secret)
var verr *agdiremail.WebhookVerificationError
if errors.As(err, &verr) {
	http.Error(w, "invalid signature", http.StatusBadRequest)
	return
}
```

With Gin use `c.GetRawData()` and `c.Request.Header`; with Echo, `io.ReadAll(c.Request().Body)` and `c.Request().Header`.

Messages older than 5 minutes are rejected (`agdiremail.WithTolerance` to change). Failed deliveries are retried for about 18 hours; use `event.Data.EmailID` to deduplicate.

## Errors

Every failed call returns an `*agdiremail.Error`:

```go
_, err := client.Emails.Send(ctx, email)
var apiErr *agdiremail.Error
if errors.As(err, &apiErr) {
	apiErr.Code       // agdiremail.CodeValidationError, CodeDailyQuotaExceeded, CodeRateLimitExceeded, CodeNetworkError, …
	apiErr.Status     // HTTP status, 0 if no response
	apiErr.Message    // human-readable explanation
	apiErr.RetryAfter // from the retry-after header, 0 if none
}
```

| Code | HTTP | Meaning |
|---|---|---|
| `CodeMissingAPIKey`, `CodeRestrictedAPIKey` | 401 | No key, or a sending-only key on a read endpoint |
| `CodeInvalidAPIKey` | 403 | Unknown or revoked key |
| `CodeValidationError` | 422 / 403 | Invalid request; unverified domain, paused team or domain limit |
| `CodeNotFound` | 404 | No such object |
| `CodeInvalidIdempotentRequest`, `CodeConcurrentIdempotentRequests` | 409 | Idempotency key reused with another body, or still in flight |
| `CodeDailyQuotaExceeded`, `CodeMonthlyQuotaExceeded` | 429 | Plan limit reached (never retried) |
| `CodeRateLimitExceeded` | 429 | Too many requests (retried automatically) |
| `CodeInternalServerError` | 5xx | Server error |
| `CodeNetworkError`, `CodeTimeout` | 0 | No response received |

Cancelling the `ctx` you pass in returns the context's error (`context.Canceled` / `context.DeadlineExceeded`) instead. `NewClient` returns `agdiremail.ErrMissingAPIKey` when no key is configured.

## Retries and idempotency

Rate limits (`429 rate_limit_exceeded`), server errors and network failures are retried up to `WithMaxRetries` times (default 2) with exponential backoff, honouring `retry-after`. Sends are only retried with an idempotency key, so a retry can never deliver an email twice: the SDK generates one per call, or you can pass your own to make sends safe across process restarts too:

```go
client.Emails.Send(ctx, email, agdiremail.WithIdempotencyKey("welcome/"+user.ID))
```

Quota errors are not retried.

## Configuration

```go
client, err := agdiremail.NewClient(
	agdiremail.WithAPIKey("re_…"),                    // default: AGDIR_EMAIL_API_KEY
	agdiremail.WithBaseURL("https://api.agdir.email"), // default: AGDIR_EMAIL_BASE_URL, else this
	agdiremail.WithMaxRetries(2),
	agdiremail.WithTimeout(30*time.Second),            // per attempt
	agdiremail.WithHTTPClient(&http.Client{Transport: myTransport}), // proxies, tracing
)
```

The client is safe for concurrent use; create one and share it.

## Development

```bash
go test -race ./...
go vet ./...

# Opt-in integration test against a running Agdir Email API (never production):
AGDIR_EMAIL_LIVE_BASE_URL=https://your-test-instance.example \
AGDIR_EMAIL_LIVE_API_KEY=re_… \
AGDIR_EMAIL_LIVE_FROM=hi@your-verified-domain.example \
go test -run TestLive -v ./...
```

## Releasing

Go modules are published by git tags; there is no registry upload. This module lives in the `go/` folder of the monorepo, so its release tags are prefixed with the folder: **`go/vX.Y.Z`** (e.g. `go/v0.1.0`).

1. Update `Version` in `client.go` and `CHANGELOG.md`, merge to `main`.
2. Tag the merge commit and push the tag: `git tag go/v0.1.0 && git push origin go/v0.1.0`.
3. pkg.go.dev picks the version up on the first `go get github.com/AgdirAS/agdirlabs-sdks/go@v0.1.0` (or request it at `https://pkg.go.dev/github.com/AgdirAS/agdirlabs-sdks/go@v0.1.0`).

Tags are immutable in the Go module proxy: never move or delete a published tag; release a new patch version instead. A v2+ release would need the module path `github.com/AgdirAS/agdirlabs-sdks/go/v2`.

## Contributing

Issues and pull requests are welcome — see [CONTRIBUTING.md](../CONTRIBUTING.md). Changes are listed in [CHANGELOG.md](CHANGELOG.md).

## License

[MIT](LICENSE) © [Agdir AS](https://agdir.no)
