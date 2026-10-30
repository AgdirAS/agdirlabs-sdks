import type { HttpConfig } from './http.js';
import { Domains } from './resources/domains.js';
import { Emails } from './resources/emails.js';
import { Webhooks } from './resources/webhooks.js';

export const DEFAULT_BASE_URL = 'https://api.agdir.email';

export interface AgdirEmailOptions {
	/** Defaults to the `AGDIR_EMAIL_API_KEY` environment variable. */
	apiKey?: string;
	/** Defaults to `AGDIR_EMAIL_BASE_URL`, else https://api.agdir.email. */
	baseUrl?: string;
	/** Retries for rate limits, server errors and network failures. Default 2; 0 disables. */
	maxRetries?: number;
	/** Per-attempt timeout. Default 30 000 ms. */
	timeoutMs?: number;
	/** Custom `fetch` (proxies, tests). Defaults to the global one. */
	fetch?: typeof fetch;
}

function env(name: string): string | undefined {
	// Empty values (e.g. `AGDIR_EMAIL_BASE_URL=` in a .env file) count as unset.
	return (typeof process !== 'undefined' && process.env?.[name]) || undefined;
}

/**
 * Agdir Email API client.
 *
 * ```ts
 * const agdir = new AgdirEmail({ apiKey: 're_…' });
 * await agdir.emails.send({ from, to, subject, html });
 * ```
 */
export class AgdirEmail {
	readonly emails: Emails;
	readonly domains: Domains;
	readonly webhooks: Webhooks;
	readonly baseUrl: string;

	constructor(options: AgdirEmailOptions | string = {}) {
		const opts = typeof options === 'string' ? { apiKey: options } : options;
		const apiKey = opts.apiKey ?? env('AGDIR_EMAIL_API_KEY');
		if (!apiKey) {
			throw new Error('Missing API key: pass { apiKey } or set AGDIR_EMAIL_API_KEY. Create one at https://app.agdir.email.');
		}
		const fetchImpl = opts.fetch ?? globalThis.fetch;
		if (!fetchImpl) throw new Error('No fetch implementation found: use Node.js 20+ or pass { fetch }.');
		const maxRetries = opts.maxRetries ?? 2;
		if (!Number.isInteger(maxRetries) || maxRetries < 0) throw new Error('maxRetries must be a non-negative integer.');

		this.baseUrl = opts.baseUrl ?? env('AGDIR_EMAIL_BASE_URL') ?? DEFAULT_BASE_URL;
		const http: HttpConfig = {
			apiKey,
			baseUrl: this.baseUrl,
			fetch: fetchImpl,
			maxRetries,
			timeoutMs: opts.timeoutMs ?? 30_000
		};
		this.emails = new Emails(http);
		this.domains = new Domains(http);
		this.webhooks = new Webhooks(http);
	}
}
