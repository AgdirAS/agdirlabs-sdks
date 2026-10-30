using System.Collections.Generic;

namespace AgdirLabs.Email;

/// <summary>One page of a list endpoint.</summary>
/// <typeparam name="T">Item type.</typeparam>
/// <param name="Data">Items on this page.</param>
/// <param name="HasMore">Whether more items follow.</param>
/// <param name="NextCursor">Pass as <c>after</c> to fetch the next page; <see langword="null"/> on the last page.</param>
public sealed record Page<T>(IReadOnlyList<T> Data, bool HasMore, string? NextCursor);

/// <summary>Identifies the object an operation acted on.</summary>
/// <param name="Id">The object's id.</param>
public sealed record ResourceRef(string Id);

/// <summary>Returned when an object is deleted.</summary>
/// <param name="Id">The deleted object's id.</param>
/// <param name="Deleted">Always <see langword="true"/>.</param>
public sealed record DeletedResource(string Id, bool Deleted);
