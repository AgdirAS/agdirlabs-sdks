using System;
using System.Collections.Generic;
using System.Runtime.CompilerServices;
using System.Threading;
using System.Threading.Tasks;

namespace AgdirLabs.Email.Internal;

internal static class Paginator
{
    /// <summary>Walks every page of a list endpoint, fetching the next page as the caller iterates.</summary>
    public static async IAsyncEnumerable<T> All<T>(
        Func<string?, CancellationToken, Task<Page<T>>> fetchPage,
        [EnumeratorCancellation] CancellationToken cancellationToken = default)
    {
        string? after = null;
        while (true)
        {
            var page = await fetchPage(after, cancellationToken).ConfigureAwait(false);
            foreach (var item in page.Data) yield return item;
            if (page.NextCursor is null) yield break;
            after = page.NextCursor;
        }
    }
}
