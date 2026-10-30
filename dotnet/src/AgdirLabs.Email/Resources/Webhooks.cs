using System;
using System.Collections.Generic;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
using AgdirLabs.Email.Internal;

namespace AgdirLabs.Email;

/// <summary>Manage webhook endpoints.</summary>
public interface IWebhooksResource
{
    /// <summary>Messages sent (or queued) to an endpoint, with their HTTP attempts.</summary>
    IWebhookDeliveriesResource Deliveries { get; }

    /// <summary>Registers an endpoint. Keep the returned signing secret to verify incoming events.</summary>
    /// <param name="request">Endpoint URL and event types.</param>
    /// <param name="cancellationToken">Cancels the request.</param>
    Task<WebhookSigningSecret> CreateAsync(CreateWebhookRequest request, CancellationToken cancellationToken = default);

    /// <summary>Reads a webhook, including its signing secret.</summary>
    /// <param name="id">Webhook id.</param>
    /// <param name="cancellationToken">Cancels the request.</param>
    Task<WebhookWithSecret> GetAsync(string id, CancellationToken cancellationToken = default);

    /// <summary>Lists one page of webhooks.</summary>
    /// <param name="limit">Page size, 1–100.</param>
    /// <param name="after">Cursor: <see cref="Page{T}.NextCursor"/> of the previous page.</param>
    /// <param name="cancellationToken">Cancels the request.</param>
    Task<Page<Webhook>> ListAsync(int? limit = null, string? after = null, CancellationToken cancellationToken = default);

    /// <summary>Every webhook, fetching further pages as you iterate.</summary>
    /// <param name="limit">Page size, 1–100.</param>
    /// <param name="cancellationToken">Cancels the iteration.</param>
    IAsyncEnumerable<Webhook> ListAllAsync(int? limit = null, CancellationToken cancellationToken = default);

    /// <summary>Changes a webhook's endpoint, events or status.</summary>
    /// <param name="id">Webhook id.</param>
    /// <param name="request">Properties to change.</param>
    /// <param name="cancellationToken">Cancels the request.</param>
    Task<ResourceRef> UpdateAsync(string id, UpdateWebhookRequest request, CancellationToken cancellationToken = default);

    /// <summary>Deletes a webhook.</summary>
    /// <param name="id">Webhook id.</param>
    /// <param name="cancellationToken">Cancels the request.</param>
    Task<DeletedResource> DeleteAsync(string id, CancellationToken cancellationToken = default);

    /// <summary>Issues a new signing secret; the old one stops working immediately.</summary>
    /// <param name="id">Webhook id.</param>
    /// <param name="cancellationToken">Cancels the request.</param>
    Task<WebhookSigningSecret> RotateSecretAsync(string id, CancellationToken cancellationToken = default);
}

/// <inheritdoc />
public sealed class WebhooksResource : IWebhooksResource
{
    private readonly HttpTransport _http;

    internal WebhooksResource(HttpTransport http)
    {
        _http = http;
        Deliveries = new WebhookDeliveriesResource(http);
    }

    /// <inheritdoc />
    public IWebhookDeliveriesResource Deliveries { get; }

    /// <inheritdoc />
    public Task<WebhookSigningSecret> CreateAsync(CreateWebhookRequest request, CancellationToken cancellationToken = default)
    {
        if (request is null) throw new ArgumentNullException(nameof(request));
        var body = Json.Write(w =>
        {
            w.WriteStartObject();
            w.WriteString("endpoint", request.Endpoint);
            w.WriteStrings("events", request.Events);
            w.WriteEndObject();
        });
        return _http.SendAsync(new ApiRequest(HttpMethod.Post, "/webhooks") { Body = body }, Wire.SigningSecret, cancellationToken);
    }

    /// <inheritdoc />
    public Task<WebhookWithSecret> GetAsync(string id, CancellationToken cancellationToken = default) =>
        _http.SendAsync(new ApiRequest(HttpMethod.Get, "/webhooks/" + Wire.Segment(id)) { Retryable = true }, Wire.WebhookWithSecret, cancellationToken);

    /// <inheritdoc />
    public Task<Page<Webhook>> ListAsync(int? limit = null, string? after = null, CancellationToken cancellationToken = default) =>
        _http.SendAsync(
            new ApiRequest(HttpMethod.Get, "/webhooks") { Query = Wire.ListQuery(limit, after), Retryable = true },
            body => Wire.Page(body, Wire.Webhook),
            cancellationToken);

    /// <inheritdoc />
    public IAsyncEnumerable<Webhook> ListAllAsync(int? limit = null, CancellationToken cancellationToken = default) =>
        Paginator.All((after, ct) => ListAsync(limit, after, ct), cancellationToken);

    /// <inheritdoc />
    public Task<ResourceRef> UpdateAsync(string id, UpdateWebhookRequest request, CancellationToken cancellationToken = default)
    {
        if (request is null) throw new ArgumentNullException(nameof(request));
        var body = Json.Write(w =>
        {
            w.WriteStartObject();
            w.WriteOptional("endpoint", request.Endpoint);
            w.WriteStrings("events", request.Events);
            if (request.Status is { } status) w.WriteString("status", Json.ToWire(status));
            w.WriteEndObject();
        });
        return _http.SendAsync(
            new ApiRequest(HttpTransport.Patch, "/webhooks/" + Wire.Segment(id)) { Body = body, Retryable = true },
            Wire.Ref,
            cancellationToken);
    }

