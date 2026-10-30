using System;
using System.Collections.Generic;
using System.Net.Http;
using System.Security.Cryptography;
using System.Text;
using Xunit;

namespace AgdirLabs.Email.Tests;

public sealed class WebhookVerifierTests
{
    private static readonly byte[] Key = Encoding.ASCII.GetBytes("0123456789abcdef0123456789abcdef");
    private static readonly string Secret = "whsec_" + Convert.ToBase64String(Key);
    private static readonly DateTimeOffset Now = new(2026, 10, 30, 12, 0, 0, TimeSpan.Zero);
    private static readonly string Timestamp = Now.ToUnixTimeSeconds().ToString();

    private const string Payload =
        """
        {"type":"email.bounced","created_at":"2026-10-30T11:59:58.000Z","data":{"created_at":"2026-10-30T11:59:00.000Z",
        "email_id":"em_1","message_id":"<m1@eu-north-1.amazonses.com>","from":"Acme <hi@acme.com>","to":["user@example.com"],
        "subject":"Hi","tags":{"kind":"welcome"},"bounce":{"message":"550 no such user","subType":"General","type":"Permanent"}}}
        """;

    private static string Sign(string id, string timestamp, string payload, byte[]? key = null)
    {
        using var hmac = new HMACSHA256(key ?? Key);
        return "v1," + Convert.ToBase64String(hmac.ComputeHash(Encoding.UTF8.GetBytes($"{id}.{timestamp}.{payload}")));
    }

    private static Dictionary<string, string> Headers(string? signature = null, string? timestamp = null) => new()
    {
        ["webhook-id"] = "msg_1",
        ["webhook-timestamp"] = timestamp ?? Timestamp,
        ["webhook-signature"] = signature ?? Sign("msg_1", timestamp ?? Timestamp, Payload),
    };

    [Fact]
    public void VerifiesAndParsesTheEvent()
    {
        var evt = WebhookVerifier.Verify(Payload, Headers(), Secret, now: Now);
        Assert.Equal(WebhookEventTypes.EmailBounced, evt.Type);
        Assert.Equal(new DateTimeOffset(2026, 10, 30, 11, 59, 58, TimeSpan.Zero), evt.CreatedAt);
        Assert.Equal("em_1", evt.Data.EmailId);
        Assert.Equal("<m1@eu-north-1.amazonses.com>", evt.Data.MessageId);
        Assert.Equal("Acme <hi@acme.com>", evt.Data.From);
        Assert.Equal(new[] { "user@example.com" }, evt.Data.To);
        Assert.Equal("welcome", evt.Data.Tags["kind"]);
        Assert.Equal(new WebhookBounce("Permanent", "General", "550 no such user"), evt.Data.Bounce);
        Assert.Null(evt.Data.Click);
        Assert.Null(evt.Data.Failed);
    }

    [Fact]
    public void ParsesClickEvents()
    {
        const string payload =
            """{"type":"email.clicked","created_at":"2026-10-30T11:59:58Z","data":{"created_at":"2026-10-30T11:59:00Z","email_id":"em_2","from":"a@acme.com","to":"b@example.com","subject":"s","click":{"ipAddress":"203.0.113.9","link":"https://acme.com/x","timestamp":"2026-10-30T11:59:57Z","userAgent":"Mozilla/5.0"}}}""";
        var headers = new Dictionary<string, string>
        {
            ["webhook-id"] = "msg_2",
            ["webhook-timestamp"] = Timestamp,
            ["webhook-signature"] = Sign("msg_2", Timestamp, payload),
        };
        var evt = WebhookVerifier.Verify(payload, headers, Secret, now: Now);
        Assert.Equal(new WebhookClick("https://acme.com/x", "203.0.113.9", "Mozilla/5.0", new DateTimeOffset(2026, 10, 30, 11, 59, 57, TimeSpan.Zero)), evt.Data.Click);
        Assert.Equal(new[] { "b@example.com" }, evt.Data.To);
        Assert.Null(evt.Data.MessageId);
        Assert.Empty(evt.Data.Tags);
    }

    [Fact]
    public void RejectsATamperedBody()
    {
        var error = Assert.Throws<WebhookVerificationException>(() => WebhookVerifier.Verify(Payload.Replace("em_1", "em_2"), Headers(), Secret, now: Now));
        Assert.Equal("Webhook signature does not match", error.Message);
    }

