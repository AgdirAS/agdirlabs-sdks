using System;
using System.Collections.Generic;
using System.Text.Json;

namespace AgdirLabs.Email;

/// <summary>Webhook event types.</summary>
public static class WebhookEventTypes
{
    /// <summary>The email was handed to the receiving mail server.</summary>
    public const string EmailSent = "email.sent";
    /// <summary>The email was scheduled for later.</summary>
    public const string EmailScheduled = "email.scheduled";
    /// <summary>The email was delivered.</summary>
    public const string EmailDelivered = "email.delivered";
    /// <summary>Delivery is being retried.</summary>
    public const string EmailDeliveryDelayed = "email.delivery_delayed";
    /// <summary>The recipient marked the email as spam.</summary>
    public const string EmailComplained = "email.complained";
    /// <summary>The email bounced.</summary>
    public const string EmailBounced = "email.bounced";
    /// <summary>The email was opened.</summary>
    public const string EmailOpened = "email.opened";
    /// <summary>A link in the email was clicked.</summary>
    public const string EmailClicked = "email.clicked";
    /// <summary>The email could not be sent.</summary>
    public const string EmailFailed = "email.failed";

    /// <summary>Every event type.</summary>
    public static IReadOnlyList<string> All { get; } = new[]
    {
        EmailSent, EmailScheduled, EmailDelivered, EmailDeliveryDelayed, EmailComplained,
        EmailBounced, EmailOpened, EmailClicked, EmailFailed,
    };
}

/// <summary>Whether a webhook endpoint receives events.</summary>
public enum WebhookStatus
{
    /// <summary>A state this SDK version does not know yet.</summary>
    Unknown,
    /// <summary>Events are delivered.</summary>
    Enabled,
    /// <summary>Events are not delivered.</summary>
    Disabled,
}

/// <summary>Request for <see cref="IWebhooksResource.CreateAsync"/>.</summary>
public sealed class CreateWebhookRequest
{
    /// <summary>Public <c>https://</c> URL that receives events.</summary>
    public string Endpoint { get; set; } = "";

    /// <summary>Event types to receive; see <see cref="WebhookEventTypes"/>.</summary>
    public IList<string> Events { get; set; } = new List<string>();
}

/// <summary>Request for <see cref="IWebhooksResource.UpdateAsync"/>. Unset properties are left unchanged.</summary>
public sealed class UpdateWebhookRequest
{
    /// <summary>New endpoint URL.</summary>
    public string? Endpoint { get; set; }

    /// <summary>New list of event types.</summary>
    public IList<string>? Events { get; set; }

    /// <summary>Enable or disable the endpoint.</summary>
    public WebhookStatus? Status { get; set; }
}

/// <summary>A webhook endpoint.</summary>
/// <param name="Id">Webhook id.</param>
/// <param name="Endpoint">URL that receives events.</param>
/// <param name="Status">Whether events are delivered.</param>
/// <param name="Events">Subscribed event types.</param>
/// <param name="CreatedAt">When the webhook was created.</param>
public record Webhook(
    string Id,
    string Endpoint,
    WebhookStatus Status,
    IReadOnlyList<string> Events,
    DateTimeOffset CreatedAt);

/// <summary>A webhook endpoint with its signing secret.</summary>
/// <param name="Id">Webhook id.</param>
/// <param name="Endpoint">URL that receives events.</param>
/// <param name="Status">Whether events are delivered.</param>
/// <param name="Events">Subscribed event types.</param>
/// <param name="CreatedAt">When the webhook was created.</param>
/// <param name="SigningSecret"><c>whsec_…</c> — pass it to <see cref="WebhookVerifier"/>.</param>
public sealed record WebhookWithSecret(
    string Id,
    string Endpoint,
    WebhookStatus Status,
    IReadOnlyList<string> Events,
    DateTimeOffset CreatedAt,
    string SigningSecret)
    : Webhook(Id, Endpoint, Status, Events, CreatedAt);

/// <summary>A webhook id and its (new) signing secret.</summary>
/// <param name="Id">Webhook id.</param>
/// <param name="SigningSecret"><c>whsec_…</c> — keep it to verify incoming events.</param>
public sealed record WebhookSigningSecret(string Id, string SigningSecret);

