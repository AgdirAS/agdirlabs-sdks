// Opt-in integration test against a running Agdir Email API. Runs only when
// AGDIR_EMAIL_LIVE_BASE_URL, AGDIR_EMAIL_LIVE_API_KEY and AGDIR_EMAIL_LIVE_FROM are set; never in CI.
import { AgdirEmail, AgdirEmailError } from '@agdirlabs/email';
import nodemailer from 'nodemailer';
import { beforeAll, describe, expect, it } from 'vitest';
import { agdirTransport } from './index.js';

const baseUrl = process.env.AGDIR_EMAIL_LIVE_BASE_URL;
const apiKey = process.env.AGDIR_EMAIL_LIVE_API_KEY;
const from = process.env.AGDIR_EMAIL_LIVE_FROM;

describe.skipIf(!baseUrl || !apiKey || !from)('live API', () => {
	let client: AgdirEmail;
	let transporter: ReturnType<typeof createTransporter>;
	const createTransporter = (c: AgdirEmail) => nodemailer.createTransport(agdirTransport({ client: c }));
	beforeAll(() => {
		client = new AgdirEmail({ apiKey: apiKey!, baseUrl: baseUrl!, maxRetries: 5 });
		transporter = createTransporter(client);
	});

	it('sends a scheduled email through Nodemailer, then cancels it with the SDK', async () => {
		const scheduledAt = new Date(Date.now() + 3600_000);
		const info = await transporter.sendMail({
			from: `Nodemailer Test <${from}>`,
			to: ['Ana <ana@example.com>', { name: 'Ben', address: 'ben@example.com' }],
			cc: 'cc@example.com',
			replyTo: 'support@example.com',
			subject: 'Nodemailer transport live test',
			text: Buffer.from('Plain body'),
			html: '<p>HTML body</p>',
			headers: {
				'X-Entity-Ref-ID': 'nodemailer-live',
				'X-Agdir-Tags': 'source=nodemailer, kind=live-test',
				'X-Agdir-Scheduled-At': scheduledAt.toISOString()
			}
		});
		expect(info.messageId).toMatch(/^[0-9a-f-]{36}$/);
		expect(info.accepted).toEqual(['ana@example.com', 'ben@example.com', 'cc@example.com']);

		const email = await client.emails.get(info.messageId);
		expect(email.status).toBe('scheduled');
		expect(email.subject).toBe('Nodemailer transport live test');
		expect(email.to).toHaveLength(2);
		expect(email.cc).toEqual(['cc@example.com']);
		expect(email.replyTo).toEqual(['support@example.com']);
		expect(email.text).toBe('Plain body');
		expect(Math.abs(email.scheduledAt!.getTime() - scheduledAt.getTime())).toBeLessThan(1000);

		await client.emails.cancel(info.messageId);
		expect((await client.emails.get(info.messageId)).status).toBe('canceled');
	});

	it('replays an idempotent send with the same id', async () => {
		const mail = {
			from: from!,
			to: 'ana@example.com',
			subject: 'Idempotent',
			text: 'once',
			scheduledAt: new Date(Date.now() + 3600_000).toISOString(),
			idempotencyKey: `nodemailer-live-${Date.now()}`
		};
		const first = await transporter.sendMail(mail);
		const second = await transporter.sendMail(mail);
		expect(second.messageId).toBe(first.messageId);
		await client.emails.cancel(first.messageId);
	});

	it('reports API errors to Nodemailer as AgdirEmailError', async () => {
		const err = await transporter
			.sendMail({ from: from!, to: 'not-an-address', subject: 'x', text: 'x' })
			.catch((e: unknown) => e);
		expect(err).toBeInstanceOf(AgdirEmailError);
		expect(err).toMatchObject({ code: 'validation_error', status: 422, responseCode: 422 });

		const bad = nodemailer.createTransport(agdirTransport({ apiKey: 're_invalid', baseUrl: baseUrl!, maxRetries: 0 }));
		const authErr = await bad.sendMail({ from: from!, to: 'ana@example.com', subject: 'x', text: 'x' }).catch((e: unknown) => e);
		expect(authErr).toMatchObject({ code: 'invalid_api_key', status: 403 });
	});
});
