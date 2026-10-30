using System;
using System.Collections.Generic;
using System.Threading.Tasks;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Options;
using Xunit;

namespace AgdirLabs.Email.Tests;

public sealed class DependencyInjectionTests
{
    [Fact]
    public async Task BindsTheAgdirEmailSection_AppliesTheDelegate_AndUsesTheFactoryClient()
    {
        var configuration = new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?>
            {
                ["AgdirEmail:ApiKey"] = "re_config",
                ["AgdirEmail:BaseUrl"] = "https://config.test",
                ["AgdirEmail:MaxRetries"] = "5",
                ["AgdirEmail:Timeout"] = "00:00:10",
            })
            .Build();
        var handler = new FakeHandler().Respond(new { id = "em_1" });

        var services = new ServiceCollection();
        services.AddSingleton<IConfiguration>(configuration);
        services.AddAgdirEmail(o => o.BaseUrl = "https://override.test/api")
            .ConfigurePrimaryHttpMessageHandler(() => handler);
        using var provider = services.BuildServiceProvider();

        var options = provider.GetRequiredService<IOptions<AgdirEmailOptions>>().Value;
        Assert.Equal("re_config", options.ApiKey);
        Assert.Equal(5, options.MaxRetries);
        Assert.Equal(TimeSpan.FromSeconds(10), options.Timeout);

        var agdir = provider.GetRequiredService<IAgdirEmailClient>();
        await agdir.Emails.SendAsync(new SendEmailRequest { From = "a@acme.com", To = { "b@example.com" }, Subject = "s", Text = "t" });
        var request = Assert.Single(handler.Requests);
        Assert.Equal("https://override.test/api/emails", request.Url.ToString());
        Assert.Equal("Bearer re_config", request.Header("Authorization"));
        Assert.Equal("https://override.test/api", provider.GetRequiredService<AgdirEmailClient>().BaseUrl);
    }

    [Fact]
    public void WorksWithoutConfiguration()
    {
        var services = new ServiceCollection();
        services.AddAgdirEmail(o => o.ApiKey = "re_delegate");
        using var provider = services.BuildServiceProvider();
        Assert.Equal("re_delegate", provider.GetRequiredService<IOptions<AgdirEmailOptions>>().Value.ApiKey);
        Assert.NotNull(provider.GetRequiredService<IAgdirEmailClient>().Emails);
    }
}
