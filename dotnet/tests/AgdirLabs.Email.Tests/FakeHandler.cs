using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Linq;
using System.Net;
using System.Net.Http;
using System.Text;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;

namespace AgdirLabs.Email.Tests;

/// <summary>A recorded request.</summary>
internal sealed record RecordedRequest(HttpMethod Method, Uri Url, Dictionary<string, string> Headers, string? Body)
{
    public string? Header(string name) =>
        Headers.FirstOrDefault(h => string.Equals(h.Key, name, StringComparison.OrdinalIgnoreCase)).Value;

    public JsonElement Json => JsonDocument.Parse(Body ?? "null").RootElement;
}

/// <summary>Replays scripted responses and records every request.</summary>
internal sealed class FakeHandler : HttpMessageHandler
{
    private readonly ConcurrentQueue<Func<HttpRequestMessage, CancellationToken, Task<HttpResponseMessage>>> _responses = new();

    public List<RecordedRequest> Requests { get; } = new();

    public FakeHandler Respond(object body, int status = 200, IDictionary<string, string>? headers = null) =>
        RespondRaw(JsonSerializer.Serialize(body), status, headers);

    public FakeHandler RespondRaw(string body, int status = 200, IDictionary<string, string>? headers = null)
    {
        _responses.Enqueue((_, _) =>
        {
            var response = new HttpResponseMessage((HttpStatusCode)status) { Content = new StringContent(body, Encoding.UTF8, "application/json") };
            foreach (var header in headers ?? new Dictionary<string, string>()) response.Headers.TryAddWithoutValidation(header.Key, header.Value);
            return Task.FromResult(response);
        });
        return this;
    }

    public FakeHandler Error(int status, string name, string message, IDictionary<string, string>? headers = null) =>
        Respond(new { statusCode = status, name, message }, status, headers);

    public FakeHandler Throw(Exception exception)
    {
        _responses.Enqueue((_, _) => throw exception);
        return this;
    }

    public FakeHandler Hang()
    {
        _responses.Enqueue(async (_, ct) =>
        {
            await Task.Delay(Timeout.Infinite, ct);
            throw new InvalidOperationException("unreachable");
        });
        return this;
    }

    protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
    {
        var headers = request.Headers.ToDictionary(h => h.Key, h => string.Join(", ", h.Value), StringComparer.OrdinalIgnoreCase);
        string? body = null;
        if (request.Content is not null)
        {
            body = await request.Content.ReadAsStringAsync();
            foreach (var h in request.Content.Headers) headers[h.Key] = string.Join(", ", h.Value);
        }
        lock (Requests) Requests.Add(new RecordedRequest(request.Method, request.RequestUri!, headers, body));
        if (!_responses.TryDequeue(out var next)) throw new InvalidOperationException($"Unexpected request {request.Method} {request.RequestUri}");
        return await next(request, cancellationToken);
    }
}

internal static class TestClient
{
    public static AgdirEmailClient Create(FakeHandler handler, int maxRetries = 2, TimeSpan? timeout = null, List<TimeSpan>? delays = null)
    {
        var client = new AgdirEmailClient(
            new AgdirEmailOptions
            {
                ApiKey = "re_test",
                BaseUrl = "https://api.test/",
                MaxRetries = maxRetries,
                Timeout = timeout ?? TimeSpan.FromSeconds(30),
            },
            new HttpClient(handler));
        client.Transport.Delay = (delay, _) =>
        {
            delays?.Add(delay);
            return Task.CompletedTask;
        };
        return client;
    }

    public static void AssertJson(string expected, string? actual)
    {
        var normalize = (string s) => JsonSerializer.Serialize(JsonDocument.Parse(s).RootElement);
        Xunit.Assert.Equal(normalize(expected), normalize(actual ?? "null"));
    }
}
