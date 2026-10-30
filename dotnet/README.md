# AgdirLabs.Email

[![NuGet](https://img.shields.io/nuget/v/AgdirLabs.Email.svg)](https://www.nuget.org/packages/AgdirLabs.Email)
[![CI](https://github.com/AgdirAS/agdirlabs-sdks/actions/workflows/dotnet.yml/badge.svg)](https://github.com/AgdirAS/agdirlabs-sdks/actions/workflows/dotnet.yml)

The official .NET SDK for [Agdir Email](https://agdir.email): programmatic SMTP for developers, hosted in the EU. Built by [Agdir Labs](https://agdirlabs.com) at [Agdir](https://agdir.no).

- .NET 8+ and .NET Standard 2.0 (.NET Framework 4.6.2+, Mono, Unity)
- `async` all the way, with `CancellationToken` on every call
- Safe automatic retries with idempotency keys
- Cursor pagination with `await foreach`
- Webhook signature verification built in
- No reflection: trimming- and Native AOT-friendly on .NET 8+
- `services.AddAgdirEmail()` for ASP.NET Core and the generic host (separate package)

## Install

```bash
dotnet add package AgdirLabs.Email
# ASP.NET Core / generic host integration (optional):
dotnet add package AgdirLabs.Email.Extensions.DependencyInjection
```

## Quick start

Create an API key at [app.agdir.email](https://app.agdir.email) and verify a sending domain, then:

```csharp
using AgdirLabs.Email;

var agdir = new AgdirEmailClient("re_…");

var result = await agdir.Emails.SendAsync(new SendEmailRequest
{
    From = "Acme <hello@acme.com>",
    To = { "user@example.com" },
    Subject = "Welcome to Acme",
    Html = "<p>Thanks for signing up!</p>",
});
Console.WriteLine(result.Id);
```

`new AgdirEmailClient()` with no arguments reads `AGDIR_EMAIL_API_KEY` from the environment.
The client is thread-safe and cheap to create; instances share one pooled connection handler.

## Emails

```csharp
// Plain text, several recipients, reply-to, tags, custom headers
await agdir.Emails.SendAsync(new SendEmailRequest
{
    From = "Acme <billing@acme.com>",
    To = { "ana@example.com", "ben@example.com" },
    Cc = new[] { "finance@acme.com" },
    ReplyTo = new[] { "support@acme.com" },
    Subject = "Your invoice",
    Text = "Invoice #1042 is attached in your dashboard.",
    Tags = new[] { new EmailTag("kind", "invoice") },
    Headers = new Dictionary<string, string> { ["X-Entity-Ref-ID"] = "inv-1042" },
});

// Schedule for later, then cancel if plans change
var scheduled = await agdir.Emails.SendAsync(new SendEmailRequest
{
    From = "Acme <hello@acme.com>",
    To = { "user@example.com" },
    Subject = "Your trial ends tomorrow",
    Text = "…",
    ScheduledAt = DateTimeOffset.UtcNow.AddHours(1),
});
await agdir.Emails.CancelAsync(scheduled.Id);

// Delivery status
var email = await agdir.Emails.GetAsync(scheduled.Id);
Console.WriteLine(email.Status); // Queued, Scheduled, Sent, Delivered, Bounced, Complained, Canceled, …

// Up to 100 emails per request (all-or-nothing)
var batch = await agdir.Emails.SendBatchAsync(new[] { welcome, receipt });

// Accept the valid ones, report the rest
var partial = await agdir.Emails.SendBatchAsync(emails, new SendBatchOptions { Validation = BatchValidation.Permissive });
foreach (var error in partial.Errors) Console.WriteLine($"#{error.Index}: {error.Message}");
```

## Domains

```csharp
var domain = await agdir.Domains.CreateAsync(new CreateDomainRequest { Name = "acme.com" });
foreach (var record in domain.Records)
{
    Console.WriteLine($"{record.Type} {record.Name} {record.Value}"); // add these to your DNS
}

await agdir.Domains.VerifyAsync(domain.Id); // check DNS now instead of waiting
var current = await agdir.Domains.GetAsync(domain.Id); // Status: Pending → Verified

await foreach (var d in agdir.Domains.ListAllAsync())
{
    Console.WriteLine($"{d.Name}: {d.Status}");
}

await agdir.Domains.DeleteAsync(domain.Id);
```

`ListAsync(limit, after)` returns one `Page<T>` (`Data`, `HasMore`, `NextCursor`) if you prefer to page yourself.

## Webhooks

```csharp
var webhook = await agdir.Webhooks.CreateAsync(new CreateWebhookRequest
{
    Endpoint = "https://acme.com/webhooks/email",
    Events = { WebhookEventTypes.EmailDelivered, WebhookEventTypes.EmailBounced, WebhookEventTypes.EmailComplained },
});
// Store webhook.SigningSecret ("whsec_…") to verify incoming events.

await agdir.Webhooks.UpdateAsync(webhook.Id, new UpdateWebhookRequest { Status = WebhookStatus.Disabled });
await agdir.Webhooks.RotateSecretAsync(webhook.Id);

// What was sent to your endpoint, and each HTTP attempt
var deliveries = await agdir.Webhooks.Deliveries.ListAsync(webhook.Id);
var attempts = await agdir.Webhooks.Deliveries.ListAttemptsAsync(webhook.Id, deliveries.Data[0].Id);
await agdir.Webhooks.Deliveries.ReplayAsync(webhook.Id, deliveries.Data[0].Id);
```

### Verifying incoming events

Every request to your endpoint is signed ([Standard Webhooks](https://www.standardwebhooks.com)). Verify it against the **raw** body before trusting it:

```csharp
// ASP.NET Core minimal API
app.MapPost("/webhooks/email", async (HttpRequest request, IConfiguration config) =>
{
    using var reader = new StreamReader(request.Body);
    var payload = await reader.ReadToEndAsync(); // the exact bytes, not re-serialized JSON

    WebhookEvent evt;
    try
    {
        evt = WebhookVerifier.Verify(payload, name => request.Headers[name], config["AgdirEmail:WebhookSecret"]!);
    }
    catch (WebhookVerificationException)
    {
        return Results.BadRequest();
    }

    if (evt.Type == WebhookEventTypes.EmailBounced)
    {
        await MarkBouncedAsync(evt.Data.To, evt.Data.Bounce!);
    }
    return Results.NoContent();
});
```

`WebhookVerifier.Verify` also accepts an `IDictionary<string, string>` (matched case-insensitively) or `HttpHeaders`.
Messages older than 5 minutes are rejected (pass `tolerance:` to change). Failed deliveries are retried for about 18 hours; use `evt.Data.EmailId` to deduplicate.

## Errors

Every failed call throws an `AgdirEmailException`:

```csharp
try
{
    await agdir.Emails.SendAsync(email);
}
catch (AgdirEmailException e) when (e.Code == AgdirEmailErrorCodes.DailyQuotaExceeded)
{
    // try again tomorrow
}
catch (AgdirEmailException e)
{
    Console.WriteLine(e.Code);       // "validation_error", "rate_limit_exceeded", "network_error", "timeout", …
    Console.WriteLine(e.StatusCode); // HTTP status, 0 if no response
    Console.WriteLine(e.Message);    // human-readable explanation
    Console.WriteLine(e.RetryAfter); // TimeSpan? from retry-after, when present
}
```

`AgdirEmailErrorCodes` lists every code. Cancelling via your `CancellationToken` throws `OperationCanceledException` as usual.

## Retries and idempotency

Rate limits (`429 rate_limit_exceeded`), server errors (408/500/502/503/504) and network failures are retried up to `MaxRetries` times (default 2), honouring `retry-after`, else with exponential backoff and jitter. Sends are only retried with an idempotency key, so a retry can never deliver an email twice: the SDK generates one per call, or you can pass your own to make sends safe across process restarts too:

```csharp
await agdir.Emails.SendAsync(email, new SendEmailOptions { IdempotencyKey = $"welcome/{user.Id}" });
```

Quota errors (`daily_quota_exceeded`, `monthly_quota_exceeded`) are never retried.

## Configuration

```csharp
var agdir = new AgdirEmailClient(new AgdirEmailOptions
{
    ApiKey = "re_…",                       // default: AGDIR_EMAIL_API_KEY
    BaseUrl = "https://api.agdir.email",   // default: AGDIR_EMAIL_BASE_URL, else this
    MaxRetries = 2,
    Timeout = TimeSpan.FromSeconds(30),    // per attempt
}, httpClient);                            // optional: your own HttpClient (proxy, handlers); never disposed by the SDK
```

## ASP.NET Core and dependency injection

With `AgdirLabs.Email.Extensions.DependencyInjection`:

```csharp
builder.Services.AddAgdirEmail(); // binds the "AgdirEmail" configuration section
```

```json
{
  "AgdirEmail": {
    "ApiKey": "re_…"
  }
}
```

(or the environment variable `AgdirEmail__ApiKey`, or `AGDIR_EMAIL_API_KEY`). Configure in code with `AddAgdirEmail(o => o.MaxRetries = 3)`. Then inject `IAgdirEmailClient`:

```csharp
app.MapPost("/signup", async (SignupForm form, IAgdirEmailClient agdir, CancellationToken ct) =>
{
    await agdir.Emails.SendAsync(new SendEmailRequest
    {
        From = "Acme <hello@acme.com>",
        To = { form.Email },
        Subject = "Welcome to Acme",
        Html = "<p>Thanks for signing up!</p>",
    }, cancellationToken: ct);
    return Results.Ok();
});
```

The client's `HttpClient` comes from `IHttpClientFactory` (named `AgdirEmail`); `AddAgdirEmail` returns the `IHttpClientBuilder`, so you can add handlers, proxies or resilience policies.

## Testing your code

`IAgdirEmailClient`, `IEmailsResource`, `IDomainsResource`, `IWebhooksResource` and `IWebhookDeliveriesResource` are interfaces, so they are easy to mock or fake. To test at the HTTP level instead, pass an `HttpClient` with your own `HttpMessageHandler`.

## Releasing

Releases are published from GitHub Actions with [NuGet Trusted Publishing](https://learn.microsoft.com/nuget/nuget-org/trusted-publishing) (OIDC): no NuGet API key is stored anywhere.

1. Bump `<Version>` in `dotnet/Directory.Build.props` and `AgdirEmailClient.Version` (a test checks they match), and update `CHANGELOG.md`.
2. Push a tag `dotnet/v<version>`, e.g. `git tag dotnet/v0.1.0 && git push origin dotnet/v0.1.0`.
3. `.github/workflows/dotnet-release.yml` checks the tag against the version, tests, packs `AgdirLabs.Email` and `AgdirLabs.Email.Extensions.DependencyInjection` (with symbols), exchanges the workflow's OIDC token for a short-lived key via `NuGet/login`, and pushes both packages.

One-time setup (by a nuget.org owner):

1. On nuget.org, sign in as the account that will own the packages (ideally an `AgdirAS` organization), open **Trusted Publishing** and add a policy: repository owner `AgdirAS`, repository `agdirlabs-sdks`, workflow file `dotnet-release.yml` (no environment).
2. In the GitHub repository, add the Actions **variable** (not secret) `NUGET_USER` = the nuget.org profile name that owns the policy.
3. Optionally reserve the `AgdirLabs.` package ID prefix on nuget.org (verified-owner badge).

## Contributing

Issues and pull requests are welcome; see [CONTRIBUTING.md](https://github.com/AgdirAS/agdirlabs-sdks/blob/main/CONTRIBUTING.md). Changes are listed in [CHANGELOG.md](https://github.com/AgdirAS/agdirlabs-sdks/blob/main/dotnet/CHANGELOG.md).

```bash
cd dotnet
dotnet test                                     # unit tests (net8.0 + net10.0)
dotnet test -p:TestNetStandard=true -f net8.0   # the same tests against the netstandard2.0 build
dotnet build tests/AgdirLabs.Email.Tests -f net472 -p:IncludeNetFx=true   # .NET Framework (runs on Windows)
dotnet format --verify-no-changes

# Live tests against a local (never production) Agdir Email API:
AGDIR_EMAIL_LIVE_BASE_URL=http://127.0.0.1:<port>/<api-path> \
AGDIR_EMAIL_LIVE_API_KEY=re_… \
AGDIR_EMAIL_LIVE_FROM=hi@your-verified-domain.example \
  dotnet test
```

## License

[MIT](https://github.com/AgdirAS/agdirlabs-sdks/blob/main/dotnet/LICENSE) © [Agdir AS](https://agdir.no)
