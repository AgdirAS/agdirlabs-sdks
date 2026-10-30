using System;
using System.Collections.Generic;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
using AgdirLabs.Email.Internal;

namespace AgdirLabs.Email;

/// <summary>Manage sending domains.</summary>
public interface IDomainsResource
{
    /// <summary>Adds a sending domain. Publish the returned <see cref="DomainWithRecords.Records"/> in DNS, then call <see cref="VerifyAsync"/>.</summary>
    /// <param name="request">Domain name and optional region.</param>
    /// <param name="cancellationToken">Cancels the request.</param>
    Task<DomainWithRecords> CreateAsync(CreateDomainRequest request, CancellationToken cancellationToken = default);

    /// <summary>Reads a domain with its DNS records.</summary>
    /// <param name="id">Domain id.</param>
    /// <param name="cancellationToken">Cancels the request.</param>
    Task<DomainWithRecords> GetAsync(string id, CancellationToken cancellationToken = default);

    /// <summary>Lists one page of domains.</summary>
    /// <param name="limit">Page size, 1–100.</param>
    /// <param name="after">Cursor: <see cref="Page{T}.NextCursor"/> of the previous page.</param>
    /// <param name="cancellationToken">Cancels the request.</param>
    Task<Page<Domain>> ListAsync(int? limit = null, string? after = null, CancellationToken cancellationToken = default);

    /// <summary>Every domain, fetching further pages as you iterate.</summary>
    /// <param name="limit">Page size, 1–100.</param>
    /// <param name="cancellationToken">Cancels the iteration.</param>
    IAsyncEnumerable<Domain> ListAllAsync(int? limit = null, CancellationToken cancellationToken = default);

    /// <summary>Starts a DNS check now instead of waiting for the next automatic one.</summary>
    /// <param name="id">Domain id.</param>
    /// <param name="cancellationToken">Cancels the request.</param>
    Task<ResourceRef> VerifyAsync(string id, CancellationToken cancellationToken = default);

    /// <summary>Deletes a domain.</summary>
    /// <param name="id">Domain id.</param>
    /// <param name="cancellationToken">Cancels the request.</param>
    Task<DeletedResource> DeleteAsync(string id, CancellationToken cancellationToken = default);
}

/// <inheritdoc />
public sealed class DomainsResource : IDomainsResource
{
    private readonly HttpTransport _http;

    internal DomainsResource(HttpTransport http) => _http = http;

    /// <inheritdoc />
    public Task<DomainWithRecords> CreateAsync(CreateDomainRequest request, CancellationToken cancellationToken = default)
    {
        if (request is null) throw new ArgumentNullException(nameof(request));
        var body = Json.Write(w =>
        {
            w.WriteStartObject();
            w.WriteString("name", request.Name);
            w.WriteOptional("region", request.Region);
            w.WriteEndObject();
        });
        return _http.SendAsync(new ApiRequest(HttpMethod.Post, "/domains") { Body = body }, Wire.DomainWithRecords, cancellationToken);
    }

    /// <inheritdoc />
    public Task<DomainWithRecords> GetAsync(string id, CancellationToken cancellationToken = default) =>
        _http.SendAsync(new ApiRequest(HttpMethod.Get, "/domains/" + Wire.Segment(id)) { Retryable = true }, Wire.DomainWithRecords, cancellationToken);

    /// <inheritdoc />
    public Task<Page<Domain>> ListAsync(int? limit = null, string? after = null, CancellationToken cancellationToken = default) =>
        _http.SendAsync(
            new ApiRequest(HttpMethod.Get, "/domains") { Query = Wire.ListQuery(limit, after), Retryable = true },
            body => Wire.Page(body, Wire.Domain),
            cancellationToken);

    /// <inheritdoc />
    public IAsyncEnumerable<Domain> ListAllAsync(int? limit = null, CancellationToken cancellationToken = default) =>
        Paginator.All((after, ct) => ListAsync(limit, after, ct), cancellationToken);

    /// <inheritdoc />
    public Task<ResourceRef> VerifyAsync(string id, CancellationToken cancellationToken = default) =>
        _http.SendAsync(new ApiRequest(HttpMethod.Post, $"/domains/{Wire.Segment(id)}/verify") { Retryable = true }, Wire.Ref, cancellationToken);

    /// <inheritdoc />
    public Task<DeletedResource> DeleteAsync(string id, CancellationToken cancellationToken = default) =>
        _http.SendAsync(new ApiRequest(HttpMethod.Delete, "/domains/" + Wire.Segment(id)), Wire.Deleted, cancellationToken);
}
