import { WebhookVerificationError } from './errors.js';
import type { WebhookEvent } from './types.js';
import { webhookEvent } from './wire.js';

export interface VerifyWebhookInput {
	/** The raw request body, exactly as received — not re-serialized JSON. */
	payload: string;
	/** Request headers: a `Headers` object or a plain object (e.g. Express `req.headers`). */
	headers: Headers | Record<string, string | string[] | undefined>;
	/** The webhook's `whsec_…` signing secret. */
	secret: string;
	/** Reject messages older or newer than this many seconds. Default 300. */
	toleranceSeconds?: number;
	/** For tests. */
	now?: Date;
}

const encoder = new TextEncoder();

function header(headers: VerifyWebhookInput['headers'], name: string): string | undefined {
	if (typeof (headers as Headers).get === 'function') return (headers as Headers).get(name) ?? undefined;
	const value = (headers as Record<string, string | string[] | undefined>)[name];
	return Array.isArray(value) ? value[0] : value;
}

function base64ToBytes(b64: string): Uint8Array<ArrayBuffer> {
	const bin = atob(b64);
	const bytes = new Uint8Array(bin.length);
	for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
	return bytes;
}

function bytesToBase64(bytes: Uint8Array): string {
	let bin = '';
	for (const b of bytes) bin += String.fromCharCode(b);
	return btoa(bin);
}

function timingSafeEqual(a: string, b: string): boolean {
	if (a.length !== b.length) return false;
	let diff = 0;
	for (let i = 0; i < a.length; i++) diff |= a.charCodeAt(i) ^ b.charCodeAt(i);
	return diff === 0;
}

/**
 * Checks that a webhook request came from Agdir Email and returns the parsed event.
 * Messages are signed per the Standard Webhooks spec (`webhook-id`, `webhook-timestamp`,
 * `webhook-signature`). Throws `WebhookVerificationError` if anything doesn't match.
 * Works in Node.js 20+, Deno, Bun and edge runtimes (Web Crypto).
 */
export async function verifyWebhook(input: VerifyWebhookInput): Promise<WebhookEvent> {
	const id = header(input.headers, 'webhook-id');
	const timestamp = header(input.headers, 'webhook-timestamp');
	const signatures = header(input.headers, 'webhook-signature');
	if (!id || !timestamp || !signatures) throw new WebhookVerificationError('Missing webhook signature headers');

	const sentAt = Number(timestamp);
	const now = Math.floor((input.now ?? new Date()).getTime() / 1000);
	if (!Number.isInteger(sentAt) || Math.abs(now - sentAt) > (input.toleranceSeconds ?? 300)) {
		throw new WebhookVerificationError('Webhook timestamp is outside the allowed tolerance');
	}

	const secret = input.secret.startsWith('whsec_') ? input.secret.slice('whsec_'.length) : input.secret;
	let keyBytes: Uint8Array<ArrayBuffer>;
	try {
		keyBytes = base64ToBytes(secret);
	} catch {
		throw new WebhookVerificationError('Signing secret is not valid base64');
	}
	const key = await crypto.subtle.importKey('raw', keyBytes, { name: 'HMAC', hash: 'SHA-256' }, false, ['sign']);
	const mac = await crypto.subtle.sign('HMAC', key, encoder.encode(`${id}.${timestamp}.${input.payload}`));
	const expected = bytesToBase64(new Uint8Array(mac));

	// The header may hold several space-separated `v1,<sig>` entries (e.g. during secret rotation).
	const valid = signatures
		.split(' ')
		.some((entry) => entry.startsWith('v1,') && timingSafeEqual(entry.slice(3), expected));
	if (!valid) throw new WebhookVerificationError('Webhook signature does not match');

	try {
		return webhookEvent(JSON.parse(input.payload));
	} catch {
		throw new WebhookVerificationError('Webhook payload is not valid JSON');
	}
}
