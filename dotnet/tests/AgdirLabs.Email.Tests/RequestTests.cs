using System;
using System.Collections.Generic;
using System.Linq;
using System.Text.Json;
using System.Threading.Tasks;
using Xunit;

namespace AgdirLabs.Email.Tests;

public sealed class RequestTests
{
    private static SendEmailRequest Simple(string to = "b@example.com") =>
        new() { From = "a@acme.com", To = { to }, Subject = "s", Text = "t" };

    [Fact]
    public async Task SendsAnEmailAsApiJsonWithAuthUserAgentAndIdempotencyKey()
    {
        var handler = new FakeHandler().Respond(new { id = "em_1" });
        var agdir = TestClient.Create(handler);

        var result = await agdir.Emails.SendAsync(new SendEmailRequest
        {
            From = "Acme <hello@acme.com>",
            To = { "user@example.com" },
            Subject = "Hi",
            Text = "Hello",
            Cc = new[] { "cc@example.com" },
            ReplyTo = new[] { "support@acme.com" },
            Headers = new Dictionary<string, string> { ["X-Entity-Ref-ID"] = "42" },
            ScheduledAt = new DateTimeOffset(2026, 11, 1, 10, 0, 0, TimeSpan.FromHours(1)),
            Tags = new[] { new EmailTag("kind", "welcome") },
        });

        Assert.Equal(new SendEmailResult("em_1"), result);
        var req = Assert.Single(handler.Requests);
        Assert.Equal("POST", req.Method.Method);
        Assert.Equal("https://api.test/emails", req.Url.ToString());
        Assert.Equal("Bearer re_test", req.Header("Authorization"));
        Assert.Equal("agdirlabs-email-dotnet/0.1.0", req.Header("User-Agent"));
        Assert.Equal("application/json", req.Header("Accept"));
        Assert.StartsWith("application/json", req.Header("Content-Type"));
        Assert.Matches("^[0-9a-f-]{36}$", req.Header("Idempotency-Key"));
        TestClient.AssertJson(
            """
            {"from":"Acme <hello@acme.com>","to":["user@example.com"],"subject":"Hi","text":"Hello","cc":["cc@example.com"],
             "reply_to":["support@acme.com"],"headers":{"X-Entity-Ref-ID":"42"},"tags":[{"name":"kind","value":"welcome"}],
             "scheduled_at":"2026-11-01T09:00:00.000Z"}
            """,
            req.Body);
    }

    [Fact]
    public async Task UsesTheCallerIdempotencyKey_AndNoneWhenRetriesAreOff()
    {
        var handler = new FakeHandler().Respond(new { id = "a" }).Respond(new { id = "b" });
        await TestClient.Create(handler).Emails.SendAsync(Simple(), new SendEmailOptions { IdempotencyKey = "order-42" });
        await TestClient.Create(handler, maxRetries: 0).Emails.SendAsync(Simple());
        Assert.Equal("order-42", handler.Requests[0].Header("Idempotency-Key"));
        Assert.Null(handler.Requests[1].Header("Idempotency-Key"));
    }

    [Fact]
    public async Task SendsPermissiveBatchesAndReturnsPerIndexErrors()
    {
        var handler = new FakeHandler()
            .Respond(new { data = new[] { new { id = "em_1" } }, errors = new[] { new { index = 1, message = "bad to" } } })
            .Respond(new { data = new[] { new { id = "em_2" } } });
        var agdir = TestClient.Create(handler);

        var result = await agdir.Emails.SendBatchAsync(new[] { Simple(), Simple("nope") }, new SendBatchOptions { Validation = BatchValidation.Permissive });
        Assert.Equal(new[] { new SendEmailResult("em_1") }, result.Data);
        Assert.Equal(new[] { new BatchError(1, "bad to") }, result.Errors);
        var req = handler.Requests[0];
        Assert.Equal("https://api.test/emails/batch", req.Url.ToString());
        Assert.Equal("permissive", req.Header("x-batch-validation"));
        Assert.Equal(2, req.Json.GetArrayLength());
        Assert.Equal("nope", req.Json[1].GetProperty("to")[0].GetString());

        var strict = await agdir.Emails.SendBatchAsync(new[] { Simple() });
        Assert.Empty(strict.Errors);
        Assert.Null(handler.Requests[1].Header("x-batch-validation"));
        Assert.NotNull(handler.Requests[1].Header("Idempotency-Key"));
    }

