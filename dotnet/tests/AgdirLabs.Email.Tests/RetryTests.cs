using System;
using System.Collections.Generic;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
using Xunit;

namespace AgdirLabs.Email.Tests;

public sealed class RetryTests
{
    private static SendEmailRequest Email() => new() { From = "a@acme.com", To = { "b@example.com" }, Subject = "s", Text = "t" };

    [Fact]
    public async Task ApiErrorsBecomeAgdirEmailException_WithoutRetrying422()
    {
        var handler = new FakeHandler().Error(422, "validation_error", "Invalid `to` field.");
        var error = await Assert.ThrowsAsync<AgdirEmailException>(() => TestClient.Create(handler).Emails.SendAsync(Email()));
        Assert.Equal(AgdirEmailErrorCodes.ValidationError, error.Code);
        Assert.Equal(422, error.StatusCode);
        Assert.Equal("Invalid `to` field.", error.Message);
        Assert.Null(error.RetryAfter);
        Assert.Single(handler.Requests);
    }

    [Fact]
    public async Task NonJsonErrorBodiesFallBackToTheStatusLine()
    {
        var handler = new FakeHandler().RespondRaw("<html>Bad gateway</html>", 502).RespondRaw("", 418);
        var agdir = TestClient.Create(handler, maxRetries: 0);
        var error = await Assert.ThrowsAsync<AgdirEmailException>(() => agdir.Domains.GetAsync("d_1"));
        Assert.Equal("Request failed with HTTP 502", error.Message);
        Assert.Equal(AgdirEmailErrorCodes.InternalServerError, error.Code);
        Assert.Equal(502, error.StatusCode);

        error = await Assert.ThrowsAsync<AgdirEmailException>(() => agdir.Domains.GetAsync("d_1"));
        Assert.Equal(AgdirEmailErrorCodes.UnknownError, error.Code);
    }

    [Fact]
    public async Task RetriesRateLimits_HonouringRetryAfter_EvenForNonIdempotentRequests()
    {
        var delays = new List<TimeSpan>();
        var handler = new FakeHandler()
            .Error(429, "rate_limit_exceeded", "Too many requests", new Dictionary<string, string> { ["retry-after"] = "1" })
            .Error(429, "rate_limit_exceeded", "Too many requests", new Dictionary<string, string> { ["retry-after"] = "0.25" })
            .Respond(new { id = "d_1", name = "acme.com", status = "not_started", created_at = "2026-10-30T10:00:00Z", records = Array.Empty<object>() });
        var domain = await TestClient.Create(handler, delays: delays).Domains.CreateAsync(new CreateDomainRequest { Name = "acme.com" });
        Assert.Equal("d_1", domain.Id);
        Assert.Equal(new[] { TimeSpan.FromSeconds(1), TimeSpan.FromMilliseconds(250) }, delays);
        Assert.Equal(3, handler.Requests.Count);
    }

    [Fact]
    public async Task GivesUpAfterMaxRetries_ReportingRetryAfter()
    {
        var handler = new FakeHandler()
            .Error(429, "rate_limit_exceeded", "slow down", new Dictionary<string, string> { ["retry-after"] = "2" })
            .Error(429, "rate_limit_exceeded", "slow down", new Dictionary<string, string> { ["retry-after"] = "2" });
        var error = await Assert.ThrowsAsync<AgdirEmailException>(() => TestClient.Create(handler, maxRetries: 1).Emails.GetAsync("em_1"));
        Assert.Equal(AgdirEmailErrorCodes.RateLimitExceeded, error.Code);
        Assert.Equal(TimeSpan.FromSeconds(2), error.RetryAfter);
        Assert.Equal(2, handler.Requests.Count);
    }

    [Theory]
    [InlineData("daily_quota_exceeded")]
    [InlineData("monthly_quota_exceeded")]
    public async Task NeverRetriesQuotaErrors(string code)
    {
        var handler = new FakeHandler().Error(429, code, "Quota reached");
        var error = await Assert.ThrowsAsync<AgdirEmailException>(() => TestClient.Create(handler).Emails.SendAsync(Email()));
        Assert.Equal(code, error.Code);
        Assert.Single(handler.Requests);
    }

