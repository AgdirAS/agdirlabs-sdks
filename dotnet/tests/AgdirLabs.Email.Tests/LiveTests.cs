using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Xunit;

namespace AgdirLabs.Email.Tests;

/// <summary>
/// Runs against a real Agdir Email API, only when AGDIR_EMAIL_LIVE_BASE_URL and AGDIR_EMAIL_LIVE_API_KEY are set
/// (AGDIR_EMAIL_LIVE_FROM: a sender on a verified domain). Never point it at production.
/// </summary>
[Collection("environment")]
public sealed class LiveTests
{
    private static readonly string? BaseUrl = Env("AGDIR_EMAIL_LIVE_BASE_URL");
    private static readonly string? ApiKey = Env("AGDIR_EMAIL_LIVE_API_KEY");
    private static readonly string From = Env("AGDIR_EMAIL_LIVE_FROM") ?? "hi@dotnet.sdk-test.example.com";

    private static string? Env(string name) => Environment.GetEnvironmentVariable(name) is { Length: > 0 } v ? v : null;

    private static AgdirEmailClient Client(string? apiKey = null)
    {
        Assert.SkipWhen(BaseUrl is null || ApiKey is null, "Set AGDIR_EMAIL_LIVE_BASE_URL and AGDIR_EMAIL_LIVE_API_KEY to run live tests.");
        return new AgdirEmailClient(new AgdirEmailOptions { ApiKey = apiKey ?? ApiKey, BaseUrl = BaseUrl, MaxRetries = 5 });
    }

    private static SendEmailRequest Scheduled(string to = "user@example.com") => new()
    {
        From = From,
        To = { to },
        Subject = "Agdir Email .NET SDK live test",
        Text = "Hello from the .NET SDK",
        Tags = new[] { new EmailTag("sdk", "dotnet") },
        ScheduledAt = DateTimeOffset.UtcNow.AddHours(1),
    };

    [Fact]
    public async Task ScheduleGetCancel()
    {
        var agdir = Client();
        var sent = await agdir.Emails.SendAsync(Scheduled());
        var email = await agdir.Emails.GetAsync(sent.Id);
        Assert.Equal(EmailStatus.Scheduled, email.Status);
        Assert.Equal(new[] { "user@example.com" }, email.To);
        Assert.Empty(email.Cc);
        Assert.NotNull(email.ScheduledAt);

        Assert.Equal(sent.Id, (await agdir.Emails.CancelAsync(sent.Id)).Id);
        Assert.Equal(EmailStatus.Canceled, (await agdir.Emails.GetAsync(sent.Id)).Status);
    }

    [Fact]
    public async Task IdempotentReplayReturnsTheSameId()
    {
        var agdir = Client();
        var options = new SendEmailOptions { IdempotencyKey = "dotnet-live-" + Guid.NewGuid() };
        var email = Scheduled();
        var first = await agdir.Emails.SendAsync(email, options);
        var second = await agdir.Emails.SendAsync(email, options);
        Assert.Equal(first.Id, second.Id);
        await agdir.Emails.CancelAsync(first.Id);
    }

    [Fact]
    public async Task PermissiveBatchReportsTheBadRecipient()
    {
        var agdir = Client();
        var result = await agdir.Emails.SendBatchAsync(
            new[] { Scheduled(), Scheduled("not-an-email") },
            new SendBatchOptions { Validation = BatchValidation.Permissive });
        var accepted = Assert.Single(result.Data);
        Assert.Equal(1, Assert.Single(result.Errors).Index);
        await agdir.Emails.CancelAsync(accepted.Id);
    }

    [Fact]
    public async Task ValidationAndAuthErrors()
    {
        var error = await Assert.ThrowsAsync<AgdirEmailException>(() => Client().Emails.SendAsync(Scheduled("not-an-email")));
        Assert.Equal(AgdirEmailErrorCodes.ValidationError, error.Code);
        Assert.Equal(422, error.StatusCode);

        error = await Assert.ThrowsAsync<AgdirEmailException>(() => Client("re_invalid").Emails.GetAsync("00000000-0000-0000-0000-000000000000"));
        Assert.Equal(AgdirEmailErrorCodes.InvalidApiKey, error.Code);
        Assert.Equal(403, error.StatusCode);
    }

    [Fact]
    public async Task DomainLifecycle()
    {
        var agdir = Client();
        var name = $"dotnet-{DateTimeOffset.UtcNow.ToUnixTimeMilliseconds()}.test";
        var created = await agdir.Domains.CreateAsync(new CreateDomainRequest { Name = name });
        try
        {
            Assert.Equal(name, created.Name);
            Assert.NotEmpty(created.Records);
            Assert.Contains(created.Records, r => r.Purpose == "DKIM");

            var fetched = await agdir.Domains.GetAsync(created.Id);
            Assert.Equal(created.Id, fetched.Id);
            Assert.Equal("eu-north-1", fetched.Region);
            Assert.Equal(created.Id, (await agdir.Domains.VerifyAsync(created.Id)).Id);

            var page = await agdir.Domains.ListAsync(limit: 1);
            Assert.Single(page.Data);
            var all = new List<Domain>();
            await foreach (var domain in agdir.Domains.ListAllAsync(limit: 1)) all.Add(domain);
            Assert.Contains(all, d => d.Id == created.Id);
            Assert.Equal(all.Count, all.Select(d => d.Id).Distinct().Count());
        }
        finally
        {
            Assert.True((await agdir.Domains.DeleteAsync(created.Id)).Deleted);
        }

        var error = await Assert.ThrowsAsync<AgdirEmailException>(() => agdir.Domains.GetAsync(created.Id));
        Assert.Equal(AgdirEmailErrorCodes.NotFound, error.Code);
        Assert.Equal(404, error.StatusCode);
    }

    [Fact]
    public async Task WebhookLifecycle()
    {
        var agdir = Client();
        var created = await agdir.Webhooks.CreateAsync(new CreateWebhookRequest
        {
            Endpoint = "https://93.184.216.34/hook",
            Events = { WebhookEventTypes.EmailDelivered, WebhookEventTypes.EmailBounced },
        });
        try
        {
            Assert.StartsWith("whsec_", created.SigningSecret);
            var fetched = await agdir.Webhooks.GetAsync(created.Id);
            Assert.Equal("https://93.184.216.34/hook", fetched.Endpoint);
            Assert.Equal(WebhookStatus.Enabled, fetched.Status);
            Assert.Equal(created.SigningSecret, fetched.SigningSecret);
            Assert.Equal(new[] { "email.bounced", "email.delivered" }, fetched.Events.OrderBy(e => e, StringComparer.Ordinal));

            await agdir.Webhooks.UpdateAsync(created.Id, new UpdateWebhookRequest { Status = WebhookStatus.Disabled });
            Assert.Equal(WebhookStatus.Disabled, (await agdir.Webhooks.GetAsync(created.Id)).Status);

            var rotated = await agdir.Webhooks.RotateSecretAsync(created.Id);
            Assert.StartsWith("whsec_", rotated.SigningSecret);
            Assert.NotEqual(created.SigningSecret, rotated.SigningSecret);

            var deliveries = await agdir.Webhooks.Deliveries.ListAsync(created.Id);
            Assert.Empty(deliveries.Data);
            Assert.False(deliveries.HasMore);
        }
        finally
        {
            Assert.True((await agdir.Webhooks.DeleteAsync(created.Id)).Deleted);
        }
    }
}
