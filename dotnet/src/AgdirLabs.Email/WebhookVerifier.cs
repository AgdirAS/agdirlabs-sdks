using System;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Net.Http.Headers;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using AgdirLabs.Email.Internal;

namespace AgdirLabs.Email;

/// <summary>
/// Checks that a webhook request came from Agdir Email. Messages are signed per the Standard Webhooks
/// spec (<c>webhook-id</c>, <c>webhook-timestamp</c>, <c>webhook-signature</c> headers).
/// </summary>
public static class WebhookVerifier
{
    /// <summary>Default maximum age (and clock skew) of a message: 5 minutes.</summary>
    public static readonly TimeSpan DefaultTolerance = TimeSpan.FromMinutes(5);

    /// <summary>Verifies a webhook request and returns the parsed event.</summary>
    /// <param name="payload">The raw request body, exactly as received — not re-serialized JSON.</param>
    /// <param name="headers">Request headers (names are matched case-insensitively).</param>
    /// <param name="secret">The webhook's <c>whsec_…</c> signing secret.</param>
    /// <param name="tolerance">Reject messages older or newer than this. Default 5 minutes.</param>
    /// <param name="now">The current time (for tests).</param>
    /// <exception cref="WebhookVerificationException">The signature, timestamp or payload is invalid.</exception>
    public static WebhookEvent Verify(string payload, IDictionary<string, string> headers, string secret, TimeSpan? tolerance = null, DateTimeOffset? now = null)
    {
        if (headers is null) throw new ArgumentNullException(nameof(headers));
        return Verify(payload, name => Lookup(headers, name), secret, tolerance, now);
    }

    /// <summary>Verifies a webhook request and returns the parsed event.</summary>
    /// <param name="payload">The raw request body, exactly as received — not re-serialized JSON.</param>
    /// <param name="headers">Request headers, e.g. <c>HttpRequestMessage.Headers</c>.</param>
    /// <param name="secret">The webhook's <c>whsec_…</c> signing secret.</param>
    /// <param name="tolerance">Reject messages older or newer than this. Default 5 minutes.</param>
    /// <param name="now">The current time (for tests).</param>
    /// <exception cref="WebhookVerificationException">The signature, timestamp or payload is invalid.</exception>
    public static WebhookEvent Verify(string payload, HttpHeaders headers, string secret, TimeSpan? tolerance = null, DateTimeOffset? now = null)
    {
        if (headers is null) throw new ArgumentNullException(nameof(headers));
        return Verify(
            payload,
            name => headers.TryGetValues(name, out var values) ? string.Join(" ", values) : null,
            secret,
            tolerance,
            now);
    }

    /// <summary>
    /// Verifies a webhook request and returns the parsed event, reading headers through a callback —
    /// e.g. ASP.NET Core: <c>WebhookVerifier.Verify(body, name =&gt; Request.Headers[name], secret)</c>.
    /// </summary>
    /// <param name="payload">The raw request body, exactly as received — not re-serialized JSON.</param>
    /// <param name="getHeader">Returns a header's value, or <see langword="null"/>/empty when absent.</param>
    /// <param name="secret">The webhook's <c>whsec_…</c> signing secret.</param>
    /// <param name="tolerance">Reject messages older or newer than this. Default 5 minutes.</param>
    /// <param name="now">The current time (for tests).</param>
    /// <exception cref="WebhookVerificationException">The signature, timestamp or payload is invalid.</exception>
    public static WebhookEvent Verify(string payload, Func<string, string?> getHeader, string secret, TimeSpan? tolerance = null, DateTimeOffset? now = null)
    {
        if (payload is null) throw new ArgumentNullException(nameof(payload));
        if (getHeader is null) throw new ArgumentNullException(nameof(getHeader));
        if (string.IsNullOrEmpty(secret)) throw new ArgumentException("A signing secret is required.", nameof(secret));

        var id = getHeader("webhook-id");
        var timestamp = getHeader("webhook-timestamp");
        var signatures = getHeader("webhook-signature");
        if (string.IsNullOrEmpty(id) || string.IsNullOrEmpty(timestamp) || string.IsNullOrEmpty(signatures))
        {
            throw new WebhookVerificationException("Missing webhook signature headers");
        }

        var currentTime = (now ?? DateTimeOffset.UtcNow).ToUnixTimeSeconds();
        var maxSkew = (long)(tolerance ?? DefaultTolerance).TotalSeconds;
        if (!long.TryParse(timestamp, NumberStyles.None, CultureInfo.InvariantCulture, out var sentAt) || Math.Abs(currentTime - sentAt) > maxSkew)
        {
            throw new WebhookVerificationException("Webhook timestamp is outside the allowed tolerance");
        }

        const string prefix = "whsec_";
        byte[] key;
        try
        {
            key = Convert.FromBase64String(secret.StartsWith(prefix, StringComparison.Ordinal) ? secret.Substring(prefix.Length) : secret);
        }
        catch (FormatException ex)
        {
            throw new WebhookVerificationException("Signing secret is not valid base64", ex);
        }

        byte[] expected;
        using (var hmac = new HMACSHA256(key))
        {
            expected = hmac.ComputeHash(Encoding.UTF8.GetBytes($"{id}.{timestamp}.{payload}"));
        }

        // The header may hold several space-separated `v1,<sig>` entries (e.g. during secret rotation).
        var valid = signatures!
            .Split(new[] { ' ' }, StringSplitOptions.RemoveEmptyEntries)
            .Where(entry => entry.StartsWith("v1,", StringComparison.Ordinal))
            .Any(entry => FixedTimeEquals(TryBase64(entry.Substring(3)), expected));
        if (!valid) throw new WebhookVerificationException("Webhook signature does not match");

        try
        {
            using var doc = JsonDocument.Parse(payload);
            return Wire.WebhookEvent(doc.RootElement);
        }
        catch (Exception ex) when (ex is JsonException || ex is FormatException)
        {
            throw new WebhookVerificationException("Webhook payload is not valid JSON", ex);
        }
    }

    private static string? Lookup(IDictionary<string, string> headers, string name)
    {
        if (headers.TryGetValue(name, out var value)) return value;
        foreach (var pair in headers)
        {
            if (string.Equals(pair.Key, name, StringComparison.OrdinalIgnoreCase)) return pair.Value;
        }
        return null;
    }

    private static byte[]? TryBase64(string value)
    {
        try
        {
            return Convert.FromBase64String(value);
        }
        catch (FormatException)
        {
            return null;
        }
    }

    private static bool FixedTimeEquals(byte[]? a, byte[] b)
    {
        if (a is null || a.Length != b.Length) return false;
#if NET8_0_OR_GREATER
        return CryptographicOperations.FixedTimeEquals(a, b);
#else
        var diff = 0;
        for (var i = 0; i < a.Length; i++) diff |= a[i] ^ b[i];
        return diff == 0;
#endif
    }
}
