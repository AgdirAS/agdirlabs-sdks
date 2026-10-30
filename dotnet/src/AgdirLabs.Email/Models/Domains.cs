using System;
using System.Collections.Generic;

namespace AgdirLabs.Email;

/// <summary>Verification state of a domain or one of its DNS records.</summary>
public enum DomainStatus
{
    /// <summary>A state this SDK version does not know yet.</summary>
    Unknown,
    /// <summary>Verification has not started.</summary>
    NotStarted,
    /// <summary>Waiting for the DNS records to appear.</summary>
    Pending,
    /// <summary>Verified: you can send from this domain.</summary>
    Verified,
    /// <summary>The records were not found in time.</summary>
    Failed,
    /// <summary>A previously verified domain's records could not be read; retried automatically.</summary>
    TemporaryFailure,
}

/// <summary>Request for <see cref="IDomainsResource.CreateAsync"/>.</summary>
public sealed class CreateDomainRequest
{
    /// <summary>The domain to send from, e.g. <c>acme.com</c> or <c>mail.acme.com</c>.</summary>
    public string Name { get; set; } = "";

    /// <summary>Sending region. Defaults to <c>eu-north-1</c>.</summary>
    public string? Region { get; set; }
}

/// <summary>A sending domain.</summary>
/// <param name="Id">Domain id.</param>
/// <param name="Name">Domain name.</param>
/// <param name="Status">Verification status.</param>
/// <param name="Region">Sending region, e.g. <c>eu-north-1</c>.</param>
/// <param name="CreatedAt">When the domain was added.</param>
/// <param name="OpenTracking">Whether opens are tracked.</param>
/// <param name="ClickTracking">Whether clicks are tracked.</param>
public record Domain(
    string Id,
    string Name,
    DomainStatus Status,
    string Region,
    DateTimeOffset CreatedAt,
    bool OpenTracking,
    bool ClickTracking);

/// <summary>A sending domain with the DNS records to publish.</summary>
/// <param name="Id">Domain id.</param>
/// <param name="Name">Domain name.</param>
/// <param name="Status">Verification status.</param>
/// <param name="Region">Sending region, e.g. <c>eu-north-1</c>.</param>
/// <param name="CreatedAt">When the domain was added.</param>
/// <param name="OpenTracking">Whether opens are tracked.</param>
/// <param name="ClickTracking">Whether clicks are tracked.</param>
/// <param name="Records">DNS records to add at your DNS provider.</param>
public sealed record DomainWithRecords(
    string Id,
    string Name,
    DomainStatus Status,
    string Region,
    DateTimeOffset CreatedAt,
    bool OpenTracking,
    bool ClickTracking,
    IReadOnlyList<DnsRecord> Records)
    : Domain(Id, Name, Status, Region, CreatedAt, OpenTracking, ClickTracking);

/// <summary>A DNS record required to verify a domain.</summary>
/// <param name="Purpose">What the record is for: <c>DKIM</c>, <c>SPF</c> or <c>DMARC</c>.</param>
/// <param name="Name">Relative to the registrable domain, e.g. <c>agdir._domainkey</c> or <c>send</c>.</param>
/// <param name="Type">DNS record type: <c>TXT</c>, <c>MX</c> or <c>CNAME</c>.</param>
/// <param name="Value">Record value.</param>
/// <param name="Ttl">TTL, e.g. <c>Auto</c>.</param>
/// <param name="Priority">MX priority, when applicable.</param>
/// <param name="Status">Whether the record was found.</param>
public sealed record DnsRecord(
    string Purpose,
    string Name,
    string Type,
    string Value,
    string Ttl,
    int? Priority,
    DomainStatus Status);
