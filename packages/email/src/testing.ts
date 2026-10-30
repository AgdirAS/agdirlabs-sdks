// Test helper (not exported from the package): a scripted `fetch` that records requests.
export interface RecordedRequest {
	method: string;
	url: URL;
	headers: Headers;
	body: unknown;
}

type Reply = { status?: number; body?: unknown; headers?: Record<string, string> } | Error;

export function fakeFetch(...replies: Reply[]) {
	const requests: RecordedRequest[] = [];
	const fetch = async (input: string | URL | Request, init: RequestInit = {}): Promise<Response> => {
		requests.push({
			method: init.method ?? 'GET',
			url: new URL(String(input)),
			headers: new Headers(init.headers),
			body: typeof init.body === 'string' ? JSON.parse(init.body) : undefined
		});
		const reply = replies.shift();
		if (!reply) throw new Error('fakeFetch: no reply scripted for this request');
		if (reply instanceof Error) throw reply;
		return new Response(JSON.stringify(reply.body ?? {}), {
			status: reply.status ?? 200,
			headers: { 'content-type': 'application/json', ...reply.headers }
		});
	};
	return { fetch: fetch as typeof globalThis.fetch, requests };
}
