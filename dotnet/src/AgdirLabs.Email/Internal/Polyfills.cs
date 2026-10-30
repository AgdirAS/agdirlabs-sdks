#if NETSTANDARD2_0
// Lets the compiler emit `init` accessors and records when targeting netstandard2.0.
namespace System.Runtime.CompilerServices;

internal static class IsExternalInit
{
}
#endif
