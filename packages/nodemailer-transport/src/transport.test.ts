import { Readable } from 'node:stream';
import { AgdirEmail, AgdirEmailError, type SendEmailInput } from '@agdirlabs/email';
import nodemailer from 'nodemailer';
import { describe, expect, it } from 'vitest';
import { agdirTransport, AgdirTransport, VERSION } from './index.js';
import { formatAddresses, parseTagsHeader } from './transport.js';

type Call = { input: SendEmailInput; options?: { idempotencyKey?: string } };

function fakeClient(result: () => Promise<{ id: string }> = async () => ({ id: 'email_123' })) {
	const calls: Call[] = [];
	const client = {
		emails: {
			async send(input: SendEmailInput, options?: { idempotencyKey?: string }) {
				calls.push({ input, options });
				return result();
			}
		}
	};
	return { client, calls };
}

function setup(result?: () => Promise<{ id: string }>) {
	const { client, calls } = fakeClient(result);
	const transporter = nodemailer.createTransport(agdirTransport({ client }));
	return { transporter, calls };
}

const base = { from: 'Acme <hi@acme.com>', to: 'ana@example.com', subject: 'Hello', text: 'Hi' };

describe('agdirTransport', () => {
	it('is a Nodemailer transport', () => {
		const transport = agdirTransport({ client: fakeClient().client });
		expect(transport).toBeInstanceOf(AgdirTransport);
		expect(transport.name).toBe('AgdirEmail');
		expect(transport.version).toBe(VERSION);
	});

	it('builds an AgdirEmail client from options', () => {
		const transport = agdirTransport({ apiKey: 're_test', baseUrl: 'http://localhost:1/api', maxRetries: 0 });
		expect(transport.client).toBeInstanceOf(AgdirEmail);
		expect((transport.client as AgdirEmail).baseUrl).toBe('http://localhost:1/api');
	});

	it('requires an API key when no client is given', () => {
		const saved = process.env.AGDIR_EMAIL_API_KEY;
		delete process.env.AGDIR_EMAIL_API_KEY;
		try {
			expect(() => agdirTransport()).toThrow(/AGDIR_EMAIL_API_KEY/);
		} finally {
			if (saved !== undefined) process.env.AGDIR_EMAIL_API_KEY = saved;
		}
	});
});

