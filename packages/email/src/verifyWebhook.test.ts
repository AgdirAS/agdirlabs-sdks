import { createHmac, randomBytes } from 'node:crypto';
import { describe, expect, it } from 'vitest';
import { verifyWebhook, WebhookVerificationError } from './index.js';

const secret = 'whsec_' + randomBytes(32).toString('base64');
const now = new Date('2026-10-30T12:00:00Z');
const payload = JSON.stringify({
	type: 'email.bounced',
	created_at: '2026-10-30T11:59:58.000Z',
	data: {
		created_at: '2026-10-30T11:59:00.000Z',
		email_id: 'em_1',
		message_id: 'ses-1',
		from: 'Acme <hello@acme.com>',
		to: ['user@example.com'],
		subject: 'Hi',
		tags: { kind: 'welcome' },
		bounce: { message: 'mailbox full', subType: 'MailboxFull', type: 'Transient' }
	}
});

function sign(body: string, opts: { id?: string; at?: Date; key?: string } = {}) {
	const id = opts.id ?? 'msg_1';
	const ts = Math.floor((opts.at ?? now).getTime() / 1000).toString();
	const key = Buffer.from((opts.key ?? secret).slice('whsec_'.length), 'base64');
	const sig = createHmac('sha256', key).update(`${id}.${ts}.${body}`).digest('base64');
	return { 'webhook-id': id, 'webhook-timestamp': ts, 'webhook-signature': `v1,${sig}` };
}

describe('verifyWebhook', () => {
	it('accepts a valid signature and returns the camelCase event', async () => {
		const event = await verifyWebhook({ payload, headers: sign(payload), secret, now });
		expect(event).toEqual({
			type: 'email.bounced',
			createdAt: new Date('2026-10-30T11:59:58.000Z'),
			data: {
				emailId: 'em_1',
				messageId: 'ses-1',
				from: 'Acme <hello@acme.com>',
				to: ['user@example.com'],
				subject: 'Hi',
				tags: { kind: 'welcome' },
				createdAt: new Date('2026-10-30T11:59:00.000Z'),
				bounce: { type: 'Transient', subType: 'MailboxFull', message: 'mailbox full' }
			}
		});
	});

	it('accepts a Headers object and one of several signatures', async () => {
		const signed = sign(payload);
		const headers = new Headers({ ...signed, 'webhook-signature': `v1,AAAA ${signed['webhook-signature']}` });
		await expect(verifyWebhook({ payload, headers, secret, now })).resolves.toMatchObject({ type: 'email.bounced' });
	});

	it('rejects a modified body, a wrong secret and missing headers', async () => {
		const headers = sign(payload);
		await expect(verifyWebhook({ payload: payload.replace('Hi', 'Ho'), headers, secret, now })).rejects.toThrow(WebhookVerificationError);
		const other = 'whsec_' + randomBytes(32).toString('base64');
		await expect(verifyWebhook({ payload, headers, secret: other, now })).rejects.toThrow(/does not match/);
		await expect(verifyWebhook({ payload, headers: {}, secret, now })).rejects.toThrow(/Missing/);
	});

	it('rejects timestamps outside the tolerance', async () => {
		const old = sign(payload, { at: new Date(now.getTime() - 301_000) });
		await expect(verifyWebhook({ payload, headers: old, secret, now })).rejects.toThrow(/tolerance/);
		await expect(verifyWebhook({ payload, headers: old, secret, now, toleranceSeconds: 600 })).resolves.toBeTruthy();
	});
});
