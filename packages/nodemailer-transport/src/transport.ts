import { AgdirEmail, AgdirEmailError, type AgdirEmailOptions, type SendEmailInput, type Tag } from '@agdirlabs/email';
import { VERSION } from './version.js';

/** The part of `AgdirEmail` the transport uses (handy for tests and wrappers). */
export interface AgdirEmailSender {
	emails: {
		send(input: SendEmailInput, options?: { idempotencyKey?: string }): Promise<{ id: string }>;
	};
}

export type AgdirTransportOptions =
	| (AgdirEmailOptions & { client?: undefined })
	| {
			/** An existing client; `apiKey`, `baseUrl` etc. are then ignored. */
			client: AgdirEmailSender;
	  };

/** Extra message fields understood by the transport (alongside the standard Nodemailer ones). */
export interface AgdirMailOptions {
	/** Tags echoed back in webhook events: `[{ name, value }]` or `{ name: value }`. ASCII letters, digits, `_`, `-`. */
	tags?: Tag[] | Record<string, string>;
	/** Deliver later: a `Date` or an ISO 8601 string with offset. */
	scheduledAt?: Date | string;
	/** Makes the send safe to repeat: the same key and message return the original email id. */
	idempotencyKey?: string;
}

/** Passed to the `sendMail` callback / resolved by its promise. */
export interface AgdirSentMessageInfo {
	/** The Agdir Email id; use it with `agdir.emails.get()` / `cancel()`. */
	messageId: string;
	envelope: { from: string | false; to: string[] };
	accepted: string[];
	rejected: string[];
	pending: string[];
	response: string;
}

/** Control headers: read by the transport and never sent with the email. */
export const TAGS_HEADER = 'x-agdir-tags';
export const SCHEDULED_AT_HEADER = 'x-agdir-scheduled-at';
export const IDEMPOTENCY_KEY_HEADER = 'x-agdir-idempotency-key';
const CONTROL_HEADERS = new Set([TAGS_HEADER, SCHEDULED_AT_HEADER, IDEMPOTENCY_KEY_HEADER]);

// Minimal structural view of Nodemailer's MailMessage, so the public types don't depend on a
// particular Nodemailer (or @types/nodemailer) version.
interface ParsedAddress {
	name?: string;
	address?: string;
	group?: ParsedAddress[];
}
interface MailMessageLike {
	data: Record<string, any>;
	message: { getEnvelope(): { from: string | false; to: string[] } };
	normalize(callback: (err: Error | null | undefined, data?: Record<string, any>) => void): void;
}

/** A Nodemailer transport; pass it to `nodemailer.createTransport()`. */
export class AgdirTransport {
	readonly name = 'AgdirEmail';
	readonly version = VERSION;
	readonly client: AgdirEmailSender;

	constructor(options: AgdirTransportOptions = {}) {
		this.client = options.client ?? new AgdirEmail(options as AgdirEmailOptions);
	}

	send(mail: MailMessageLike, callback: (err: Error | null, info?: AgdirSentMessageInfo) => void): void {
		this.sendAsync(mail).then(
			(info) => callback(null, info),
			(err: unknown) => callback(toNodemailerError(err))
		);
	}

	private async sendAsync(mail: MailMessageLike): Promise<AgdirSentMessageInfo> {
		rejectUnsupported(mail.data);
		const envelope = mail.message.getEnvelope();
		// Resolves html/text given as Buffers, streams, files or URLs, and parses every address field.
		const data = await new Promise<Record<string, any>>((resolve, reject) =>
			mail.normalize((err, normalized) => (err ? reject(err) : resolve(normalized!)))
		);
		const { input, idempotencyKey } = toSendInput(data);
		const { id } = await this.client.emails.send(input, idempotencyKey ? { idempotencyKey } : {});
		return {
			messageId: id,
			envelope,
			accepted: [...envelope.to],
			rejected: [],
			pending: [],
			response: `Accepted ${id}`
		};
	}
}

