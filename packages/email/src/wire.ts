// Mapping between the SDK's camelCase objects and the API's snake_case JSON.
import type {
	DnsRecord,
	Domain,
	DomainWithRecords,
	Email,
	EmailStatus,
	Page,
	SendEmailInput,
	Webhook,
	WebhookAttempt,
	WebhookDelivery,
	WebhookDeliveryDetail,
	WebhookEvent,
	WebhookWithSecret
} from './types.js';

type Json = Record<string, any>;

const date = (v: string) => new Date(v);
const optionalDate = (v: string | null | undefined) => (v ? new Date(v) : null);
const list = (v: string | string[] | null | undefined) => (v == null ? [] : Array.isArray(v) ? v : [v]);

export function sendEmailBody(input: SendEmailInput): Json {
	const scheduledAt = input.scheduledAt instanceof Date ? input.scheduledAt.toISOString() : input.scheduledAt;
	return {
		from: input.from,
		to: input.to,
		subject: input.subject,
		html: input.html,
		text: input.text,
		cc: input.cc,
		bcc: input.bcc,
		reply_to: input.replyTo,
		headers: input.headers,
		tags: input.tags,
		scheduled_at: scheduledAt
	};
}

export const email = (e: Json): Email => ({
	id: e.id,
	from: e.from,
	to: list(e.to),
	cc: list(e.cc),
	bcc: list(e.bcc),
	replyTo: list(e.reply_to),
	subject: e.subject,
	html: e.html ?? null,
	text: e.text ?? null,
	createdAt: date(e.created_at),
	scheduledAt: optionalDate(e.scheduled_at),
	status: e.last_event as EmailStatus
});

const record = (r: Json): DnsRecord => ({
	purpose: r.record,
	name: r.name,
	type: r.type,
	value: r.value,
	ttl: r.ttl,
	...(r.priority !== undefined ? { priority: r.priority } : {}),
	status: r.status
});

export const domain = (d: Json): Domain => ({
	id: d.id,
	name: d.name,
	status: d.status,
	region: d.region,
	createdAt: date(d.created_at),
	openTracking: Boolean(d.open_tracking),
	clickTracking: Boolean(d.click_tracking)
});

export const domainWithRecords = (d: Json): DomainWithRecords => ({
	...domain(d),
	records: (d.records ?? []).map(record)
});

export const webhook = (w: Json): Webhook => ({
	id: w.id,
	endpoint: w.endpoint,
	status: w.status,
	events: w.events,
	createdAt: date(w.created_at)
});

export const webhookWithSecret = (w: Json): WebhookWithSecret => ({ ...webhook(w), signingSecret: w.signing_secret });

export const delivery = (e: Json): WebhookDelivery => ({
	id: e.id,
	type: e.type,
	status: e.status,
	createdAt: date(e.created_at)
});

export const deliveryDetail = (e: Json): WebhookDeliveryDetail => ({
	...delivery(e),
	nextAttemptAt: optionalDate(e.next_attempt_at),
	payload: e.payload
});

export const attempt = (a: Json): WebhookAttempt => ({
	id: a.id,
	httpStatus: a.http_status_code,
	response: a.response,
	sentAt: date(a.sent_at)
});

export function page<T>(body: Json, map: (item: Json) => T): Page<T> {
	const data = (body.data as Json[]).map(map);
	const last = (body.data as Json[]).at(-1);
	const hasMore = Boolean(body.has_more);
	return { data, hasMore, nextCursor: hasMore && last ? String(last.id) : null };
}

export function webhookEvent(body: Json): WebhookEvent {
	const d: Json = body.data ?? {};
	return {
		type: body.type,
		createdAt: date(body.created_at),
		data: {
			emailId: d.email_id,
			...(d.message_id ? { messageId: d.message_id } : {}),
			from: d.from,
			to: list(d.to),
			subject: d.subject,
			tags: d.tags ?? {},
			createdAt: date(d.created_at),
			...(d.bounce ? { bounce: { type: d.bounce.type, subType: d.bounce.subType, message: d.bounce.message } } : {}),
			...(d.click
				? {
						click: {
							link: d.click.link,
							ipAddress: d.click.ipAddress,
							userAgent: d.click.userAgent,
							timestamp: date(d.click.timestamp)
						}
					}
				: {}),
			...(d.failed ? { failed: { reason: d.failed.reason } } : {})
		}
	};
}
