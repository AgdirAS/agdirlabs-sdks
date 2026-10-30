using System;

namespace AgdirLabs.Email;

/// <summary>Configuration for <see cref="AgdirEmailClient"/>.</summary>
public sealed class AgdirEmailOptions
{
    /// <summary>The default API base URL, <c>https://api.agdir.email</c>.</summary>
    public const string DefaultBaseUrl = "https://api.agdir.email";

    /// <summary>The configuration section read by <c>services.AddAgdirEmail()</c>.</summary>
    public const string SectionName = "AgdirEmail";

    /// <summary>API key (<c>re_…</c>). Defaults to the <c>AGDIR_EMAIL_API_KEY</c> environment variable.</summary>
    public string? ApiKey { get; set; }

    /// <summary>API base URL. Defaults to <c>AGDIR_EMAIL_BASE_URL</c>, else <see cref="DefaultBaseUrl"/>.</summary>
    public string? BaseUrl { get; set; }

    /// <summary>Retries for rate limits, server errors and network failures. Default 2; 0 disables.</summary>
    public int MaxRetries { get; set; } = 2;

    /// <summary>Timeout per attempt. Default 30 seconds.</summary>
    public TimeSpan Timeout { get; set; } = TimeSpan.FromSeconds(30);
}