/**
 * Creates a Nodemailer transport that sends through the Agdir Email API.
 *
 * ```ts
 * const transporter = nodemailer.createTransport(agdirTransport({ apiKey: 're_…' }));
 * ```
 */
export function agdirTransport(options: AgdirTransportOptions = {}): AgdirTransport {
	return new AgdirTransport(options);
}

function unsupported(message: string): AgdirEmailError {
	return new AgdirEmailError(message, 'unsupported', 0);
}

function invalid(message: string): AgdirEmailError {
	return new AgdirEmailError(message, 'validation_error', 0);
}

function rejectUnsupported(data: Record<string, any>): void {
	if (Array.isArray(data.attachments) ? data.attachments.length > 0 : data.attachments) {
		throw unsupported('Attachments are not supported by the Agdir Email API yet. Link to the file from the email body instead.');
	}
	if (data.raw) throw unsupported('Raw MIME messages (`raw`) are not supported by the Agdir Email API.');
	if (Array.isArray(data.alternatives) ? data.alternatives.length > 0 : data.alternatives) {
		throw unsupported('`alternatives` are not supported by the Agdir Email API; use `html` and `text`.');
	}
	if (data.icalEvent) throw unsupported('Calendar events (`icalEvent`) are not supported by the Agdir Email API yet.');
	if (data.envelope) {
		throw unsupported('A custom SMTP `envelope` is not supported: the email is sent to its to, cc and bcc recipients.');
	}
}

function toSendInput(data: Record<string, any>): { input: SendEmailInput; idempotencyKey?: string } {
	const headers: Record<string, string> = {};
	const control: Record<string, string> = {};
	for (const [key, value] of headerEntries(data.headers)) {
		const lower = key.toLowerCase();
		if (CONTROL_HEADERS.has(lower)) control[lower] = value;
		else headers[key] = key in headers ? `${headers[key]}, ${value}` : value;
	}
	// Threading and List-* headers, as Nodemailer formats them from inReplyTo / references / list.
	for (const [key, value] of Object.entries((data.normalizedHeaders ?? {}) as Record<string, unknown>)) {
		if ((key === 'in-reply-to' || key === 'references' || key.startsWith('list-')) && typeof value === 'string' && value) {
			const existing = Object.keys(headers).find((k) => k.toLowerCase() === key);
			headers[existing ?? canonicalHeaderName(key)] = value;
		}
	}
	Object.assign(headers, priorityHeaders(data.priority));

	const tags = [...toTags(data.tags), ...(control[TAGS_HEADER] ? parseTagsHeader(control[TAGS_HEADER]) : [])];
	const scheduledAt: Date | string | undefined = data.scheduledAt ?? control[SCHEDULED_AT_HEADER];
	const idempotencyKey: string | undefined = data.idempotencyKey ?? control[IDEMPOTENCY_KEY_HEADER];

	const input: SendEmailInput = {
		from: formatAddresses(data.from)[0] as string,
		to: formatAddresses(data.to),
		subject: data.subject as string
	};
	const html = content(data.html);
	const text = content(data.text);
	if (html !== undefined) input.html = html;
	if (text !== undefined) input.text = text;
	const cc = formatAddresses(data.cc);
	const bcc = formatAddresses(data.bcc);
	const replyTo = formatAddresses(data.replyTo);
	if (cc.length) input.cc = cc;
	if (bcc.length) input.bcc = bcc;
	if (replyTo.length) input.replyTo = replyTo;
	if (Object.keys(headers).length) input.headers = headers;
	if (tags.length) input.tags = tags;
	if (scheduledAt !== undefined && scheduledAt !== '') input.scheduledAt = scheduledAt;
	return { input, idempotencyKey: idempotencyKey || undefined };
}

