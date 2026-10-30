using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Text.Json;

namespace AgdirLabs.Email.Internal;

/// <summary>Reflection-free JSON helpers (trim/AOT safe): the API is snake_case, the SDK is not.</summary>
internal static class Json
{
    public static JsonElement? Prop(this JsonElement e, string name) =>
        e.ValueKind == JsonValueKind.Object && e.TryGetProperty(name, out var v) && v.ValueKind != JsonValueKind.Null && v.ValueKind != JsonValueKind.Undefined
            ? v
            : null;

    public static string? OptString(this JsonElement e, string name) => e.Prop(name) is { } v
        ? v.ValueKind == JsonValueKind.String ? v.GetString() : v.GetRawText()
        : null;

    public static string String(this JsonElement e, string name) => e.OptString(name) ?? "";

    public static int? OptInt(this JsonElement e, string name) =>
        e.Prop(name) is { ValueKind: JsonValueKind.Number } v && v.TryGetInt32(out var i) ? i : null;

    public static bool Bool(this JsonElement e, string name) => e.Prop(name) is { ValueKind: JsonValueKind.True };

    public static DateTimeOffset? OptDate(this JsonElement e, string name) =>
        e.OptString(name) is { Length: > 0 } s
            ? DateTimeOffset.Parse(s, CultureInfo.InvariantCulture, DateTimeStyles.AssumeUniversal)
            : null;

    public static DateTimeOffset Date(this JsonElement e, string name) => e.OptDate(name) ?? default;

    /// <summary>A string, an array of strings, or null (→ empty).</summary>
    public static IReadOnlyList<string> StringList(this JsonElement e, string name)
    {
        if (e.Prop(name) is not { } v) return Array.Empty<string>();
        if (v.ValueKind == JsonValueKind.String) return new[] { v.GetString()! };
        if (v.ValueKind != JsonValueKind.Array) return Array.Empty<string>();
        var list = new List<string>(v.GetArrayLength());
        foreach (var item in v.EnumerateArray())
        {
            if (item.ValueKind == JsonValueKind.String) list.Add(item.GetString()!);
        }
        return list;
    }

    public static IReadOnlyList<T> List<T>(this JsonElement e, string name, Func<JsonElement, T> map)
    {
        if (e.Prop(name) is not { ValueKind: JsonValueKind.Array } v) return Array.Empty<T>();
        var list = new List<T>(v.GetArrayLength());
        foreach (var item in v.EnumerateArray()) list.Add(map(item));
        return list;
    }

    /// <summary>Parses a snake_case wire value (e.g. <c>delivery_delayed</c>) into a PascalCase enum member; unknown values map to 0.</summary>
    public static T Enum<T>(this JsonElement e, string name) where T : struct, System.Enum =>
        ParseEnum<T>(e.OptString(name));

    public static T ParseEnum<T>(string? value) where T : struct, System.Enum =>
        value is { Length: > 0 } && System.Enum.TryParse<T>(value.Replace("_", ""), ignoreCase: true, out var result) && !int.TryParse(value, out _)
            ? result
            : default;

    /// <summary>PascalCase enum member → snake_case wire value.</summary>
    public static string ToWire<T>(T value) where T : struct, System.Enum
    {
        var name = value.ToString();
        var sb = new System.Text.StringBuilder(name.Length + 4);
        for (var i = 0; i < name.Length; i++)
        {
            var c = name[i];
            if (char.IsUpper(c))
            {
                if (i > 0) sb.Append('_');
                sb.Append(char.ToLowerInvariant(c));
            }
            else
            {
                sb.Append(c);
            }
        }
        return sb.ToString();
    }

    public static byte[] Write(Action<Utf8JsonWriter> write)
    {
        using var stream = new MemoryStream();
        using (var writer = new Utf8JsonWriter(stream))
        {
            write(writer);
        }
        return stream.ToArray();
    }

    public static void WriteOptional(this Utf8JsonWriter w, string name, string? value)
    {
        if (value is not null) w.WriteString(name, value);
    }

    public static void WriteStrings(this Utf8JsonWriter w, string name, IEnumerable<string>? values)
    {
        if (values is null) return;
        w.WriteStartArray(name);
        foreach (var v in values) w.WriteStringValue(v);
        w.WriteEndArray();
    }

    /// <summary>ISO 8601 in UTC with millisecond precision, e.g. <c>2026-11-01T09:00:00.000Z</c>.</summary>
    public static string Iso(DateTimeOffset value) =>
        value.UtcDateTime.ToString("yyyy-MM-dd'T'HH:mm:ss.fff'Z'", CultureInfo.InvariantCulture);
}
