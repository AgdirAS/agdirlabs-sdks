using System;

namespace AgdirLabs.Email;

/// <summary>Thrown by every client method when a request fails.</summary>
public class AgdirEmailException : Exception
{
    /// <summary>Creates an exception.</summary>
    /// <param name="message">Human-readable explanation.</param>
    /// <param name="code">Machine-readable code; see <see cref="AgdirEmailErrorCodes"/>.</param>
    /// <param name="statusCode">HTTP status, or 0 when no response was received.</param>
    /// <param name="retryAfter">How long the API asked us to wait, if it did.</param>
    /// <param name="innerException">The underlying exception, if any.</param>
    public AgdirEmailException(string message, string code, int statusCode, TimeSpan? retryAfter = null, Exception? innerException = null)
        : base(message, innerException)
    {
        Code = code;
        StatusCode = statusCode;
        RetryAfter = retryAfter;
    }

    /// <summary>Machine-readable code, e.g. <c>validation_error</c> or <c>rate_limit_exceeded</c>; see <see cref="AgdirEmailErrorCodes"/>.</summary>
    public string Code { get; }

    /// <summary>HTTP status, or 0 when no response was received.</summary>
    public int StatusCode { get; }

    /// <summary>How long the API asked us to wait (from <c>retry-after</c>), if it did.</summary>
    public TimeSpan? RetryAfter { get; }
}

/// <summary>Error codes returned by the Agdir Email API, plus the client-side <see cref="NetworkError"/> and <see cref="Timeout"/>.</summary>
public static class AgdirEmailErrorCodes
{
    /// <summary>No API key was sent (401).</summary>
    public const string MissingApiKey = "missing_api_key";
    /// <summary>The API key is not valid (403).</summary>
    public const string InvalidApiKey = "invalid_api_key";
    /// <summary>A sending-only key was used on an endpoint that needs full access (401).</summary>
    public const string RestrictedApiKey = "restricted_api_key";
    /// <summary>The request is invalid (422), or the domain is unverified, the team paused or a limit reached (403).</summary>
    public const string ValidationError = "validation_error";
    /// <summary>The object does not exist (404).</summary>
    public const string NotFound = "not_found";
    /// <summary>The idempotency key was reused with a different body (409).</summary>
    public const string InvalidIdempotentRequest = "invalid_idempotent_request";
    /// <summary>A request with the same idempotency key is still in progress (409).</summary>
    public const string ConcurrentIdempotentRequests = "concurrent_idempotent_requests";
    /// <summary>The daily sending quota is used up (429). Not retried.</summary>
    public const string DailyQuotaExceeded = "daily_quota_exceeded";
    /// <summary>The monthly quota or spend limit is reached (429). Not retried.</summary>
    public const string MonthlyQuotaExceeded = "monthly_quota_exceeded";
    /// <summary>Too many requests per second (429). Retried automatically.</summary>
    public const string RateLimitExceeded = "rate_limit_exceeded";
    /// <summary>The API failed (5xx).</summary>
    public const string InternalServerError = "internal_server_error";
    /// <summary>An error response without a recognizable body.</summary>
    public const string UnknownError = "unknown_error";
    /// <summary>The API could not be reached (status 0).</summary>
    public const string NetworkError = "network_error";
    /// <summary>The request exceeded <see cref="AgdirEmailOptions.Timeout"/> (status 0).</summary>
    public const string Timeout = "timeout";
}

/// <summary>Thrown by <see cref="WebhookVerifier"/> when a payload's signature or timestamp is invalid.</summary>
public sealed class WebhookVerificationException : Exception
{
    /// <summary>Creates an exception.</summary>
    /// <param name="message">What did not match.</param>
    /// <param name="innerException">The underlying exception, if any.</param>
    public WebhookVerificationException(string message, Exception? innerException = null)
        : base(message, innerException)
    {
    }
}
