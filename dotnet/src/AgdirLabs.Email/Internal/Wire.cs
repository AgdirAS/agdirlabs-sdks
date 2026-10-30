using System;
using System.Collections.Generic;
using System.Text.Json;

namespace AgdirLabs.Email.Internal;

/// <summary>Mapping between the SDK's models and the API's snake_case JSON.</summary>
internal static class Wire
{
    public static void WriteEmail(Utf8JsonWriter w, SendEmailRequest input)
    {
        w.WriteStartObject();
        w.WriteString("from", input.From);
        w.WriteStrings("to", input.To);
        w.WriteString("subject", input.Subject);
        w.WriteOptional("html", input.Html);
        w.WriteOptional("text", input.Text);
        w.WriteStrings("cc", input.Cc);
        w.WriteStrings("bcc", input.Bcc);
        w.WriteStrings("reply_to", input.ReplyTo);
        if (input.Headers is not null)
        {
            w.WriteStartObject("headers");
            foreach (var header in input.Headers) w.WriteString(header.Key, header.Value);
            w.WriteEndObject();
        }
        if (input.Tags is not null)
        {
            w.WriteStartArray("tags");
            foreach (var tag in input.Tags)
            {
                w.WriteStartObject();
                w.WriteString("name", tag.Name);
                w.WriteString("value", tag.Value);
                w.WriteEndObject();
            }
            w.WriteEndArray();
        }
        if (input.ScheduledAt is { } scheduledAt) w.WriteString("scheduled_at", Json.Iso(scheduledAt));
        w.WriteEndObject();
    }

    public static string Id(JsonElement e) => e.String("id");

    public static ResourceRef Ref(JsonElement e) => new(e.String("id"));

    public static DeletedResource Deleted(JsonElement e) => new(e.String("id"), true);

    public static Email Email(JsonElement e) => new(
        e.String("id"),
        e.String("from"),
        e.StringList("to"),
        e.StringList("cc"),
        e.StringList("bcc"),
        e.StringList("reply_to"),
        e.String("subject"),
        e.OptString("html"),
        e.OptString("text"),
        e.Date("created_at"),
        e.OptDate("scheduled_at"),
        e.Enum<EmailStatus>("last_event"));

    public static SendBatchResult Batch(JsonElement e) => new(
        e.List("data", d => new SendEmailResult(d.String("id"))),
        e.List("errors", x => new BatchError(x.OptInt("index") ?? -1, x.String("message"))));

    public static DnsRecord Record(JsonElement r) => new(
        r.String("record"),
        r.String("name"),
        r.String("type"),
        r.String("value"),
        r.String("ttl"),
        r.OptInt("priority"),
        r.Enum<DomainStatus>("status"));

    public static Domain Domain(JsonElement d) => new(
        d.String("id"),
        d.String("name"),
        d.Enum<DomainStatus>("status"),
        d.String("region"),
        d.Date("created_at"),
        d.Bool("open_tracking"),
        d.Bool("click_tracking"));

    public static DomainWithRecords DomainWithRecords(JsonElement d) => new(
        d.String("id"),
        d.String("name"),
        d.Enum<DomainStatus>("status"),
        d.String("region"),
        d.Date("created_at"),
        d.Bool("open_tracking"),
        d.Bool("click_tracking"),
        d.List("records", Record));

    public static Webhook Webhook(JsonElement w) => new(
        w.String("id"),
        w.String("endpoint"),
        w.Enum<WebhookStatus>("status"),
        w.StringList("events"),
        w.Date("created_at"));

    public static WebhookWithSecret WebhookWithSecret(JsonElement w) => new(
        w.String("id"),
        w.String("endpoint"),
        w.Enum<WebhookStatus>("status"),
        w.StringList("events"),
        w.Date("created_at"),
        w.String("signing_secret"));

    public static WebhookSigningSecret SigningSecret(JsonElement w) => new(w.String("id"), w.String("signing_secret"));

    public static WebhookDelivery Delivery(JsonElement e) => new(
        e.String("id"),
        e.String("type"),
        e.Enum<WebhookDeliveryStatus>("status"),
        e.Date("created_at"));

    public static WebhookDeliveryDetail DeliveryDetail(JsonElement e) => new(
        e.String("id"),
        e.String("type"),
        e.Enum<WebhookDeliveryStatus>("status"),
        e.Date("created_at"),
        e.OptDate("next_attempt_at"),
        // Clone: the JsonDocument behind `e` is disposed after mapping.
        e.Prop("payload")?.Clone() ?? default);

    public static WebhookAttempt Attempt(JsonElement a) => new(
        a.String("id"),
        a.OptInt("http_status_code") ?? 0,
        a.String("response"),
        a.Date("sent_at"));

    public static Page<T> Page<T>(JsonElement body, Func<JsonElement, T> map)
    {
        var data = body.List("data", map);
        var hasMore = body.Bool("has_more");
        string? next = null;
        if (hasMore && body.Prop("data") is { ValueKind: JsonValueKind.Array } raw && raw.GetArrayLength() > 0)
        {
            next = raw[raw.GetArrayLength() - 1].OptString("id");
        }
        return new Page<T>(data, hasMore, next);
    }

    public static WebhookEvent WebhookEvent(JsonElement body)
    {
        var d = body.Prop("data") ?? default;
        var tags = new Dictionary<string, string>();
        if (d.ValueKind == JsonValueKind.Object && d.Prop("tags") is { ValueKind: JsonValueKind.Object } t)
        {
            foreach (var tag in t.EnumerateObject())
            {
                tags[tag.Name] = tag.Value.ValueKind == JsonValueKind.String ? tag.Value.GetString()! : tag.Value.GetRawText();
            }
        }
        var bounce = d.ValueKind == JsonValueKind.Object && d.Prop("bounce") is { ValueKind: JsonValueKind.Object } b
            ? new WebhookBounce(b.String("type"), b.String("subType"), b.String("message"))
            : null;
        var click = d.ValueKind == JsonValueKind.Object && d.Prop("click") is { ValueKind: JsonValueKind.Object } c
            ? new WebhookClick(c.String("link"), c.String("ipAddress"), c.String("userAgent"), c.Date("timestamp"))
            : null;
        var failed = d.ValueKind == JsonValueKind.Object && d.Prop("failed") is { ValueKind: JsonValueKind.Object } f
            ? new WebhookFailure(f.String("reason"))
            : null;
        return new WebhookEvent(
            body.String("type"),
            body.Date("created_at"),
            new WebhookEventData(
                d.String("email_id"),
                d.OptString("message_id") is { Length: > 0 } messageId ? messageId : null,
                d.String("from"),
                d.StringList("to"),
                d.String("subject"),
                tags,
                d.Date("created_at"),
                bounce,
                click,
                failed));
    }

    public static string Segment(string value)
    {
        if (string.IsNullOrEmpty(value)) throw new ArgumentException("An id is required.", nameof(value));
        return Uri.EscapeDataString(value);
    }

    public static KeyValuePair<string, string?>[] ListQuery(int? limit, string? after) => new[]
    {
        new KeyValuePair<string, string?>("limit", limit?.ToString(System.Globalization.CultureInfo.InvariantCulture)),
        new KeyValuePair<string, string?>("after", after),
    };
}
