using System;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Net.Http;
using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;

namespace AgdirLabs.Email.Internal;

internal sealed class ApiRequest
{
    public ApiRequest(HttpMethod method, string path)
    {
        Method = method;
        Path = path;
    }

    public HttpMethod Method { get; }
    public string Path { get; }
    public IReadOnlyList<KeyValuePair<string, string?>>? Query { get; init; }
    public byte[]? Body { get; init; }
    public IReadOnlyList<KeyValuePair<string, string?>>? Headers { get; init; }

    /// <summary>Safe to repeat: GETs, and POSTs that carry an idempotency key.</summary>
    public bool Retryable { get; init; }
}

/// <summary>Sends API requests, retrying rate limits, server errors and network failures when safe.</summary>
internal sealed class HttpTransport
{
    internal static readonly HttpMethod Patch = new("PATCH");
    private static readonly HashSet<int> RetryStatuses = new() { 408, 500, 502, 503, 504 };
    private static readonly Random Jitter = new();

    // One shared HttpClient for every client that doesn't bring its own, as .NET recommends:
    // creating clients per request then never exhausts sockets.
    private static readonly Lazy<HttpClient> SharedHttp = new(CreateDefaultHttpClient);

    private readonly HttpClient _http;
    private readonly string _apiKey;
    private readonly string _baseUrl;

    public HttpTransport(string apiKey, string baseUrl, int maxRetries, TimeSpan timeout, HttpClient? http)
    {
        _apiKey = apiKey;
        _baseUrl = baseUrl.TrimEnd('/');
        MaxRetries = maxRetries;
        Timeout = timeout;
        _http = http ?? SharedHttp.Value;
    }

    public int MaxRetries { get; }
    public TimeSpan Timeout { get; }

    /// <summary>Replaced in tests to avoid real sleeps.</summary>
    internal Func<TimeSpan, CancellationToken, Task> Delay { get; set; } = Task.Delay;

    private static HttpClient CreateDefaultHttpClient()
    {
#if NET8_0_OR_GREATER
        // Recycle connections so DNS changes are picked up by long-lived clients.
        var handler = new SocketsHttpHandler { PooledConnectionLifetime = TimeSpan.FromMinutes(5) };
        return new HttpClient(handler) { Timeout = System.Threading.Timeout.InfiniteTimeSpan };
#else
        return new HttpClient { Timeout = System.Threading.Timeout.InfiniteTimeSpan };
#endif
    }

    /// <summary>Exponential backoff with jitter: ~0.5 s, 1 s, 2 s … capped at 8 s.</summary>
    internal static TimeSpan Backoff(int attempt)
    {
        var baseMs = Math.Min(8000, 500 * Math.Pow(2, attempt));
        double r;
        lock (Jitter) r = Jitter.NextDouble();
        return TimeSpan.FromMilliseconds(baseMs / 2 + r * baseMs / 2);
    }

    public async Task<T> SendAsync<T>(ApiRequest request, Func<JsonElement, T> map, CancellationToken cancellationToken)
    {
        var url = BuildUrl(request);
        for (var attempt = 0; ; attempt++)
        {
            var canRetry = attempt < MaxRetries;
            using var message = BuildMessage(request, url);
            using var attemptCts = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
            if (Timeout != System.Threading.Timeout.InfiniteTimeSpan) attemptCts.CancelAfter(Timeout);

            HttpResponseMessage response;
            string text;
            try
            {
                response = await _http.SendAsync(message, HttpCompletionOption.ResponseHeadersRead, attemptCts.Token).ConfigureAwait(false);
                try
                {
                    text = await ReadAsync(response.Content, attemptCts.Token).ConfigureAwait(false);
                }
                catch
                {
                    response.Dispose();
                    throw;
                }
            }
            catch (Exception ex) when (!cancellationToken.IsCancellationRequested && (ex is OperationCanceledException || ex is HttpRequestException))
            {
                var timedOut = ex is OperationCanceledException;
                // A timed-out POST may have been accepted: only retry it when it is idempotent.
                if (canRetry && request.Retryable)
                {
                    await Delay(Backoff(attempt), cancellationToken).ConfigureAwait(false);
                    continue;
                }
                throw timedOut
                    ? new AgdirEmailException($"Request timed out after {Timeout.TotalMilliseconds:0} ms", AgdirEmailErrorCodes.Timeout, 0, null, ex)
                    : new AgdirEmailException("Could not reach the Agdir Email API", AgdirEmailErrorCodes.NetworkError, 0, null, ex);
            }

            using (response)
            {
                var status = (int)response.StatusCode;
                if (response.IsSuccessStatusCode)
                {
                    try
                    {
                        using var doc = JsonDocument.Parse(text.Length == 0 ? "{}" : text);
                        return map(doc.RootElement);
                    }
                    catch (JsonException ex)
                    {
                        throw new AgdirEmailException($"Invalid JSON in API response (HTTP {status})", AgdirEmailErrorCodes.UnknownError, status, null, ex);
                    }
                }

                var error = ToError(response, text);
                // Rate limits are refused before any work is done, so they are always safe to retry.
                var retry = canRetry && (error.Code == AgdirEmailErrorCodes.RateLimitExceeded || (request.Retryable && RetryStatuses.Contains(status)));
                if (!retry) throw error;
                await Delay(error.RetryAfter ?? Backoff(attempt), cancellationToken).ConfigureAwait(false);
            }
        }
    }

