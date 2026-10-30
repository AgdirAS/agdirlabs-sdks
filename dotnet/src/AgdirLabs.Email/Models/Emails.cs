using System;
using System.Collections.Generic;

namespace AgdirLabs.Email;

/// <summary>An email to send with <see cref="IEmailsResource.SendAsync"/> or as part of a batch.</summary>
/// <remarks>
/// Addresses are <c>"Name &lt;addr@example.com&gt;"</c> or a bare address.
/// At least one of <see cref="Html"/> and <see cref="Text"/> is required.
/// </remarks>
public sealed class SendEmailRequest
{
    /// <summary>Sender, on a verified domain, e.g. <c>Acme &lt;hello@acme.com&gt;</c>.</summary>
    public string From { get; set; } = "";

    /// <summary>Recipients (at least one).</summary>
    public IList<string> To { get; set; } = new List<string>();

    /// <summary>Subject line.</summary>
    public string Subject { get; set; } = "";

    /// <summary>HTML body.</summary>
    public string? Html { get; set; }

    /// <summary>Plain-text body.</summary>
    public string? Text { get; set; }

    /// <summary>Carbon-copy recipients.</summary>
    public IList<string>? Cc { get; set; }

    /// <summary>Blind carbon-copy recipients.</summary>
    public IList<string>? Bcc { get; set; }

    /// <summary>Reply-To addresses.</summary>
    public IList<string>? ReplyTo { get; set; }

    /// <summary>Custom MIME headers, e.g. <c>X-Entity-Ref-ID</c>.</summary>
    public IDictionary<string, string>? Headers { get; set; }

    /// <summary>Tags (ASCII letters, digits, <c>_</c> and <c>-</c>), echoed back in webhook events.</summary>
    public IList<EmailTag>? Tags { get; set; }

    /// <summary>Deliver later instead of now. Cancel with <see cref="IEmailsResource.CancelAsync"/>.</summary>
    public DateTimeOffset? ScheduledAt { get; set; }
}

/// <summary>A name/value tag attached to an email.</summary>
/// <param name="Name">ASCII letters, digits, <c>_</c> and <c>-</c>.</param>
/// <param name="Value">ASCII letters, digits, <c>_</c> and <c>-</c>.</param>
public sealed record EmailTag(string Name, string Value);

/// <summary>Options for <see cref="IEmailsResource.SendAsync"/>.</summary>
public class SendEmailOptions
{
    /// <summary>
    /// Makes a send safe to retry: the same key with the same body returns the original result
    /// instead of sending again. When unset and retries are enabled, the client generates one per call.
    /// </summary>
    public string? IdempotencyKey { get; set; }
}

/// <summary>Options for <see cref="IEmailsResource.SendBatchAsync"/>.</summary>
public sealed class SendBatchOptions : SendEmailOptions
{
    /// <summary>How invalid emails in a batch are handled. Default <see cref="BatchValidation.Strict"/>.</summary>
    public BatchValidation Validation { get; set; } = BatchValidation.Strict;
}

/// <summary>Batch validation mode.</summary>
public enum BatchValidation
{
    /// <summary>One invalid email rejects the whole batch with a <c>validation_error</c>.</summary>
    Strict,

    /// <summary>Valid emails are accepted; invalid ones are reported in <see cref="SendBatchResult.Errors"/>.</summary>
    Permissive,
}

/// <summary>The result of accepting an email.</summary>
/// <param name="Id">The email's id.</param>
public sealed record SendEmailResult(string Id);

/// <summary>The result of <see cref="IEmailsResource.SendBatchAsync"/>.</summary>
/// <param name="Data">Accepted emails, in input order.</param>
/// <param name="Errors">With <see cref="BatchValidation.Permissive"/>: emails that were not accepted, by input index.</param>
public sealed record SendBatchResult(IReadOnlyList<SendEmailResult> Data, IReadOnlyList<BatchError> Errors);

/// <summary>An email in a permissive batch that was not accepted.</summary>
/// <param name="Index">Zero-based position in the input list.</param>
/// <param name="Message">Why it was rejected.</param>
public sealed record BatchError(int Index, string Message);

/// <summary>Delivery state of an email.</summary>
public enum EmailStatus
{
    /// <summary>A state this SDK version does not know yet.</summary>
    Unknown,
    /// <summary>Accepted and waiting to be sent.</summary>
    Queued,
    /// <summary>Waiting for its scheduled time.</summary>
    Scheduled,
    /// <summary>Handed to the receiving mail server.</summary>
    Sent,
    /// <summary>Delivery is being retried.</summary>
    DeliveryDelayed,
    /// <summary>Accepted by the recipient's mail server.</summary>
    Delivered,
    /// <summary>Opened by the recipient.</summary>
    Opened,
    /// <summary>A link was clicked.</summary>
    Clicked,
    /// <summary>Permanently rejected by the recipient's server.</summary>
    Bounced,
    /// <summary>Could not be sent.</summary>
    Failed,
    /// <summary>Marked as spam by the recipient.</summary>
    Complained,
    /// <summary>A scheduled email that was canceled.</summary>
    Canceled,
}

/// <summary>An email and its current delivery status.</summary>
/// <param name="Id">Email id.</param>
/// <param name="From">Sender.</param>
/// <param name="To">Recipients.</param>
/// <param name="Cc">Carbon-copy recipients (empty when none).</param>
/// <param name="Bcc">Blind carbon-copy recipients (empty when none).</param>
/// <param name="ReplyTo">Reply-To addresses (empty when none).</param>
/// <param name="Subject">Subject line.</param>
/// <param name="Html">HTML body, if any.</param>
/// <param name="Text">Plain-text body, if any.</param>
/// <param name="CreatedAt">When the email was accepted.</param>
/// <param name="ScheduledAt">Scheduled delivery time, if any.</param>
/// <param name="Status">Most advanced delivery state reached so far.</param>
public sealed record Email(
    string Id,
    string From,
    IReadOnlyList<string> To,
    IReadOnlyList<string> Cc,
    IReadOnlyList<string> Bcc,
    IReadOnlyList<string> ReplyTo,
    string Subject,
    string? Html,
    string? Text,
    DateTimeOffset CreatedAt,
    DateTimeOffset? ScheduledAt,
    EmailStatus Status);
