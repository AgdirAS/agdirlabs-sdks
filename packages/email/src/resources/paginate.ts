import type { ListOptions, Page } from '../types.js';

/** Walks every page of a list endpoint: `for await (const item of paginate(opts => list(opts)))`. */
export async function* paginate<T>(
	fetchPage: (options: ListOptions) => Promise<Page<T>>,
	options: ListOptions = {}
): AsyncGenerator<T, void, undefined> {
	let after = options.after;
	for (;;) {
		const page = await fetchPage({ ...options, after });
		yield* page.data;
		if (!page.nextCursor) return;
		after = page.nextCursor;
	}
}