describe('sending', () => {
	it('sends a simple message and reports the Agdir email id', async () => {
		const { transporter, calls } = setup();
		const info = await transporter.sendMail({ ...base, html: '<p>Hi</p>' });

		expect(calls).toHaveLength(1);
		expect(calls[0]!.input).toEqual({
			from: 'Acme <hi@acme.com>',
			to: ['ana@example.com'],
			subject: 'Hello',
			text: 'Hi',
			html: '<p>Hi</p>'
		});
		expect(calls[0]!.options).toEqual({});
		expect(info.messageId).toBe('email_123');
		expect(info.envelope).toEqual({ from: 'hi@acme.com', to: ['ana@example.com'] });
		expect(info.accepted).toEqual(['ana@example.com']);
		expect(info.rejected).toEqual([]);
		expect(info.response).toBe('Accepted email_123');
	});

	it('normalises address fields given as strings, objects and arrays', async () => {
		const { transporter, calls } = setup();
		const info = await transporter.sendMail({
			from: { name: 'Acme, Inc.', address: 'hi@acme.com' },
			to: ['ana@example.com', { name: 'Ben Bo', address: 'ben@example.com' }, 'Cy <cy@example.com>, dee@example.com'],
			cc: 'Team: tom@example.com, tim@example.com;',
			bcc: [{ name: '', address: 'audit@acme.com' }],
			replyTo: 'Support "Desk" <support@acme.com>',
			subject: 'Hi',
			text: 'x'
		});
		const input = calls[0]!.input;
		expect(input.from).toBe('"Acme, Inc." <hi@acme.com>');
		expect(input.to).toEqual(['ana@example.com', 'Ben Bo <ben@example.com>', 'Cy <cy@example.com>', 'dee@example.com']);
		expect(input.cc).toEqual(['tom@example.com', 'tim@example.com']);
		expect(input.bcc).toEqual(['audit@acme.com']);
		expect(input.replyTo).toHaveLength(1);
		expect(input.replyTo![0]).toMatch(/<support@acme\.com>$/);
		expect(info.envelope.to).toEqual([
			'ana@example.com',
			'ben@example.com',
			'cy@example.com',
			'dee@example.com',
			'tom@example.com',
			'tim@example.com',
			'audit@acme.com'
		]);
	});

	it('resolves html and text given as Buffers and streams', async () => {
		const { transporter, calls } = setup();
		await transporter.sendMail({
			...base,
			text: Buffer.from('plain body'),
			html: Readable.from([Buffer.from('<p>streamed '), Buffer.from('body</p>')])
		});
		expect(calls[0]!.input.text).toBe('plain body');
		expect(calls[0]!.input.html).toBe('<p>streamed body</p>');
	});

	it('forwards custom headers and threading headers, dropping the X-Agdir-* control headers', async () => {
		const { transporter, calls } = setup();
		await transporter.sendMail({
			...base,
			inReplyTo: '<abc@acme.com>',
			references: ['<abc@acme.com>', '<def@acme.com>'],
			priority: 'high',
			headers: {
				'X-Entity-Ref-ID': 'inv-1042',
				'X-Multi': ['a', 'b'],
				'X-Prepared': { prepared: true, value: 'raw' },
				'X-Agdir-Tags': 'kind=invoice',
				'X-Agdir-Scheduled-At': '2030-01-01T00:00:00Z',
				'X-Agdir-Idempotency-Key': 'inv-1042'
			}
		});
		const { input, options } = calls[0]!;
		expect(input.headers).toEqual({
			'X-Entity-Ref-ID': 'inv-1042',
			'X-Multi': 'a, b',
			'X-Prepared': 'raw',
			'In-Reply-To': '<abc@acme.com>',
			References: '<abc@acme.com> <def@acme.com>',
			'X-Priority': '1 (Highest)',
			'X-MSMail-Priority': 'High',
			Importance: 'High'
		});
		expect(input.tags).toEqual([{ name: 'kind', value: 'invoice' }]);
		expect(input.scheduledAt).toBe('2030-01-01T00:00:00Z');
		expect(options).toEqual({ idempotencyKey: 'inv-1042' });
	});

	it('accepts headers in array form', async () => {
		const { transporter, calls } = setup();
		await transporter.sendMail({
			...base,
			headers: [
				{ key: 'X-One', value: '1' },
				{ key: 'x-agdir-tags', value: 'a=1; b=2' }
			]
		});
		expect(calls[0]!.input.headers).toEqual({ 'X-One': '1' });
		expect(calls[0]!.input.tags).toEqual([
			{ name: 'a', value: '1' },
			{ name: 'b', value: '2' }
		]);
	});

	it('reads tags, scheduledAt and idempotencyKey from the message options', async () => {
		const { transporter, calls } = setup();
		const when = new Date('2030-01-01T10:00:00Z');
		const mail = {
			...base,
			tags: { category: 'welcome', plan: 'pro' },
			scheduledAt: when,
			idempotencyKey: 'welcome/42',
			headers: { 'X-Agdir-Tags': 'source=signup', 'X-Agdir-Idempotency-Key': 'ignored' }
		};
		await transporter.sendMail(mail);
		const { input, options } = calls[0]!;
		expect(input.tags).toEqual([
			{ name: 'category', value: 'welcome' },
			{ name: 'plan', value: 'pro' },
			{ name: 'source', value: 'signup' }
		]);
		expect(input.scheduledAt).toBe(when);
		expect(input.headers).toBeUndefined();
		expect(options).toEqual({ idempotencyKey: 'welcome/42' });
	});

	it('accepts tags as an array of { name, value }', async () => {
		const { transporter, calls } = setup();
		const mail = { ...base, tags: [{ name: 'kind', value: 'receipt' }] };
		await transporter.sendMail(mail);
		expect(calls[0]!.input.tags).toEqual([{ name: 'kind', value: 'receipt' }]);
	});

	it('rejects a malformed X-Agdir-Tags header before calling the API', async () => {
		const { transporter, calls } = setup();
		const err = await transporter.sendMail({ ...base, headers: { 'X-Agdir-Tags': 'novalue' } }).catch((e: unknown) => e);
		expect(err).toBeInstanceOf(AgdirEmailError);
		expect(err).toMatchObject({ code: 'validation_error', status: 0 });
		expect(calls).toHaveLength(0);
	});

	it('works with the callback API', async () => {
		const { transporter } = setup();
		const info = await new Promise((resolve, reject) =>
			transporter.sendMail(base, (err, info) => (err ? reject(err) : resolve(info)))
		);
		expect(info).toMatchObject({ messageId: 'email_123' });
	});
});