/** Nodemailer headers: `{ key: value | value[] | { prepared, value } }` or `[{ key, value }]`. */
function headerEntries(headers: unknown): [string, string][] {
	const entries: [string, unknown][] = [];
	if (Array.isArray(headers)) {
		for (const h of headers) if (h && typeof h.key === 'string') entries.push([h.key, h.value]);
	} else if (headers && typeof headers === 'object') {
		for (const [key, value] of Object.entries(headers)) {
			for (const v of Array.isArray(value) ? value : [value]) entries.push([key, v]);
		}
	}
	const out: [string, string][] = [];
	for (const [key, raw] of entries) {
		const value = raw && typeof raw === 'object' && 'value' in raw ? (raw as { value: unknown }).value : raw;
		if (value === undefined || value === null || value === false || value === '') continue;
		out.push([key, String(value)]);
	}
	return out;
}

function canonicalHeaderName(key: string): string {
	if (key === 'in-reply-to') return 'In-Reply-To';
	return key.replace(/(^|-)([a-z])/g, (_, dash: string, c: string) => dash + c.toUpperCase());
}

function priorityHeaders(priority: unknown): Record<string, string> {
	switch (String(priority ?? '').toLowerCase()) {
		case 'high':
			return { 'X-Priority': '1 (Highest)', 'X-MSMail-Priority': 'High', Importance: 'High' };
		case 'low':
			return { 'X-Priority': '5 (Lowest)', 'X-MSMail-Priority': 'Low', Importance: 'Low' };
		default:
			return {};
	}
}

function toTags(tags: unknown): Tag[] {
	if (!tags) return [];
	if (Array.isArray(tags)) return tags.map((t: Tag) => ({ name: String(t.name), value: String(t.value) }));
	if (typeof tags === 'object') return Object.entries(tags).map(([name, value]) => ({ name, value: String(value) }));
	throw invalid('`tags` must be an array of { name, value } or an object of name → value.');
}

/** `X-Agdir-Tags: category=welcome, plan=pro` (`,` or `;` separated). */
export function parseTagsHeader(value: string): Tag[] {
	return value
		.split(/[,;]/)
		.map((part) => part.trim())
		.filter(Boolean)
		.map((part) => {
			const eq = part.indexOf('=');
			if (eq <= 0) throw invalid(`Invalid X-Agdir-Tags entry "${part}": expected name=value.`);
			return { name: part.slice(0, eq).trim(), value: part.slice(eq + 1).trim() };
		});
}

function content(value: unknown): string | undefined {
	if (value === undefined || value === null || value === false) return undefined;
	if (typeof value === 'string') return value;
	if (Buffer.isBuffer(value)) return value.toString();
	if (typeof value === 'object' && 'content' in value) return content((value as { content: unknown }).content);
	return String(value);
}

/** Normalized Nodemailer addresses (`{ name, address }`, groups, arrays) → `"Name <addr>"` strings. */
export function formatAddresses(value: unknown): string[] {
	if (!value) return [];
	const out: string[] = [];
	for (const entry of Array.isArray(value) ? value : [value]) {
		if (!entry) continue;
		if (typeof entry === 'string') {
			out.push(entry);
			continue;
		}
		const parsed = entry as ParsedAddress;
		if (parsed.group) {
			out.push(...formatAddresses(parsed.group));
		} else if (parsed.address) {
			out.push(parsed.name ? `${quoteName(parsed.name)} <${parsed.address}>` : parsed.address);
		}
	}
	return out;
}

function quoteName(name: string): string {
	// Atoms (letters, digits, spaces and a few symbols) can go bare; anything else is quoted.
	return /^[\w !#$%&'*+\-/=?^`{|}~]*$/.test(name) ? name : `"${name.replace(/["\\]/g, (c) => `\\${c}`)}"`;
}

function toNodemailerError(err: unknown): Error {
	if (err instanceof AgdirEmailError) {
		// Nodemailer's own transports report the server status as `responseCode`.
		if (err.status) Object.assign(err, { responseCode: err.status });
		return err;
	}
	return err instanceof Error ? err : new Error(String(err));
}