    /// <inheritdoc />
    public Task<DeletedResource> DeleteAsync(string id, CancellationToken cancellationToken = default) =>
        _http.SendAsync(new ApiRequest(HttpMethod.Delete, "/webhooks/" + Wire.Segment(id)), Wire.Deleted, cancellationToken);

    /// <inheritdoc />
    public Task<WebhookSigningSecret> RotateSecretAsync(string id, CancellationToken cancellationToken = default) =>
        _http.SendAsync(new ApiRequest(HttpMethod.Post, $"/webhooks/{Wire.Segment(id)}/signing-secret/rotate"), Wire.SigningSecret, cancellationToken);
}

/// <summary>Messages sent (or queued) to a webhook endpoint, with their HTTP attempts.</summary>
public interface IWebhookDeliveriesResource
{
    /// <summary>Lists one page of messages for a webhook, newest first.</summary>
    /// <param name="webhookId">Webhook id.</param>
    /// <param name="limit">Page size, 1–100.</param>
    /// <param name="after">Cursor: <see cref="Page{T}.NextCursor"/> of the previous page.</param>
    /// <param name="cancellationToken">Cancels the request.</param>
    Task<Page<WebhookDelivery>> ListAsync(string webhookId, int? limit = null, string? after = null, CancellationToken cancellationToken = default);

    /// <summary>Every message for a webhook, fetching further pages as you iterate.</summary>
    /// <param name="webhookId">Webhook id.</param>
    /// <param name="limit">Page size, 1–100.</param>
    /// <param name="cancellationToken">Cancels the iteration.</param>
    IAsyncEnumerable<WebhookDelivery> ListAllAsync(string webhookId, int? limit = null, CancellationToken cancellationToken = default);

    /// <summary>Reads one message, including the exact payload sent.</summary>
    /// <param name="webhookId">Webhook id.</param>
    /// <param name="deliveryId">Delivery id.</param>
    /// <param name="cancellationToken">Cancels the request.</param>
    Task<WebhookDeliveryDetail> GetAsync(string webhookId, string deliveryId, CancellationToken cancellationToken = default);

    /// <summary>Lists the HTTP attempts for one message, newest first.</summary>
    /// <param name="webhookId">Webhook id.</param>
    /// <param name="deliveryId">Delivery id.</param>
    /// <param name="limit">Page size, 1–100.</param>
    /// <param name="after">Cursor: <see cref="Page{T}.NextCursor"/> of the previous page.</param>
    /// <param name="cancellationToken">Cancels the request.</param>
    Task<Page<WebhookAttempt>> ListAttemptsAsync(string webhookId, string deliveryId, int? limit = null, string? after = null, CancellationToken cancellationToken = default);

    /// <summary>Sends the identical message again, once, right now (also to disabled endpoints).</summary>
    /// <param name="webhookId">Webhook id.</param>
    /// <param name="deliveryId">Delivery id.</param>
    /// <param name="cancellationToken">Cancels the request.</param>
    Task<ResourceRef> ReplayAsync(string webhookId, string deliveryId, CancellationToken cancellationToken = default);
}

/// <inheritdoc />
public sealed class WebhookDeliveriesResource : IWebhookDeliveriesResource
{
    private readonly HttpTransport _http;

    internal WebhookDeliveriesResource(HttpTransport http) => _http = http;

    /// <inheritdoc />
    public Task<Page<WebhookDelivery>> ListAsync(string webhookId, int? limit = null, string? after = null, CancellationToken cancellationToken = default) =>
        _http.SendAsync(
            new ApiRequest(HttpMethod.Get, $"/webhooks/{Wire.Segment(webhookId)}/events") { Query = Wire.ListQuery(limit, after), Retryable = true },
            body => Wire.Page(body, Wire.Delivery),
            cancellationToken);

    /// <inheritdoc />
    public IAsyncEnumerable<WebhookDelivery> ListAllAsync(string webhookId, int? limit = null, CancellationToken cancellationToken = default) =>
        Paginator.All((after, ct) => ListAsync(webhookId, limit, after, ct), cancellationToken);

    /// <inheritdoc />
    public Task<WebhookDeliveryDetail> GetAsync(string webhookId, string deliveryId, CancellationToken cancellationToken = default) =>
        _http.SendAsync(
            new ApiRequest(HttpMethod.Get, $"/webhooks/{Wire.Segment(webhookId)}/events/{Wire.Segment(deliveryId)}") { Retryable = true },
            Wire.DeliveryDetail,
            cancellationToken);

    /// <inheritdoc />
    public Task<Page<WebhookAttempt>> ListAttemptsAsync(string webhookId, string deliveryId, int? limit = null, string? after = null, CancellationToken cancellationToken = default) =>
        _http.SendAsync(
            new ApiRequest(HttpMethod.Get, $"/webhooks/{Wire.Segment(webhookId)}/events/{Wire.Segment(deliveryId)}/attempts")
            {
                Query = Wire.ListQuery(limit, after),
                Retryable = true,
            },
            body => Wire.Page(body, Wire.Attempt),
            cancellationToken);

    /// <inheritdoc />
    public Task<ResourceRef> ReplayAsync(string webhookId, string deliveryId, CancellationToken cancellationToken = default) =>
        _http.SendAsync(
            new ApiRequest(HttpMethod.Post, $"/webhooks/{Wire.Segment(webhookId)}/events/{Wire.Segment(deliveryId)}/replay"),
            Wire.Ref,
            cancellationToken);
}
