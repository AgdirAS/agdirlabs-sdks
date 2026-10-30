/** `"Name <addr@example.com>"` or a bare address. */
export type Address = string;

export interface Tag {
	name: string;
	value: string;
}

export interface SendEmailInput {
	from: Address;
	to: Address | Address[];
	subject: string;
	/** At least one of `html` and `text` is required. */
	html?: string;
	text?: string;
	cc?: Address | Address[];
	bcc?: Address | Address[];
	replyTo?: Address | Address[];
	headers?: Record<string, string>;
	/** ASCII letters, numbers, `_` and `-` only. Echoed back in webhook events. */
	tags?: Tag[];
	/** Deliver later: a `Date` or an ISO 8601 string with offset. */
	scheduledAt?: Date | string;
}

export interface RequestOptions {
	/**
	 * Makes a send safe to retry: the same key with the same body returns the original result.
	 * The client generates one automatically when retries are enabled.
	 */
	idempotencyKey?: string;
	signal?: AbortSignal;
}

export interface SendBatchOptions extends RequestOptions {
	/**
	 * `strict` (default): one invalid email rejects the whole batch.
	 * `permissive`: valid emails are accepted, invalid ones are reported in `errors`.
	 */
	validation?: 'strict' | 'permissive';
}

export interface SendResult {
	id: string;
}

export interface SendBatchResult {
	data: SendResult[];
	/** Only with `validation: 'permissive'`: emails that were not accepted, by input index. */
	errors: { index: number; message: string }[];
}

export type EmailStatus =
	| 'queued'
	| 'scheduled'
	| 'sent'
	| 'delivery_delayed'
	| 'delivered'
	| 'opened'
	| 'clicked'
	| 'bounced'
	| 'failed'
	| 'complained'
	| 'canceled';

export interface Email {
	id: string;
	from: string;
	to: string[];
	cc: string[];
	bcc: string[];
	replyTo: string[];
	subject: string;
	html: string | null;
	text: string | null;
	createdAt: Date;
	scheduledAt: Date | null;
	/** Most advanced delivery state reached so far. */
	status: EmailStatus;
}

export type DomainStatus = 'not_started' | 'pending' | 'verified' | 'failed' | 'temporary_failure';

export interface DnsRecord {
	/** What the record is for. */
	purpose: 'DKIM' | 'SPF' | 'DMARC';
	/** Relative to the registrable domain, e.g. `agdir._domainkey` or `send`. */
	name: string;
	type: 'TXT' | 'MX' | 'CNAME';
	value: string;
	ttl: string;
	priority?: number;
	status: DomainStatus;
}

export interface Domain {
	id: string;
	name: string;
	status: DomainStatus;
	region: string;
	createdAt: Date;
	openTracking: boolean;
	clickTracking: boolean;
}

export interface DomainWithRecords extends Domain {
	records: DnsRecord[];
}

export interface CreateDomainInput {
	name: string;
	region?: string;
}

export const WEBHOOK_EVENT_TYPES = [
	'email.sent',
	'email.scheduled',
	'email.delivered',
	'email.delivery_delayed',
	'email.complained',
	'email.bounced',
	'email.opened',
	'email.clicked',
	'email.failed'
] as const;
export type WebhookEventType = (typeof WEBHOOK_EVENT_TYPES)[number];

export interface Webhook {
	id: string;
	endpoint: string;
	status: 'enabled' | 'disabled';
	events: WebhookEventType[];
	createdAt: Date;
}

export interface WebhookWithSecret extends Webhook {
	/** `whsec_…` — pass it to `verifyWebhook`. */
	signingSecret: string;
}

export interface CreateWebhookInput {
	/** Public `https://` URL. */
	endpoint: string;
	events: WebhookEventType[];
}

export interface UpdateWebhookInput {
	endpoint?: string;
	events?: WebhookEventType[];
	status?: 'enabled' | 'disabled';
}

export type WebhookDeliveryStatus = 'pending' | 'attempting' | 'success' | 'failed';

export interface WebhookDelivery {
	id: string;
	type: WebhookEventType;
	status: WebhookDeliveryStatus;
	createdAt: Date;
}

export interface WebhookDeliveryDetail extends WebhookDelivery {
	nextAttemptAt: Date | null;
	/** Exact JSON body sent to your endpoint. */
	payload: unknown;
}

export interface WebhookAttempt {
	id: string;
	/** 0 when no HTTP response was received. */
	httpStatus: number;
	response: string;
	sentAt: Date;
}

export interface ListOptions {
	/** 1–100. */
	limit?: number;
	/** Cursor: the id of the last item of the previous page. */
	after?: string;
	signal?: AbortSignal;
}

export interface Page<T> {
	data: T[];
	hasMore: boolean;
	/** Pass as `after` to fetch the next page; `null` on the last page. */
	nextCursor: string | null;
}

/** An event received on your webhook endpoint, as returned by `verifyWebhook`. */
export interface WebhookEvent {
	type: WebhookEventType;
	createdAt: Date;
	data: {
		emailId: string;
		messageId?: string;
		from: string;
		to: string[];
		subject: string;
		tags: Record<string, string>;
		createdAt: Date;
		bounce?: { type: string; subType: string; message: string };
		click?: { link: string; ipAddress: string; userAgent: string; timestamp: Date };
		failed?: { reason: string };
	};
}