    [Fact]
    public async Task MapsAnEmail_NullListsBecomeEmpty_LastEventBecomesStatus()
    {
        var handler = new FakeHandler().Respond(new
        {
            @object = "email",
            id = "em_1",
            from = "a@acme.com",
            to = new[] { "b@example.com" },
            cc = (string[]?)null,
            bcc = (string[]?)null,
            reply_to = "r@acme.com",
            subject = "s",
            html = (string?)null,
            text = "t",
            created_at = "2026-10-30T10:00:00.123Z",
            scheduled_at = (string?)null,
            last_event = "delivery_delayed",
        });
        var email = await TestClient.Create(handler).Emails.GetAsync("em_1");

        Assert.Equal("https://api.test/emails/em_1", handler.Requests[0].Url.ToString());
        Assert.Equal("GET", handler.Requests[0].Method.Method);
        Assert.Equal(new[] { "b@example.com" }, email.To);
        Assert.Empty(email.Cc);
        Assert.Empty(email.Bcc);
        Assert.Equal(new[] { "r@acme.com" }, email.ReplyTo);
        Assert.Null(email.Html);
        Assert.Equal("t", email.Text);
        Assert.Equal(new DateTimeOffset(2026, 10, 30, 10, 0, 0, 123, TimeSpan.Zero), email.CreatedAt);
        Assert.Null(email.ScheduledAt);
        Assert.Equal(EmailStatus.DeliveryDelayed, email.Status);
    }

    [Fact]
    public async Task UnknownStatusesMapToUnknown()
    {
        var handler = new FakeHandler().Respond(new { id = "em_1", created_at = "2026-10-30T10:00:00Z", last_event = "teleported" });
        Assert.Equal(EmailStatus.Unknown, (await TestClient.Create(handler).Emails.GetAsync("em_1")).Status);
    }

    [Fact]
    public async Task CancelsAndEscapesIds()
    {
        var handler = new FakeHandler().Respond(new { @object = "email", id = "a/b c" });
        var result = await TestClient.Create(handler).Emails.CancelAsync("a/b c");
        Assert.Equal("a/b c", result.Id);
        Assert.Equal("https://api.test/emails/a%2Fb%20c/cancel", handler.Requests[0].Url.OriginalString);
        await Assert.ThrowsAsync<ArgumentException>(() => TestClient.Create(handler).Emails.GetAsync(""));
    }

    private static object DomainJson(string id, string name) => new
    {
        @object = "domain",
        id,
        name,
        status = "temporary_failure",
        created_at = "2026-10-30T10:00:00Z",
        region = "eu-north-1",
        capabilities = new { sending = "enabled", receiving = "disabled" },
        open_tracking = false,
        click_tracking = true,
        records = new object[]
        {
            new { record = "DKIM", name = "agdir._domainkey", type = "TXT", ttl = "Auto", status = "verified", value = "p=abc" },
            new { record = "SPF", name = "send", type = "MX", ttl = "Auto", status = "pending", value = "feedback-smtp.eu-north-1.amazonses.com", priority = 10 },
        },
    };

