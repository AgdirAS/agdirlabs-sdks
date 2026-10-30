using System;
using System.Reflection;
using Xunit;

namespace AgdirLabs.Email.Tests;

[CollectionDefinition("environment", DisableParallelization = true)]
public sealed class EnvironmentCollection
{
}

[Collection("environment")]
public sealed class ConfigurationTests : IDisposable
{
    private readonly string? _key = Environment.GetEnvironmentVariable("AGDIR_EMAIL_API_KEY");
    private readonly string? _url = Environment.GetEnvironmentVariable("AGDIR_EMAIL_BASE_URL");

    public void Dispose()
    {
        Environment.SetEnvironmentVariable("AGDIR_EMAIL_API_KEY", _key);
        Environment.SetEnvironmentVariable("AGDIR_EMAIL_BASE_URL", _url);
    }

    [Fact]
    public void UsesApiAgdirEmailByDefault()
    {
        Environment.SetEnvironmentVariable("AGDIR_EMAIL_BASE_URL", null);
        Assert.Equal("https://api.agdir.email", new AgdirEmailClient("re_test").BaseUrl);
        Assert.Equal(AgdirEmailOptions.DefaultBaseUrl, new AgdirEmailClient("re_test").BaseUrl);
    }

    [Fact]
    public void ReadsKeyAndBaseUrlFromEnvironment_EmptyValuesCountAsUnset()
    {
        Environment.SetEnvironmentVariable("AGDIR_EMAIL_API_KEY", "re_env");
        Environment.SetEnvironmentVariable("AGDIR_EMAIL_BASE_URL", "http://localhost:8080");
        Assert.Equal("http://localhost:8080", new AgdirEmailClient().BaseUrl);

        // An explicit option wins over the environment; an empty option falls back to it.
        Assert.Equal("https://x.test", new AgdirEmailClient(new AgdirEmailOptions { BaseUrl = "https://x.test" }).BaseUrl);
        Assert.Equal("http://localhost:8080", new AgdirEmailClient(new AgdirEmailOptions { BaseUrl = "" }).BaseUrl);

        Environment.SetEnvironmentVariable("AGDIR_EMAIL_API_KEY", "");
        Environment.SetEnvironmentVariable("AGDIR_EMAIL_BASE_URL", "");
        var error = Assert.Throws<InvalidOperationException>(() => new AgdirEmailClient());
        Assert.Contains("AGDIR_EMAIL_API_KEY", error.Message);
        Assert.Contains("https://app.agdir.email", error.Message);
        Assert.Equal(AgdirEmailOptions.DefaultBaseUrl, new AgdirEmailClient("re_test").BaseUrl);
    }

    [Fact]
    public void RejectsInvalidOptions()
    {
        Assert.Throws<ArgumentOutOfRangeException>(() => new AgdirEmailClient(new AgdirEmailOptions { ApiKey = "re_test", MaxRetries = -1 }));
        Assert.Throws<ArgumentOutOfRangeException>(() => new AgdirEmailClient(new AgdirEmailOptions { ApiKey = "re_test", Timeout = TimeSpan.Zero }));
        Assert.Throws<ArgumentException>(() => new AgdirEmailClient(new AgdirEmailOptions { ApiKey = "re_test", BaseUrl = "not a url" }));
    }

    [Fact]
    public void VersionMatchesThePackageVersion()
    {
        var informational = typeof(AgdirEmailClient).Assembly.GetCustomAttribute<AssemblyInformationalVersionAttribute>()!.InformationalVersion;
        Assert.Equal(AgdirEmailClient.Version, informational.Split('+')[0]);
        Assert.Equal("agdirlabs-email-dotnet/" + AgdirEmailClient.Version, AgdirEmailClient.UserAgent);
    }
}
