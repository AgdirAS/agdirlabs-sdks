import { request, type HttpConfig } from '../http.js';
import type { CreateDomainInput, Domain, DomainWithRecords, ListOptions, Page } from '../types.js';
import * as wire from '../wire.js';
import { paginate } from './paginate.js';

type Signal = { signal?: AbortSignal };

export class Domains {
	constructor(private readonly http: HttpConfig) {}

	/** Adds a sending domain. Publish the returned `records` in DNS, then call `verify`. */
	async create(input: CreateDomainInput, options: Signal = {}): Promise<DomainWithRecords> {
		const res = await request<Record<string, unknown>>(this.http, {
			method: 'POST',
			path: '/domains',
			body: { name: input.name, region: input.region },
			retryable: false,
			signal: options.signal
		});
		return wire.domainWithRecords(res);
	}

	async get(id: string, options: Signal = {}): Promise<DomainWithRecords> {
		const res = await request<Record<string, unknown>>(this.http, {
			method: 'GET',
			path: `/domains/${encodeURIComponent(id)}`,
			retryable: true,
			signal: options.signal
		});
		return wire.domainWithRecords(res);
	}

	async list(options: ListOptions = {}): Promise<Page<Domain>> {
		const res = await request<Record<string, unknown>>(this.http, {
			method: 'GET',
			path: '/domains',
			query: { limit: options.limit, after: options.after },
			retryable: true,
			signal: options.signal
		});
		return wire.page(res, wire.domain);
	}

	/** Every domain, fetching further pages as you iterate. */
	listAll(options: ListOptions = {}): AsyncGenerator<Domain, void, undefined> {
		return paginate((o) => this.list(o), options);
	}

	/** Starts a DNS check now instead of waiting for the next automatic one. */
	async verify(id: string, options: Signal = {}): Promise<{ id: string }> {
		const res = await request<{ id: string }>(this.http, {
			method: 'POST',
			path: `/domains/${encodeURIComponent(id)}/verify`,
			retryable: true,
			signal: options.signal
		});
		return { id: res.id };
	}

	async delete(id: string, options: Signal = {}): Promise<{ id: string; deleted: true }> {
		const res = await request<{ id: string }>(this.http, {
			method: 'DELETE',
			path: `/domains/${encodeURIComponent(id)}`,
			retryable: false,
			signal: options.signal
		});
		return { id: res.id, deleted: true };
	}
}
