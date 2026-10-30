import { AgdirEmailError } from './errors.js';
import { VERSION } from './version.js';

export interface HttpConfig {
	apiKey: string;
	baseUrl: string;
	fetch: typeof fetch;
	maxRetries: number;
	timeoutMs: number;
}

export interface HttpRequest {
	method: 'GET' | 'POST' | 'PATCH' | 'DELETE';
	path: string;
	query?: Record<string, string | number | undefined>;
	body?: unknown;
	headers?: Record<string, string | undefined>;
	/** Safe to repeat: GETs, and POSTs that carry an idempotency key. */
	retryable: boolean;
	signal?: AbortSignal;
}

const RETRY_STATUSES = new Set([408, 500, 502, 503, 504]);
const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

/** Exponential backoff with jitter: ~0.5 s, 1 s, 2 s … capped at 8 s. */
function backoffMs(attempt: number): number {
	const base = Math.min(8000, 500 * 2 ** attempt);
	return base / 2 + Math.random() * (base / 2);
}

function retryAfterSeconds(res: Response): number | undefined {
	const raw = res.headers.get('retry-after');
	if (!raw) return undefined;
	const seconds = Number(raw);
	if (Number.isFinite(seconds)) return Math.max(0, seconds);
	const date = Date.parse(raw);
	return Number.isNaN(date) ? undefined : Math.max(0, (date - Date.now()) / 1000);
}

async function toError(res: Response): Promise<AgdirEmailError> {
	let body: { name?: string; message?: string } = {};
	try {
		body = (await res.json()) as typeof body;
	} catch {
		// Not JSON (e.g. a proxy error page) — fall back to the status line.
	}
	return new AgdirEmailError(
		body.message ?? `Request failed with HTTP ${res.status}`,
		body.name ?? (res.status >= 500 ? 'internal_server_error' : 'unknown_error'),
		res.status,
		retryAfterSeconds(res)
	);
}

/** Sends one API request, retrying rate limits, server errors and network failures when safe. */
export async function request<T>(config: HttpConfig, req: HttpRequest): Promise<T> {
	const url = new URL(config.baseUrl.replace(/\/+$/, '') + req.path);
	for (const [key, value] of Object.entries(req.query ?? {})) {
		if (value !== undefined) url.searchParams.set(key, String(value));
	}
	const headers: Record<string, string> = {
		authorization: `Bearer ${config.apiKey}`,
		'user-agent': `agdirlabs-email-node/${VERSION}`,
		accept: 'application/json'
	};
	if (req.body !== undefined) headers['content-type'] = 'application/json';
	for (const [key, value] of Object.entries(req.headers ?? {})) if (value !== undefined) headers[key] = value;

	for (let attempt = 0; ; attempt++) {
		const canRetry = attempt < config.maxRetries;
		const timeout = AbortSignal.timeout(config.timeoutMs);
		const signal = req.signal ? AbortSignal.any([req.signal, timeout]) : timeout;

		let res: Response;
		try {
			res = await config.fetch(url, {
				method: req.method,
				headers,
				body: req.body === undefined ? undefined : JSON.stringify(req.body),
				signal
			});
		} catch (cause) {
			if (req.signal?.aborted) throw cause;
			const timedOut = timeout.aborted;
			// A timed-out POST may have been accepted: only retry it when it is idempotent.
			if (canRetry && req.retryable) {
				await sleep(backoffMs(attempt));
				continue;
			}
			throw new AgdirEmailError(
				timedOut ? `Request timed out after ${config.timeoutMs} ms` : 'Could not reach the Agdir Email API',
				timedOut ? 'timeout' : 'network_error',
				0,
				undefined,
				{ cause }
			);
		}

		if (res.ok) return (await res.json()) as T;

		const error = await toError(res);
		// Rate limits are refused before any work is done, so they are always safe to retry.
		const retry =
			canRetry && (error.code === 'rate_limit_exceeded' || (req.retryable && RETRY_STATUSES.has(res.status)));
		if (!retry) throw error;
		await sleep(error.retryAfter !== undefined ? error.retryAfter * 1000 : backoffMs(attempt));
	}
}
