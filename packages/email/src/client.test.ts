import { afterEach, describe, expect, it, vi } from 'vitest';
import { AgdirEmail, AgdirEmailError, DEFAULT_BASE_URL, VERSION } from './index.js';
import { fakeFetch } from './testing.js';

afterEach(() => vi.unstubAllEnvs());

describe('AgdirEmail configuration', () => {
	it('uses api.agdir.email by default', () => {
		vi.stubEnv('AGDIR_EMAIL_BASE_URL', '');
		expect(new AgdirEmail('re_test').baseUrl).toBe(DEFAULT_BASE_URL);
	});

	it('reads the API key and base URL from the environment; empty values count as unset', () => {
		vi.stubEnv('AGDIR_EMAIL_API_KEY', 're_env');
		vi.stubEnv('AGDIR_EMAIL_BASE_URL', 'http://localhost:8080');
		expect(new AgdirEmail().baseUrl).toBe('http://localhost:8080');
		vi.stubEnv('AGDIR_EMAIL_API_KEY', '');
		expect(() => new AgdirEmail()).toThrow(/Missing API key/);
	});

	it('rejects an invalid maxRetries', () => {
		expect(() => new AgdirEmail({ apiKey: 're_test', maxRetries: -1 })).toThrow(/maxRetries/);
	});
});

describe('requests', () => {
	it('sends a camelCase email as the API JSON with auth, user agent and an idempotency key', async () => {
		const { fetch, requests } = fakeFetch({ body: { id: 'em_1' } });
		const agdir = new AgdirEmail({ apiKey: 're_test', baseUrl: 'https://api.test/', fetch });

		const result = await agdir.emails.send({
			from: 'Acme <hello@acme.com>',
			to: 'user@example.com',
			subject: 'Hi',
			text: 'Hello',
			replyTo: 'support@acme.com',
			scheduledAt: new Date('2026-11-01T09:00:00Z'),
			tags: [{ name: 'kind', value: 'welcome' }]
		});

		expect(result).toEqual({ id: 'em_1' });
		const [req] = requests;
		expect(req!.method).toBe('POST');
		expect(req!.url.href).toBe('https://api.test/emails');
		expect(req!.headers.get('authorization')).toBe('Bearer re_test');
		expect(req!.headers.get('user-agent')).toBe(`agdirlabs-email-node/${VERSION}`);
		expect(req!.headers.get('idempotency-key')).toMatch(/^[0-9a-f-]{36}$/);
		expect(req!.body).toEqual({
			from: 'Acme <hello@acme.com>',
			to: 'user@example.com',
			subject: 'Hi',
			text: 'Hello',
			reply_to: 'support@acme.com',
			scheduled_at: '2026-11-01T09:00:00.000Z',
			tags: [{ name: 'kind', value: 'welcome' }]
		});
	});

	it('uses a caller idempotency key, and none when retries are off', async () => {
		const { fetch, requests } = fakeFetch({ body: { id: 'a' } }, { body: { id: 'b' } });
		await new AgdirEmail({ apiKey: 're_test', fetch }).emails.send(
			{ from: 'a@acme.com', to: 'b@example.com', subject: 's', text: 't' },
			{ idempotencyKey: 'order-42' }
		);
		await new AgdirEmail({ apiKey: 're_test', fetch, maxRetries: 0 }).emails.send({
			from: 'a@acme.com',
			to: 'b@example.com',
			subject: 's',
			text: 't'
		});
		expect(requests[0]!.headers.get('idempotency-key')).toBe('order-42');
		expect(requests[1]!.headers.has('idempotency-key')).toBe(false);
	});

	it('sends permissive batches and returns per-index errors', async () => {
		const { fetch, requests } = fakeFetch({ body: { data: [{ id: 'em_1' }], errors: [{ index: 1, message: 'bad to' }] } });
		const agdir = new AgdirEmail({ apiKey: 're_test', fetch });
		const email = { from: 'a@acme.com', to: 'b@example.com', subject: 's', text: 't' };

		const result = await agdir.emails.sendBatch([email, { ...email, to: 'nope' }], { validation: 'permissive' });

		expect(result).toEqual({ data: [{ id: 'em_1' }], errors: [{ index: 1, message: 'bad to' }] });
		expect(requests[0]!.url.pathname).toBe('/emails/batch');
		expect(requests[0]!.headers.get('x-batch-validation')).toBe('permissive');
		expect(requests[0]!.body).toHaveLength(2);
	});

	it('maps an email to camelCase with dates and empty lists', async () => {
		const { fetch } = fakeFetch({
			body: {
				object: 'email',
				id: 'em_1',
				from: 'a@acme.com',
				to: ['b@example.com'],
				cc: null,
				bcc: null,
				reply_to: null,
				subject: 's',
				html: null,
				text: 't',
				created_at: '2026-10-30T10:00:00.000Z',
				scheduled_at: null,
				last_event: 'delivered'
			}
		});
		const email = await new AgdirEmail({ apiKey: 're_test', fetch }).emails.get('em_1');
		expect(email).toEqual({
			id: 'em_1',
			from: 'a@acme.com',
			to: ['b@example.com'],
			cc: [],
			bcc: [],
			replyTo: [],
			subject: 's',
			html: null,
			text: 't',
			createdAt: new Date('2026-10-30T10:00:00.000Z'),
			scheduledAt: null,
			status: 'delivered'
		});
	});

	it('maps domains with DNS records', async () => {
		const { fetch, requests } = fakeFetch({
			body: {
				object: 'domain',
				id: 'd_1',
				name: 'acme.com',
				status: 'pending',
				region: 'eu-north-1',
				created_at: '2026-10-30T10:00:00.000Z',
				open_tracking: false,
				click_tracking: false,
				records: [
					{ record: 'SPF', name: 'send', type: 'MX', ttl: 'Auto', status: 'pending', value: 'feedback-smtp.eu-north-1.amazonses.com', priority: 10 }
				]
			}
		});
		const domain = await new AgdirEmail({ apiKey: 're_test', fetch }).domains.create({ name: 'acme.com' });
		expect(requests[0]!.body).toEqual({ name: 'acme.com' });
		expect(domain.records).toEqual([
			{ purpose: 'SPF', name: 'send', type: 'MX', ttl: 'Auto', status: 'pending', value: 'feedback-smtp.eu-north-1.amazonses.com', priority: 10 }
		]);
		expect(domain.createdAt).toEqual(new Date('2026-10-30T10:00:00.000Z'));
	});

	it('maps webhook create and rotate to signingSecret', async () => {
		const { fetch, requests } = fakeFetch(
			{ body: { object: 'webhook', id: 'wh_1', signing_secret: 'whsec_a' } },
			{ body: { object: 'webhook', id: 'wh_1', signing_secret: 'whsec_b' } }
		);
		const agdir = new AgdirEmail({ apiKey: 're_test', fetch });
		expect(await agdir.webhooks.create({ endpoint: 'https://acme.com/hook', events: ['email.delivered'] })).toEqual({
			id: 'wh_1',
			signingSecret: 'whsec_a'
		});
		expect(await agdir.webhooks.rotateSecret('wh_1')).toEqual({ id: 'wh_1', signingSecret: 'whsec_b' });
		expect(requests[1]!.url.pathname).toBe('/webhooks/wh_1/signing-secret/rotate');
	});

	it('escapes ids in paths', async () => {
		const { fetch, requests } = fakeFetch({ body: { id: 'x' } });
		await new AgdirEmail({ apiKey: 're_test', fetch }).emails.cancel('a/b?c');
		expect(requests[0]!.url.pathname).toBe('/emails/a%2Fb%3Fc/cancel');
	});
});