describe('unsupported features', () => {
	it.each([
		['attachments', { attachments: [{ filename: 'a.txt', content: 'hello' }] }, /Attachments are not supported/],
		['raw', { raw: 'From: a@b.c\r\n\r\nhi' }, /raw/],
		['alternatives', { alternatives: [{ contentType: 'text/x-web-markdown', content: '**hi**' }] }, /alternatives/],
		['icalEvent', { icalEvent: { content: 'BEGIN:VCALENDAR' } }, /icalEvent/],
		['envelope', { envelope: { from: 'x@acme.com', to: ['y@example.com'] } }, /envelope/]
	])('fails clearly for %s', async (_name, extra, message) => {
		const { transporter, calls } = setup();
		const err = await transporter.sendMail({ ...base, ...extra }).catch((e: unknown) => e);
		expect(err).toBeInstanceOf(AgdirEmailError);
		expect(err).toMatchObject({ code: 'unsupported', status: 0 });
		expect((err as Error).message).toMatch(message);
		expect(calls).toHaveLength(0);
	});

	it('allows an empty attachments array', async () => {
		const { transporter, calls } = setup();
		await transporter.sendMail({ ...base, attachments: [] });
		expect(calls).toHaveLength(1);
	});
});

describe('errors', () => {
	it('passes AgdirEmailError through with code, status and responseCode', async () => {
		const { transporter } = setup(async () => {
			throw new AgdirEmailError('Daily quota exceeded', 'daily_quota_exceeded', 429);
		});
		const err = await transporter.sendMail(base).catch((e: unknown) => e);
		expect(err).toBeInstanceOf(AgdirEmailError);
		expect(err).toMatchObject({
			code: 'daily_quota_exceeded',
			status: 429,
			responseCode: 429,
			message: 'Daily quota exceeded'
		});
	});

	it('passes network errors through', async () => {
		const { transporter } = setup(async () => {
			throw new AgdirEmailError('fetch failed', 'network_error', 0);
		});
		const err = await transporter.sendMail(base).catch((e: unknown) => e);
		expect(err).toMatchObject({ code: 'network_error', status: 0 });
		expect(err).not.toHaveProperty('responseCode');
	});

	it('reports failures to the callback API', async () => {
		const { transporter } = setup(async () => {
			throw new AgdirEmailError('Invalid `to` field', 'validation_error', 422);
		});
		const err = await new Promise((resolve) => transporter.sendMail(base, (e) => resolve(e)));
		expect(err).toMatchObject({ code: 'validation_error', status: 422 });
	});

	it('sends through the real SDK client end to end (stubbed fetch)', async () => {
		const requests: { url: string; headers: Headers; body: unknown }[] = [];
		const fetch = (async (url: string, init: RequestInit) => {
			requests.push({ url: String(url), headers: new Headers(init.headers), body: JSON.parse(String(init.body)) });
			return new Response(JSON.stringify({ id: 'e_1' }), { status: 200, headers: { 'content-type': 'application/json' } });
		}) as typeof globalThis.fetch;
		const transporter = nodemailer.createTransport(agdirTransport({ apiKey: 're_test', baseUrl: 'https://api.test', fetch }));
		const info = await transporter.sendMail({ ...base, replyTo: 'support@acme.com', headers: { 'X-Agdir-Idempotency-Key': 'k1' } });
		expect(info.messageId).toBe('e_1');
		expect(requests).toHaveLength(1);
		expect(requests[0]!.url).toBe('https://api.test/emails');
		expect(requests[0]!.headers.get('authorization')).toBe('Bearer re_test');
		expect(requests[0]!.headers.get('idempotency-key')).toBe('k1');
		expect(requests[0]!.body).toEqual({
			from: 'Acme <hi@acme.com>',
			to: ['ana@example.com'],
			subject: 'Hello',
			text: 'Hi',
			reply_to: ['support@acme.com']
		});
	});
});

describe('helpers', () => {
	it('formatAddresses handles groups, blanks and quoting', () => {
		expect(
			formatAddresses([
				{ name: 'G', group: [{ name: 'A', address: 'a@x.io' }, { address: 'b@x.io' }] },
				null,
				{ name: 'Dr. Who', address: 'who@x.io' },
				{ name: 'Say "hi"', address: 'q@x.io' }
			])
		).toEqual(['A <a@x.io>', 'b@x.io', '"Dr. Who" <who@x.io>', '"Say \\"hi\\"" <q@x.io>']);
	});

	it('parseTagsHeader splits on , and ;', () => {
		expect(parseTagsHeader(' a=1 ,b=2; c = 3 ;')).toEqual([
			{ name: 'a', value: '1' },
			{ name: 'b', value: '2' },
			{ name: 'c', value: '3' }
		]);
		expect(() => parseTagsHeader('=x')).toThrow(/expected name=value/);
	});
});