/// <summary>State of one message to a webhook endpoint.</summary>
public enum WebhookDeliveryStatus
{
    /// <summary>A state this SDK version does not know yet.</summary>
    Unknown,
    /// <summary>Waiting for its (next) attempt.</summary>
    Pending,
    /// <summary>An attempt is in progress.</summary>
    Attempting,
    /// <summary>The endpoint answered with a 2xx status.</summary>
    Success,
    /// <summary>All attempts failed.</summary>
    Failed,
}

/// <summary>A message sent (or queued) to a webhook endpoint.</summary>
/// <param name="Id">Delivery id (the <c>webhook-id</c> header).</param>
/// <param name="Type">Event type.</param>
/// <param name="Status">Delivery state.</param>
/// <param name="CreatedAt">When the message was created.</param>
public record WebhookDelivery(string Id, string Type, WebhookDeliveryStatus Status, DateTimeOffset CreatedAt);

/// <summary>A webhook message with its payload.</summary>
/// <param name="Id">Delivery id (the <c>webhook-id</c> header).</param>
/// <param name="Type">Event type.</param>
/// <param name="Status">Delivery state.</param>
/// <param name="CreatedAt">When the message was created.</param>
/// <param name="NextAttemptAt">When the next retry is due, if any.</param>
/// <param name="Payload">The exact JSON body sent to your endpoint.</param>
public sealed record WebhookDeliveryDetail(
    string Id,
    string Type,
    WebhookDeliveryStatus Status,
    DateTimeOffset CreatedAt,
    DateTimeOffset? NextAttemptAt,
    JsonElement Payload)
    : WebhookDelivery(Id, Type, Status, CreatedAt);

/// <summary>One HTTP attempt to deliver a webhook message.</summary>
/// <param name="Id">Attempt id.</param>
/// <param name="HttpStatus">Response status, or 0 when no HTTP response was received.</param>
/// <param name="Response">Response body (truncated) or the connection error.</param>
/// <param name="SentAt">When the attempt was made.</param>
public sealed record WebhookAttempt(string Id, int HttpStatus, string Response, DateTimeOffset SentAt);

/// <summary>An event received on your webhook endpoint, as returned by <see cref="WebhookVerifier"/>.</summary>
/// <param name="Type">Event type; see <see cref="WebhookEventTypes"/>.</param>
/// <param name="CreatedAt">When the event occurred.</param>
/// <param name="Data">Details about the email.</param>
public sealed record WebhookEvent(string Type, DateTimeOffset CreatedAt, WebhookEventData Data);

/// <summary>The email a webhook event is about.</summary>
/// <param name="EmailId">Email id — use it to deduplicate retried events.</param>
/// <param name="MessageId">The <c>Message-ID</c> of the sent email, once known.</param>
/// <param name="From">Sender.</param>
/// <param name="To">Recipients.</param>
/// <param name="Subject">Subject line.</param>
/// <param name="Tags">The email's tags.</param>
/// <param name="CreatedAt">When the email was accepted.</param>
/// <param name="Bounce">Bounce details for <c>email.bounced</c>.</param>
/// <param name="Click">Click details for <c>email.clicked</c>.</param>
/// <param name="Failed">Failure details for <c>email.failed</c>.</param>
public sealed record WebhookEventData(
    string EmailId,
    string? MessageId,
    string From,
    IReadOnlyList<string> To,
    string Subject,
    IReadOnlyDictionary<string, string> Tags,
    DateTimeOffset CreatedAt,
    WebhookBounce? Bounce,
    WebhookClick? Click,
    WebhookFailure? Failed);

/// <summary>Why an email bounced.</summary>
/// <param name="Type">Bounce type, e.g. <c>Permanent</c>.</param>
/// <param name="SubType">Bounce sub-type, e.g. <c>General</c>.</param>
/// <param name="Message">Diagnostic message from the receiving server.</param>
public sealed record WebhookBounce(string Type, string SubType, string Message);

/// <summary>A link click.</summary>
/// <param name="Link">The clicked URL.</param>
/// <param name="IpAddress">Client IP address.</param>
/// <param name="UserAgent">Client user agent.</param>
/// <param name="Timestamp">When the click happened.</param>
public sealed record WebhookClick(string Link, string IpAddress, string UserAgent, DateTimeOffset Timestamp);

/// <summary>Why an email could not be sent.</summary>
/// <param name="Reason">Failure reason.</param>
public sealed record WebhookFailure(string Reason);