    [Fact]
    public async Task RetriesServerErrorsForIdempotentRequestsOnly()
    {
        var delays = new List<TimeSpan>();
        var handler = new FakeHandler()
            .Error(503, "internal_server_error", "down")
            .Respond(new { id = "em_1" })
            .Error(503, "internal_server_error", "down");
        var agdir = TestClient.Create(handler, delays: delays);

        // A send carries a generated idempotency key, so it is retried with the same key.
        Assert.Equal("em_1", (await agdir.Emails.SendAsync(Email())).Id);
        Assert.Equal(handler.Requests[0].Header("Idempotency-Key"), handler.Requests[1].Header("Idempotency-Key"));
        var delay = Assert.Single(delays);
        Assert.InRange(delay, TimeSpan.FromMilliseconds(250), TimeSpan.FromMilliseconds(500));

        // Creating a domain is not idempotent.
        var error = await Assert.ThrowsAsync<AgdirEmailException>(() => agdir.Domains.CreateAsync(new CreateDomainRequest { Name = "acme.com" }));
        Assert.Equal(503, error.StatusCode);
        Assert.Equal(3, handler.Requests.Count);
    }

    [Fact]
    public async Task RetriesNetworkErrorsForGets_ThenReportsNetworkError()
    {
        var handler = new FakeHandler()
            .Throw(new HttpRequestException("connection refused"))
            .Throw(new HttpRequestException("connection refused"))
            .Throw(new HttpRequestException("connection refused"));
        var error = await Assert.ThrowsAsync<AgdirEmailException>(() => TestClient.Create(handler).Emails.GetAsync("em_1"));
        Assert.Equal(AgdirEmailErrorCodes.NetworkError, error.Code);
        Assert.Equal(0, error.StatusCode);
        Assert.IsType<HttpRequestException>(error.InnerException);
        Assert.Equal(3, handler.Requests.Count);
    }

    [Fact]
    public async Task DoesNotRetryNetworkErrorsForNonIdempotentRequests()
    {
        var handler = new FakeHandler().Throw(new HttpRequestException("reset"));
        var error = await Assert.ThrowsAsync<AgdirEmailException>(() => TestClient.Create(handler).Webhooks.RotateSecretAsync("wh_1"));
        Assert.Equal(AgdirEmailErrorCodes.NetworkError, error.Code);
        Assert.Single(handler.Requests);
    }

    [Fact]
    public async Task TimesOutEachAttempt()
    {
        var handler = new FakeHandler().Hang().Hang();
        var error = await Assert.ThrowsAsync<AgdirEmailException>(
            () => TestClient.Create(handler, maxRetries: 1, timeout: TimeSpan.FromMilliseconds(50)).Emails.GetAsync("em_1"));
        Assert.Equal(AgdirEmailErrorCodes.Timeout, error.Code);
        Assert.Equal(0, error.StatusCode);
        Assert.Contains("50 ms", error.Message);
        Assert.Equal(2, handler.Requests.Count);
    }

    [Fact]
    public async Task CallerCancellationIsNotWrapped()
    {
        var handler = new FakeHandler().Hang();
        using var cts = new CancellationTokenSource(TimeSpan.FromMilliseconds(50));
        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => TestClient.Create(handler).Emails.GetAsync("em_1", cts.Token));
        Assert.Single(handler.Requests);
    }

    [Fact]
    public void ParsesRetryAfterDates()
    {
        using var response = new HttpResponseMessage((System.Net.HttpStatusCode)429);
        response.Headers.TryAddWithoutValidation("Retry-After", DateTimeOffset.UtcNow.AddSeconds(30).ToString("R"));
        var wait = Internal.HttpTransport.RetryAfter(response);
        Assert.NotNull(wait);
        Assert.InRange(wait!.Value, TimeSpan.FromSeconds(28), TimeSpan.FromSeconds(31));
    }

    [Fact]
    public void BackoffGrowsExponentiallyWithJitterAndIsCapped()
    {
        for (var attempt = 0; attempt < 8; attempt++)
        {
            var max = Math.Min(8000, 500 * Math.Pow(2, attempt));
            var delay = Internal.HttpTransport.Backoff(attempt).TotalMilliseconds;
            Assert.InRange(delay, max / 2, max);
        }
    }
}
