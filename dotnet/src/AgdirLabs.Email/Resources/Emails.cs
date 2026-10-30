using System;
using System.Collections.Generic;
using System.Linq;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
using AgdirLabs.Email.Internal;

namespace AgdirLabs.Email;

/// <summary>Send, read and cancel emails.</summary>
public interface IEmailsResource
{
    /// <summary>Accepts one email for delivery (or for <see cref="SendEmailRequest.ScheduledAt"/>).</summary>
    /// <param name="email">The email.</param>
    /// <param name="options">Idempotency key.</param>
    /// <param name="cancellationToken">Cancels the request.</param>
    /// <returns>The new email's id.</returns>
    /// <exception cref="AgdirEmailException">The API refused the email or could not be reached.</exception>
    Task<SendEmailResult> SendAsync(SendEmailRequest email, SendEmailOptions? options = null, CancellationToken cancellationToken = default);

    /// <summary>Accepts up to 100 emails in one request. Strict validation is all-or-nothing.</summary>
    /// <param name="emails">1–100 emails.</param>
    /// <param name="options">Idempotency key and validation mode.</param>
    /// <param name="cancellationToken">Cancels the request.</param>
    /// <exception cref="AgdirEmailException">The API refused the batch or could not be reached.</exception>
    Task<SendBatchResult> SendBatchAsync(IEnumerable<SendEmailRequest> emails, SendBatchOptions? options = null, CancellationToken cancellationToken = default);

    /// <summary>Reads an email and its current delivery status. Requires a full-access key.</summary>
    /// <param name="id">Email id.</param>
    /// <param name="cancellationToken">Cancels the request.</param>
    /// <exception cref="AgdirEmailException">For example <c>not_found</c>.</exception>
    Task<Email> GetAsync(string id, CancellationToken cancellationToken = default);

    /// <summary>Cancels a scheduled email that has not been sent yet.</summary>
    /// <param name="id">Email id.</param>
    /// <param name="cancellationToken">Cancels the request.</param>
    /// <exception cref="AgdirEmailException">For example when the email is not scheduled.</exception>
    Task<ResourceRef> CancelAsync(string id, CancellationToken cancellationToken = default);
}

/// <inheritdoc />
public sealed class EmailsResource : IEmailsResource
{
    /// <summary>The most emails one batch request accepts.</summary>
    public const int MaxBatchSize = 100;

    private readonly HttpTransport _http;

    internal EmailsResource(HttpTransport http) => _http = http;

    /// <inheritdoc />
    public Task<SendEmailResult> SendAsync(SendEmailRequest email, SendEmailOptions? options = null, CancellationToken cancellationToken = default)
    {
        if (email is null) throw new ArgumentNullException(nameof(email));
        var key = IdempotencyKey(options);
        var request = new ApiRequest(HttpMethod.Post, "/emails")
        {
            Body = Json.Write(w => Wire.WriteEmail(w, email)),
            Headers = new[] { new KeyValuePair<string, string?>("Idempotency-Key", key) },
            Retryable = key is not null,
        };
        return _http.SendAsync(request, e => new SendEmailResult(Wire.Id(e)), cancellationToken);
    }

    /// <inheritdoc />
    public Task<SendBatchResult> SendBatchAsync(IEnumerable<SendEmailRequest> emails, SendBatchOptions? options = null, CancellationToken cancellationToken = default)
    {
        if (emails is null) throw new ArgumentNullException(nameof(emails));
        var list = emails.ToList();
        var key = IdempotencyKey(options);
        var request = new ApiRequest(HttpMethod.Post, "/emails/batch")
        {
            Body = Json.Write(w =>
            {
                w.WriteStartArray();
                foreach (var email in list) Wire.WriteEmail(w, email);
                w.WriteEndArray();
            }),
            Headers = new[]
            {
                new KeyValuePair<string, string?>("Idempotency-Key", key),
                new KeyValuePair<string, string?>("x-batch-validation", options?.Validation == BatchValidation.Permissive ? "permissive" : null),
            },
            Retryable = key is not null,
        };
        return _http.SendAsync(request, Wire.Batch, cancellationToken);
    }

    /// <inheritdoc />
    public Task<Email> GetAsync(string id, CancellationToken cancellationToken = default) =>
        _http.SendAsync(new ApiRequest(HttpMethod.Get, "/emails/" + Wire.Segment(id)) { Retryable = true }, Wire.Email, cancellationToken);

    /// <inheritdoc />
    public Task<ResourceRef> CancelAsync(string id, CancellationToken cancellationToken = default) =>
        _http.SendAsync(new ApiRequest(HttpMethod.Post, $"/emails/{Wire.Segment(id)}/cancel") { Retryable = true }, Wire.Ref, cancellationToken);

    private string? IdempotencyKey(SendEmailOptions? options)
    {
        if (!string.IsNullOrEmpty(options?.IdempotencyKey)) return options!.IdempotencyKey;
        // Without a key a retried send could be delivered twice, so only generate one when we may retry.
        return _http.MaxRetries > 0 ? Guid.NewGuid().ToString() : null;
    }
}
