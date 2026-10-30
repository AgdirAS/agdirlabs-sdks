import { request, type HttpConfig } from '../http.js';
import type {
	CreateWebhookInput,
	ListOptions,
	Page,
	UpdateWebhookInput,
	Webhook,
	WebhookAttempt,
	WebhookDelivery,
	WebhookDeliveryDetail,
	WebhookWithSecret
} from '../types.js';
import * as wire from '../wire.js';
import { paginate } from './paginate.js';

type Signal = { signal?: AbortSignal };
type Json = Record<string, unknown>;
const seg = encodeURIComponent;

/** Messages sent (or queued) to one webhook endpoint, with their HTTP attempts. */
export class WebhookDeliveries {
	constructor(private readonly http: HttpConfig) {}

	async list(webhookId: string, options: ListOptions = {}): Promise<Page<WebhookDelivery>> {
		const res = await request<Json>(this.http, {
			method: 'GET',
			path: `/webhooks/${seg(webhookId)}/events`,
			query: { limit: options.limit, after: options.after },
			retryable: true,
			signal: options.signal
		});
		return wire.page(res, wire.delivery);
	}

	listAll(webhookId: string, options: ListOptions = {}): AsyncGenerator<WebhookDelivery, void, undefined> {
		return paginate((o) => this.list(webhookId, o), options);
	}

	async get(webhookId: string, deliveryId: string, options: Signal = {}): Promise<WebhookDeliveryDetail> {
		const res = await request<Json>(this.http, {
			method: 'GET',
			path: `/webhooks/${seg(webhookId)}/events/${seg(deliveryId)}`,
			retryable: true,
			signal: options.signal
		});
		return wire.deliveryDetail(res);
	}

	/** HTTP attempts for one delivery, newest first. */
	async attempts(webhookId: string, deliveryId: string, options: ListOptions = {}): Promise<Page<WebhookAttempt>> {
		const res = await request<Json>(this.http, {
			method: 'GET',
			path: `/webhooks/${seg(webhookId)}/events/${seg(deliveryId)}/attempts`,
			query: { limit: options.limit, after: options.after },
			retryable: true,
			signal: options.signal
		});
		return wire.page(res, wire.attempt);
	}

	/** Sends the identical message again, once, right now (also to disabled endpoints). */
	async replay(webhookId: string, deliveryId: string, options: Signal = {}): Promise<{ id: string }> {
		const res = await request<{ id: string }>(this.http, {
			method: 'POST',
			path: `/webhooks/${seg(webhookId)}/events/${seg(deliveryId)}/replay`,
			retryable: false,
			signal: options.signal
		});
		return { id: res.id };
	}
}

export class Webhooks {
	readonly deliveries: WebhookDeliveries;

	constructor(private readonly http: HttpConfig) {
		this.deliveries = new WebhookDeliveries(http);
	}

	/** Registers an endpoint. Keep the returned `signingSecret` to verify incoming events. */
	async create(input: CreateWebhookInput, options: Signal = {}): Promise<{ id: string; signingSecret: string }> {
		const res = await request<{ id: string; signing_secret: string }>(this.http, {
			method: 'POST',
			path: '/webhooks',
			body: { endpoint: input.endpoint, events: input.events },
			retryable: false,
			signal: options.signal
		});
		return { id: res.id, signingSecret: res.signing_secret };
	}

	async get(id: string, options: Signal = {}): Promise<WebhookWithSecret> {
		const res = await request<Json>(this.http, {
			method: 'GET',
			path: `/webhooks/${seg(id)}`,
			retryable: true,
			signal: options.signal
		});
		return wire.webhookWithSecret(res);
	}

	async list(options: ListOptions = {}): Promise<Page<Webhook>> {
		const res = await request<Json>(this.http, {
			method: 'GET',
			path: '/webhooks',
			query: { limit: options.limit, after: options.after },
			retryable: true,
			signal: options.signal
		});
		return wire.page(res, wire.webhook);
	}

	listAll(options: ListOptions = {}): AsyncGenerator<Webhook, void, undefined> {
		return paginate((o) => this.list(o), options);
	}

	async update(id: string, input: UpdateWebhookInput, options: Signal = {}): Promise<{ id: string }> {
		const res = await request<{ id: string }>(this.http, {
			method: 'PATCH',
			path: `/webhooks/${seg(id)}`,
			body: { endpoint: input.endpoint, events: input.events, status: input.status },
			retryable: true,
			signal: options.signal
		});
		return { id: res.id };
	}

	async delete(id: string, options: Signal = {}): Promise<{ id: string; deleted: true }> {
		const res = await request<{ id: string }>(this.http, {
			method: 'DELETE',
			path: `/webhooks/${seg(id)}`,
			retryable: false,
			signal: options.signal
		});
		return { id: res.id, deleted: true };
	}

	/** Issues a new signing secret; the old one stops working immediately. */
	async rotateSecret(id: string, options: Signal = {}): Promise<{ id: string; signingSecret: string }> {
		const res = await request<{ id: string; signing_secret: string }>(this.http, {
			method: 'POST',
			path: `/webhooks/${seg(id)}/signing-secret/rotate`,
			retryable: false,
			signal: options.signal
		});
		return { id: res.id, signingSecret: res.signing_secret };
	}
}
