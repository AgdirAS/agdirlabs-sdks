import { request, type HttpConfig } from '../http.js';
import type { Email, RequestOptions, SendBatchOptions, SendBatchResult, SendEmailInput, SendResult } from '../types.js';
import * as wire from '../wire.js';

export const MAX_BATCH_SIZE = 100;

export class Emails {
	constructor(private readonly http: HttpConfig) {}

	/** Accepts one email for delivery (or for `scheduledAt`). */
	async send(input: SendEmailInput, options: RequestOptions = {}): Promise<SendResult> {
		const idempotencyKey = this.idempotencyKey(options);
		const res = await request<{ id: string }>(this.http, {
			method: 'POST',
			path: '/emails',
			body: wire.sendEmailBody(input),
			headers: { 'idempotency-key': idempotencyKey },
			retryable: idempotencyKey !== undefined,
			signal: options.signal
		});
		return { id: res.id };
	}

	/** Accepts up to 100 emails in one request. Strict validation is all-or-nothing. */
	async sendBatch(inputs: SendEmailInput[], options: SendBatchOptions = {}): Promise<SendBatchResult> {
		const idempotencyKey = this.idempotencyKey(options);
		const res = await request<{ data: { id: string }[]; errors?: { index: number; message: string }[] }>(this.http, {
			method: 'POST',
			path: '/emails/batch',
			body: inputs.map(wire.sendEmailBody),
			headers: {
				'idempotency-key': idempotencyKey,
				'x-batch-validation': options.validation === 'permissive' ? 'permissive' : undefined
			},
			retryable: idempotencyKey !== undefined,
			signal: options.signal
		});
		return { data: res.data.map((d) => ({ id: d.id })), errors: res.errors ?? [] };
	}

	/** Reads an email and its current delivery status. Requires a full-access key. */
	async get(id: string, options: { signal?: AbortSignal } = {}): Promise<Email> {
		const res = await request<Record<string, unknown>>(this.http, {
			method: 'GET',
			path: `/emails/${encodeURIComponent(id)}`,
			retryable: true,
			signal: options.signal
		});
		return wire.email(res);
	}

	/** Cancels a scheduled email that has not been sent yet. */
	async cancel(id: string, options: { signal?: AbortSignal } = {}): Promise<{ id: string }> {
		const res = await request<{ id: string }>(this.http, {
			method: 'POST',
			path: `/emails/${encodeURIComponent(id)}/cancel`,
			retryable: true,
			signal: options.signal
		});
		return { id: res.id };
	}

	private idempotencyKey(options: RequestOptions): string | undefined {
		if (options.idempotencyKey) return options.idempotencyKey;
		// Without a key a retried send could be delivered twice, so only generate one when we may retry.
		return this.http.maxRetries > 0 ? crypto.randomUUID() : undefined;
	}
}