    private string BuildUrl(ApiRequest request)
    {
        var sb = new StringBuilder(_baseUrl).Append(request.Path);
        var first = true;
        foreach (var pair in request.Query ?? Array.Empty<KeyValuePair<string, string?>>())
        {
            if (pair.Value is null) continue;
            sb.Append(first ? '?' : '&').Append(Uri.EscapeDataString(pair.Key)).Append('=').Append(Uri.EscapeDataString(pair.Value));
            first = false;
        }
        return sb.ToString();
    }

    private HttpRequestMessage BuildMessage(ApiRequest request, string url)
    {
        var message = new HttpRequestMessage(request.Method, url);
        message.Headers.TryAddWithoutValidation("Authorization", "Bearer " + _apiKey);
        message.Headers.TryAddWithoutValidation("User-Agent", AgdirEmailClient.UserAgent);
        message.Headers.Accept.Add(new MediaTypeWithQualityHeaderValue("application/json"));
        foreach (var header in request.Headers ?? Array.Empty<KeyValuePair<string, string?>>())
        {
            if (header.Value is not null) message.Headers.TryAddWithoutValidation(header.Key, header.Value);
        }
        if (request.Body is not null)
        {
            message.Content = new ByteArrayContent(request.Body);
            message.Content.Headers.ContentType = new MediaTypeHeaderValue("application/json");
        }
        return message;
    }

    private static Task<string> ReadAsync(HttpContent content, CancellationToken cancellationToken)
    {
#if NET8_0_OR_GREATER
        return content.ReadAsStringAsync(cancellationToken);
#else
        return content.ReadAsStringAsync();
#endif
    }

    private static AgdirEmailException ToError(HttpResponseMessage response, string text)
    {
        var status = (int)response.StatusCode;
        string? name = null, message = null;
        try
        {
            using var doc = JsonDocument.Parse(text);
            name = doc.RootElement.OptString("name");
            message = doc.RootElement.OptString("message");
        }
        catch (JsonException)
        {
            // Not JSON (e.g. a proxy error page) — fall back to the status line.
        }
        return new AgdirEmailException(
            message ?? $"Request failed with HTTP {status}",
            name ?? (status >= 500 ? AgdirEmailErrorCodes.InternalServerError : AgdirEmailErrorCodes.UnknownError),
            status,
            RetryAfter(response));
    }

    internal static TimeSpan? RetryAfter(HttpResponseMessage response)
    {
        string? raw = null;
#if NET8_0_OR_GREATER
        if (response.Headers.NonValidated.TryGetValues("Retry-After", out var values)) raw = values.FirstOrDefault();
#else
        if (response.Headers.TryGetValues("Retry-After", out var values)) raw = values.FirstOrDefault();
#endif
        if (raw is null && response.Headers.RetryAfter is { } typed)
        {
            if (typed.Delta is { } delta) return delta;
            raw = typed.Date?.ToString("R", CultureInfo.InvariantCulture);
        }
        raw = raw?.Trim();
        if (string.IsNullOrEmpty(raw)) return null;
        if (double.TryParse(raw, NumberStyles.Float, CultureInfo.InvariantCulture, out var seconds))
        {
            return TimeSpan.FromSeconds(Math.Max(0, seconds));
        }
        if (DateTimeOffset.TryParse(raw, CultureInfo.InvariantCulture, DateTimeStyles.AssumeUniversal, out var date))
        {
            var wait = date - DateTimeOffset.UtcNow;
            return wait < TimeSpan.Zero ? TimeSpan.Zero : wait;
        }
        return null;
    }
}
