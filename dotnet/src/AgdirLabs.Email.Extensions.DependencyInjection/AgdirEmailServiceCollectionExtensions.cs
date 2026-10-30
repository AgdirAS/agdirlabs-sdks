using System;
using System.Net.Http;
using AgdirLabs.Email;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection.Extensions;
using Microsoft.Extensions.Options;

// Extension methods live in the DI namespace so they show up next to services.AddHttpClient() etc.
namespace Microsoft.Extensions.DependencyInjection;

/// <summary>Registers the Agdir Email client.</summary>
public static class AgdirEmailServiceCollectionExtensions
{
    /// <summary>The name of the <see cref="HttpClient"/> used by the client (for <c>IHttpClientFactory</c>).</summary>
    public const string HttpClientName = "AgdirEmail";

    /// <summary>
    /// Registers <see cref="IAgdirEmailClient"/> (and <see cref="AgdirEmailClient"/>) backed by <c>IHttpClientFactory</c>.
    /// Options are bound from the <c>AgdirEmail</c> configuration section when an <see cref="IConfiguration"/> is registered
    /// (e.g. <c>"AgdirEmail": { "ApiKey": "re_…" }</c> or the <c>AgdirEmail__ApiKey</c> environment variable),
    /// then <paramref name="configure"/> runs; an unset key falls back to <c>AGDIR_EMAIL_API_KEY</c>.
    /// </summary>
    /// <param name="services">The service collection.</param>
    /// <param name="configure">Optional extra configuration.</param>
    /// <returns>The <see cref="IHttpClientBuilder"/> for the client's <see cref="HttpClient"/>, to add handlers or resilience.</returns>
    public static IHttpClientBuilder AddAgdirEmail(this IServiceCollection services, Action<AgdirEmailOptions>? configure = null)
    {
        if (services is null) throw new ArgumentNullException(nameof(services));

        services.AddOptions();
        services.TryAddEnumerable(ServiceDescriptor.Singleton<IConfigureOptions<AgdirEmailOptions>, BindAgdirEmailSection>());
        if (configure is not null) services.Configure(configure);

        var builder = services.AddHttpClient(HttpClientName, http => http.Timeout = System.Threading.Timeout.InfiniteTimeSpan);

        // Transient on top of the factory's pooled handlers, as recommended for typed clients.
        services.TryAddTransient(sp => new AgdirEmailClient(
            sp.GetRequiredService<IOptions<AgdirEmailOptions>>().Value,
            sp.GetRequiredService<IHttpClientFactory>().CreateClient(HttpClientName)));
        services.TryAddTransient<IAgdirEmailClient>(sp => sp.GetRequiredService<AgdirEmailClient>());
        return builder;
    }

    private sealed class BindAgdirEmailSection : IConfigureOptions<AgdirEmailOptions>
    {
        private readonly IConfiguration? _configuration;

        public BindAgdirEmailSection(IServiceProvider services) =>
            _configuration = (IConfiguration?)services.GetService(typeof(IConfiguration));

        public void Configure(AgdirEmailOptions options)
        {
            if (_configuration is null) return;
            var section = _configuration.GetSection(AgdirEmailOptions.SectionName);
            if (section["ApiKey"] is { Length: > 0 } apiKey) options.ApiKey = apiKey;
            if (section["BaseUrl"] is { Length: > 0 } baseUrl) options.BaseUrl = baseUrl;
            if (section.GetValue<int?>("MaxRetries") is { } maxRetries) options.MaxRetries = maxRetries;
            if (section.GetValue<TimeSpan?>("Timeout") is { } timeout) options.Timeout = timeout;
        }
    }
}
