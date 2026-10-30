using System;
using System.Net.Http;
using AgdirLabs.Email.Internal;

namespace AgdirLabs.Email;

/// <summary>The Agdir Email API: emails, domains and webhooks.</summary>
public interface IAgdirEmailClient
{
    /// <summary>Send, read and cancel emails.</summary>
    IEmailsResource Emails { get; }

    /// <summary>Manage sending domains.</summary>
    IDomainsResource Domains { get; }

    /// <summary>Manage webhook endpoints and their deliveries.</summary>
    IWebhooksResource Webhooks { get; }
}

/// <summary>
/// Agdir Email API client. Thread-safe and cheap to create: without an explicit <see cref="HttpClient"/>,
/// every instance shares one pooled connection handler.
/// </summary>
/// <example>
/// <code>
/// var agdir = new AgdirEmailClient("re_…");
/// var result = await agdir.Emails.SendAsync(new SendEmailRequest
/// {
///     From = "Acme &lt;hello@acme.com&gt;",
///     To = { "user@example.com" },
///     Subject = "Welcome",
///     Html = "&lt;p&gt;Thanks for signing up!&lt;/p&gt;",
/// });
/// </code>
/// </example>
public sealed class AgdirEmailClient : IAgdirEmailClient
{
    /// <summary>This SDK's version.</summary>
    public const string Version = "0.1.0";

    /// <summary>The <c>User-Agent</c> sent with every request.</summary>
    public const string UserAgent = "agdirlabs-email-dotnet/" + Version;

    private readonly HttpTransport _http;

    /// <summary>Creates a client with an API key (default: the <c>AGDIR_EMAIL_API_KEY</c> environment variable).</summary>
    /// <param name="apiKey">API key, <c>re_…</c>.</param>
    /// <exception cref="InvalidOperationException">No API key was given or found in the environment.</exception>
    public AgdirEmailClient(string? apiKey = null)
        : this(new AgdirEmailOptions { ApiKey = apiKey })
    {
    }

    /// <summary>Creates a client from options, optionally on your own <see cref="HttpClient"/>.</summary>
    /// <param name="options">API key, base URL, retries and timeout.</param>
    /// <param name="httpClient">
    /// An <see cref="HttpClient"/> to send requests with (proxies, tests, <c>IHttpClientFactory</c>); this client never disposes it.
    /// Its own <see cref="HttpClient.Timeout"/> still applies on top of <see cref="AgdirEmailOptions.Timeout"/>.
    /// </param>
    /// <exception cref="InvalidOperationException">No API key was given or found in the environment.</exception>
    /// <exception cref="ArgumentOutOfRangeException"><see cref="AgdirEmailOptions.MaxRetries"/> is negative or the timeout is not positive.</exception>
    public AgdirEmailClient(AgdirEmailOptions options, HttpClient? httpClient = null)
    {
        if (options is null) throw new ArgumentNullException(nameof(options));
        var apiKey = NonEmpty(options.ApiKey) ?? Env("AGDIR_EMAIL_API_KEY")
            ?? throw new InvalidOperationException(
                "Missing API key: pass one to AgdirEmailClient or set AGDIR_EMAIL_API_KEY. Create one at https://app.agdir.email.");
        if (options.MaxRetries < 0)
        {
            throw new ArgumentOutOfRangeException(nameof(options), "MaxRetries must be a non-negative integer.");
        }
        if (options.Timeout <= TimeSpan.Zero && options.Timeout != System.Threading.Timeout.InfiniteTimeSpan)
        {
            throw new ArgumentOutOfRangeException(nameof(options), "Timeout must be positive.");
        }

        BaseUrl = NonEmpty(options.BaseUrl) ?? Env("AGDIR_EMAIL_BASE_URL") ?? AgdirEmailOptions.DefaultBaseUrl;
        if (!Uri.TryCreate(BaseUrl, UriKind.Absolute, out _))
        {
            throw new ArgumentException($"BaseUrl must be an absolute URL, got \"{BaseUrl}\".", nameof(options));
        }
        _http = new HttpTransport(apiKey, BaseUrl, options.MaxRetries, options.Timeout, httpClient);
        Emails = new EmailsResource(_http);
        Domains = new DomainsResource(_http);
        Webhooks = new WebhooksResource(_http);
    }

    /// <summary>The API base URL in use.</summary>
    public string BaseUrl { get; }

    /// <inheritdoc />
    public IEmailsResource Emails { get; }

    /// <inheritdoc />
    public IDomainsResource Domains { get; }

    /// <inheritdoc />
    public IWebhooksResource Webhooks { get; }

    internal HttpTransport Transport => _http;

    // Empty values (e.g. `AGDIR_EMAIL_BASE_URL=` in a .env file) count as unset.
    private static string? NonEmpty(string? value) => string.IsNullOrWhiteSpace(value) ? null : value;

    private static string? Env(string name) => NonEmpty(Environment.GetEnvironmentVariable(name));
}