describe('pagination', () => {
	it('returns pages with a cursor and walks all of them with listAll', async () => {
		const hook = (id: string) => ({ id, endpoint: 'https://acme.com/h', status: 'enabled', events: ['email.sent'], created_at: '2026-10-30T10:00:00.000Z' });
		const { fetch, requests } = fakeFetch(
			{ body: { object: 'list', has_more: true, data: [hook('w1'), hook('w2')] } },
			{ body: { object: 'list', has_more: false, data: [hook('w3')] } }
		);
		const agdir = new AgdirEmail({ apiKey: 're_test', fetch });

		const ids: string[] = [];
		for await (const w of agdir.webhooks.listAll({ limit: 2 })) ids.push(w.id);

		expect(ids).toEqual(['w1', 'w2', 'w3']);
		expect(requests.map((r) => r.url.search)).toEqual(['?limit=2', '?limit=2&after=w2']);
	});
});

describe('errors and retries', () => {
	it('throws AgdirEmailError with the API code, status and message', async () => {
		const { fetch } = fakeFetch({ status: 422, body: { statusCode: 422, name: 'validation_error', message: 'to: Invalid email address' } });
		const err = await new AgdirEmail({ apiKey: 're_test', fetch })
			.emails.send({ from: 'a@acme.com', to: 'nope', subject: 's', text: 't' })
			.catch((e: unknown) => e);
		expect(err).toBeInstanceOf(AgdirEmailError);
		expect(err).toMatchObject({ code: 'validation_error', status: 422, message: 'to: Invalid email address' });
	});

	it('retries rate limits after retry-after, reusing the idempotency key', async () => {
		const { fetch, requests } = fakeFetch(
			{ status: 429, body: { name: 'rate_limit_exceeded', message: 'slow down' }, headers: { 'retry-after': '0' } },
			{ body: { id: 'em_1' } }
		);
		const result = await new AgdirEmail({ apiKey: 're_test', fetch }).emails.send({ from: 'a@acme.com', to: 'b@example.com', subject: 's', text: 't' });
		expect(result).toEqual({ id: 'em_1' });
		expect(requests).toHaveLength(2);
		expect(requests[1]!.headers.get('idempotency-key')).toBe(requests[0]!.headers.get('idempotency-key'));
	});

	it('does not retry quota errors', async () => {
		const { fetch, requests } = fakeFetch({ status: 429, body: { name: 'daily_quota_exceeded', message: 'quota' } });
		await expect(
			new AgdirEmail({ apiKey: 're_test', fetch }).emails.send({ from: 'a@acme.com', to: 'b@example.com', subject: 's', text: 't' })
		).rejects.toMatchObject({ code: 'daily_quota_exceeded' });
		expect(requests).toHaveLength(1);
	});

	it('retries GET server errors and network failures, then gives up after maxRetries', async () => {
		const unavailable = { status: 503, body: {}, headers: { 'retry-after': '0' } };
		const { fetch, requests } = fakeFetch(unavailable, unavailable, unavailable);
		const err = await new AgdirEmail({ apiKey: 're_test', fetch }).domains.get('d_1').catch((e: unknown) => e);
		expect(requests).toHaveLength(3);
		expect(err).toMatchObject({ status: 503, code: 'internal_server_error' });
	});

	it('does not retry a non-idempotent POST after a network failure', async () => {
		const { fetch, requests } = fakeFetch(new TypeError('fetch failed'));
		const err = await new AgdirEmail({ apiKey: 're_test', fetch }).webhooks
			.create({ endpoint: 'https://acme.com/h', events: ['email.sent'] })
			.catch((e: unknown) => e);
		expect(requests).toHaveLength(1);
		expect(err).toMatchObject({ code: 'network_error', status: 0 });
	});
});