    [Fact]
    public async Task DomainsCreateGetVerifyDelete()
    {
        var handler = new FakeHandler()
            .Respond(DomainJson("d_1", "acme.com"))
            .Respond(DomainJson("d_1", "acme.com"))
            .Respond(new { @object = "domain", id = "d_1" })
            .Respond(new { @object = "domain", id = "d_1", deleted = true });
        var agdir = TestClient.Create(handler);

        var created = await agdir.Domains.CreateAsync(new CreateDomainRequest { Name = "acme.com" });
        TestClient.AssertJson("""{"name":"acme.com"}""", handler.Requests[0].Body);
        Assert.Equal(DomainStatus.TemporaryFailure, created.Status);
        Assert.True(created.ClickTracking);
        Assert.False(created.OpenTracking);
        Assert.Equal("eu-north-1", created.Region);
        Assert.Equal(new DnsRecord("DKIM", "agdir._domainkey", "TXT", "p=abc", "Auto", null, DomainStatus.Verified), created.Records[0]);
        Assert.Equal(new DnsRecord("SPF", "send", "MX", "feedback-smtp.eu-north-1.amazonses.com", "Auto", 10, DomainStatus.Pending), created.Records[1]);

        var fetched = await agdir.Domains.GetAsync("d_1");
        Assert.Equal(created.Records, fetched.Records);
        Assert.Equal(new ResourceRef("d_1"), await agdir.Domains.VerifyAsync("d_1"));
        Assert.Equal(new DeletedResource("d_1", true), await agdir.Domains.DeleteAsync("d_1"));

        Assert.Equal(
            new[] { "POST /domains", "GET /domains/d_1", "POST /domains/d_1/verify", "DELETE /domains/d_1" },
            handler.Requests.Select(r => $"{r.Method} {r.Url.AbsolutePath}"));
    }

    [Fact]
    public async Task ListsPagesAndIteratesEveryPage()
    {
        var handler = new FakeHandler()
            .Respond(new { @object = "list", has_more = true, data = new[] { DomainJson("d_1", "a.com"), DomainJson("d_2", "b.com") } })
            .Respond(new { @object = "list", has_more = false, data = new[] { DomainJson("d_3", "c.com") } })
            .Respond(new { @object = "list", has_more = false, data = Array.Empty<object>() });
        var agdir = TestClient.Create(handler);

        var names = new List<string>();
        await foreach (var domain in agdir.Domains.ListAllAsync(limit: 2)) names.Add(domain.Name);
        Assert.Equal(new[] { "a.com", "b.com", "c.com" }, names);
        Assert.Equal("?limit=2", handler.Requests[0].Url.Query);
        Assert.Equal("?limit=2&after=d_2", handler.Requests[1].Url.Query);

        var page = await agdir.Domains.ListAsync();
        Assert.False(page.HasMore);
        Assert.Empty(page.Data);
        Assert.Null(page.NextCursor);
        Assert.Equal("", handler.Requests[2].Url.Query);
    }