    [Fact]
    public void RejectsTheWrongSecret()
    {
        var other = "whsec_" + Convert.ToBase64String(Encoding.ASCII.GetBytes("another-secret-another-secret!!!"));
        Assert.Throws<WebhookVerificationException>(() => WebhookVerifier.Verify(Payload, Headers(), other, now: Now));
        var error = Assert.Throws<WebhookVerificationException>(() => WebhookVerifier.Verify(Payload, Headers(), "whsec_***", now: Now));
        Assert.Equal("Signing secret is not valid base64", error.Message);
    }

    [Theory]
    [InlineData("webhook-id")]
    [InlineData("webhook-timestamp")]
    [InlineData("webhook-signature")]
    public void RejectsMissingHeaders(string missing)
    {
        var headers = Headers();
        headers.Remove(missing);
        var error = Assert.Throws<WebhookVerificationException>(() => WebhookVerifier.Verify(Payload, headers, Secret, now: Now));
        Assert.Equal("Missing webhook signature headers", error.Message);
    }

    [Fact]
    public void RejectsStaleAndFutureTimestamps()
    {
        var stale = (Now.ToUnixTimeSeconds() - 301).ToString();
        var error = Assert.Throws<WebhookVerificationException>(() => WebhookVerifier.Verify(Payload, Headers(timestamp: stale), Secret, now: Now));
        Assert.Equal("Webhook timestamp is outside the allowed tolerance", error.Message);
        var future = (Now.ToUnixTimeSeconds() + 301).ToString();
        Assert.Throws<WebhookVerificationException>(() => WebhookVerifier.Verify(Payload, Headers(timestamp: future), Secret, now: Now));
        Assert.Throws<WebhookVerificationException>(() => WebhookVerifier.Verify(Payload, Headers(timestamp: "soon"), Secret, now: Now));

        // A wider tolerance accepts the stale message.
        WebhookVerifier.Verify(Payload, Headers(timestamp: stale), Secret, tolerance: TimeSpan.FromMinutes(10), now: Now);
    }

    [Fact]
    public void AcceptsAnyMatchingSignatureInAMultiSignatureHeader()
    {
        var rotatedOut = Sign("msg_1", Timestamp, Payload, Encoding.ASCII.GetBytes("old-key-old-key-old-key-old-key!"));
        var header = $"v2,ignored {rotatedOut} v1,not-base64!! {Sign("msg_1", Timestamp, Payload)}";
        Assert.Equal("em_1", WebhookVerifier.Verify(Payload, Headers(signature: header), Secret, now: Now).Data.EmailId);
        Assert.Throws<WebhookVerificationException>(() => WebhookVerifier.Verify(Payload, Headers(signature: rotatedOut), Secret, now: Now));
    }

    [Fact]
    public void AcceptsHttpHeadersCaseInsensitiveDictionariesAndCallbacks()
    {
        using var message = new HttpRequestMessage();
        foreach (var h in Headers()) message.Headers.TryAddWithoutValidation(h.Key, h.Value);
        Assert.Equal("em_1", WebhookVerifier.Verify(Payload, message.Headers, Secret, now: Now).Data.EmailId);

        var upper = new Dictionary<string, string>();
        foreach (var h in Headers()) upper[h.Key.ToUpperInvariant()] = h.Value;
        Assert.Equal("em_1", WebhookVerifier.Verify(Payload, upper, Secret, now: Now).Data.EmailId);

        var headers = Headers();
        Assert.Equal("em_1", WebhookVerifier.Verify(Payload, name => headers.TryGetValue(name, out var v) ? v : null, Secret, now: Now).Data.EmailId);
    }

    [Fact]
    public void AcceptsASecretWithoutThePrefix_AndRejectsInvalidJson()
    {
        Assert.Equal("em_1", WebhookVerifier.Verify(Payload, Headers(), Convert.ToBase64String(Key), now: Now).Data.EmailId);
        var headers = new Dictionary<string, string>
        {
            ["webhook-id"] = "msg_1",
            ["webhook-timestamp"] = Timestamp,
            ["webhook-signature"] = Sign("msg_1", Timestamp, "not json"),
        };
        var error = Assert.Throws<WebhookVerificationException>(() => WebhookVerifier.Verify("not json", headers, Secret, now: Now));
        Assert.Equal("Webhook payload is not valid JSON", error.Message);
    }
}