    [Fact]
    public async Task Webhooks()
    {
        var webhook = new
        {
            @object = "webhook",
            id = "wh_1",
            endpoint = "https://acme.com/hook",
            created_at = "2026-10-30T10:00:00Z",
            status = "enabled",
            events = new[] { "email.sent" },
            signing_secret = "whsec_abc",
        };
        var handler = new FakeHandler()
            .Respond(new { @object = "webhook", id = "wh_1", signing_secret = "whsec_abc" })
            .Respond(webhook)
            .Respond(new { @object = "list", has_more = false, data = new[] { webhook } })
            .Respond(new { @object = "webhook", id = "wh_1" })
            .Respond(new { @object = "webhook", id = "wh_1", signing_secret = "whsec_new" })
            .Respond(new { @object = "webhook", id = "wh_1", deleted = true });
        var agdir = TestClient.Create(handler);

        var created = await agdir.Webhooks.CreateAsync(new CreateWebhookRequest
        {
            Endpoint = "https://acme.com/hook",
            Events = { WebhookEventTypes.EmailSent, WebhookEventTypes.EmailDeliveryDelayed },
        });
        Assert.Equal(new WebhookSigningSecret("wh_1", "whsec_abc"), created);
        TestClient.AssertJson("""{"endpoint":"https://acme.com/hook","events":["email.sent","email.delivery_delayed"]}""", handler.Requests[0].Body);

        var fetched = await agdir.Webhooks.GetAsync("wh_1");
        Assert.Equal("whsec_abc", fetched.SigningSecret);
        Assert.Equal(WebhookStatus.Enabled, fetched.Status);
        Assert.Equal(new[] { "email.sent" }, fetched.Events);

        var page = await agdir.Webhooks.ListAsync(limit: 10, after: "wh_0");
        Assert.Equal("https://acme.com/hook", Assert.Single(page.Data).Endpoint);
        Assert.Equal("?limit=10&after=wh_0", handler.Requests[2].Url.Query);

        Assert.Equal(new ResourceRef("wh_1"), await agdir.Webhooks.UpdateAsync("wh_1", new UpdateWebhookRequest { Status = WebhookStatus.Disabled }));
        Assert.Equal("PATCH", handler.Requests[3].Method.Method);
        TestClient.AssertJson("""{"status":"disabled"}""", handler.Requests[3].Body);

        Assert.Equal("whsec_new", (await agdir.Webhooks.RotateSecretAsync("wh_1")).SigningSecret);
        Assert.Equal("/webhooks/wh_1/signing-secret/rotate", handler.Requests[4].Url.AbsolutePath);
        Assert.True((await agdir.Webhooks.DeleteAsync("wh_1")).Deleted);
        Assert.Equal("DELETE", handler.Requests[5].Method.Method);
    }

    [Fact]
    public async Task WebhookDeliveries()
    {
        var handler = new FakeHandler()
            .Respond(new { @object = "list", has_more = true, data = new[] { new { id = "ev_1", type = "email.sent", created_at = "2026-10-30T10:00:00Z", status = "attempting" } } })
            .Respond(new
            {
                @object = "webhook_event",
                id = "ev_1",
                type = "email.sent",
                created_at = "2026-10-30T10:00:00Z",
                status = "pending",
                next_attempt_at = "2026-10-30T10:05:00Z",
                payload = new { type = "email.sent", data = new { email_id = "em_1" } },
            })
            .Respond(new { @object = "list", has_more = false, data = new[] { new { id = "at_1", http_status_code = 0, response = "ECONNREFUSED", sent_at = "2026-10-30T10:00:01Z" } } })
            .Respond(new { @object = "webhook_event", id = "ev_1" });
        var deliveries = TestClient.Create(handler).Webhooks.Deliveries;

        var page = await deliveries.ListAsync("wh_1", limit: 1);
        Assert.Equal(WebhookDeliveryStatus.Attempting, page.Data[0].Status);
        Assert.Equal("ev_1", page.NextCursor);
        Assert.Equal("/webhooks/wh_1/events", handler.Requests[0].Url.AbsolutePath);

        var detail = await deliveries.GetAsync("wh_1", "ev_1");
        Assert.Equal(new DateTimeOffset(2026, 10, 30, 10, 5, 0, TimeSpan.Zero), detail.NextAttemptAt);
        Assert.Equal("em_1", detail.Payload.GetProperty("data").GetProperty("email_id").GetString());

        var attempts = await deliveries.ListAttemptsAsync("wh_1", "ev_1");
        Assert.Equal(new WebhookAttempt("at_1", 0, "ECONNREFUSED", new DateTimeOffset(2026, 10, 30, 10, 0, 1, TimeSpan.Zero)), attempts.Data[0]);
        Assert.Equal("/webhooks/wh_1/events/ev_1/attempts", handler.Requests[2].Url.AbsolutePath);

        Assert.Equal("ev_1", (await deliveries.ReplayAsync("wh_1", "ev_1")).Id);
        Assert.Equal("POST /webhooks/wh_1/events/ev_1/replay", $"{handler.Requests[3].Method} {handler.Requests[3].Url.AbsolutePath}");
    }
}
