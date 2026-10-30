# @agdirlabs/email

[![npm](https://img.shields.io/npm/v/@agdirlabs/email.svg)](https://www.npmjs.com/package/@agdirlabs/email)
[![CI](https://github.com/AgdirAS/agdirlabs-npm/actions/workflows/ci.yml/badge.svg)](https://github.com/AgdirAS/agdirlabs-npm/actions/workflows/ci.yml)

The official Node.js SDK for [Agdir Email](https://agdir.email) — programmatic SMTP for developers, hosted in the EU. Built by [Agdir Labs](https://agdirlabs.com) at [Agdir](https://agdir.no).

- Zero dependencies, fully typed, ESM and CommonJS
- Node.js 20+, Bun, Deno and edge runtimes (uses `fetch` and Web Crypto)
- Safe automatic retries with idempotency keys
- Cursor pagination with `for await`
- Webhook signature verification built in

## Install

```bash
npm install @agdirlabs/email
```

## Quick start

Create an API key at [app.agdir.email](https://app.agdir.email) and verify a sending domain, then:

```ts
import { AgdirEmail } from '@agdirlabs/email';

const agdir = new AgdirEmail({ apiKey: process.env.AGDIR_EMAIL_API_KEY });

const { id } = await agdir.emails.send({
  from: 'Acme <hello@acme.com>',
  to: 'user@example.com',
  subject: 'Welcome to Acme',
  html: '<p>Thanks for signing up!</p>'
});
```

`new AgdirEmail()` with no arguments reads `AGDIR_EMAIL_API_KEY` from the environment.

## Emails

```ts
// Plain text, several recipients, reply-to, tags, custom headers
await agdir.emails.send({
  from: 'Acme <billing@acme.com>',
  to: ['ana@example.com', 'ben@example.com'],
  cc: 'finance@acme.com',
  replyTo: 'support@acme.com',
  subject: 'Your invoice',
  text: 'Invoice #1042 is attached in your dashboard.',
  tags: [{ name: 'kind', value: 'invoice' }],
  headers: { 'X-Entity-Ref-ID': 'inv-1042' }
});

// Schedule for later, then cancel if plans change
const { id } = await agdir.emails.send({ ...email, scheduledAt: new Date(Date.now() + 3600_000) });
await agdir.emails.cancel(id);

// Delivery status
const sent = await agdir.emails.get(id);
sent.status; // 'queued' | 'scheduled' | 'sent' | 'delivered' | 'bounced' | 'complained' | …

// Up to 100 emails per request
const { data } = await agdir.emails.sendBatch([email1, email2]);

// Accept the valid ones, report the rest
const { data: accepted, errors } = await agdir.emails.sendBatch(emails, { validation: 'permissive' });
```

## Domains

```ts
const domain = await agdir.domains.create({ name: 'acme.com' });
for (const r of domain.records) console.log(r.type, r.name, r.value); // add these to your DNS

await agdir.domains.verify(domain.id); // check DNS now instead of waiting
const { status } = await agdir.domains.get(domain.id); // 'pending' → 'verified'

for await (const d of agdir.domains.listAll()) console.log(d.name, d.status);
await agdir.domains.delete(domain.id);
```

## Webhooks

```ts
const { id, signingSecret } = await agdir.webhooks.create({
  endpoint: 'https://acme.com/webhooks/email',
  events: ['email.delivered', 'email.bounced', 'email.complained']
});

await agdir.webhooks.update(id, { status: 'disabled' });
await agdir.webhooks.rotateSecret(id);

// What was sent to your endpoint, and each HTTP attempt
const { data: deliveries } = await agdir.webhooks.deliveries.list(id);
const { data: attempts } = await agdir.webhooks.deliveries.attempts(id, deliveries[0].id);
await agdir.webhooks.deliveries.replay(id, deliveries[0].id);
```

### Verifying incoming events

Every request to your endpoint is signed. Verify it against the **raw** body before trusting it:

```ts
import { verifyWebhook, WebhookVerificationError } from '@agdirlabs/email';

// Express — note express.text(), not express.json(): the signature covers the exact bytes.
app.post('/webhooks/email', express.text({ type: '*/*' }), async (req, res) => {
  try {
    const event = await verifyWebhook({
      payload: req.body,
      headers: req.headers,
      secret: process.env.AGDIR_EMAIL_WEBHOOK_SECRET!
    });
    if (event.type === 'email.bounced') await markBounced(event.data.to, event.data.bounce);
    res.sendStatus(204);
  } catch (err) {
    if (err instanceof WebhookVerificationError) return res.sendStatus(400);
    throw err;
  }
});

// Fetch-style handlers (Next.js route handlers, Hono, Bun, Workers)
export async function POST(request: Request) {
  const event = await verifyWebhook({
    payload: await request.text(),
    headers: request.headers,
    secret: process.env.AGDIR_EMAIL_WEBHOOK_SECRET!
  });
  // …
  return new Response(null, { status: 204 });
}
```

Messages older than 5 minutes are rejected (`toleranceSeconds` to change). Failed deliveries are retried for about 18 hours; use the event's `data.emailId` to deduplicate.

## Errors

Every failed call throws an `AgdirEmailError`:

```ts
import { AgdirEmailError } from '@agdirlabs/email';

try {
  await agdir.emails.send(email);
} catch (err) {
  if (err instanceof AgdirEmailError) {
    err.code; // 'validation_error' | 'daily_quota_exceeded' | 'rate_limit_exceeded' | 'network_error' | …
    err.status; // HTTP status, 0 if no response
    err.message; // human-readable explanation
  }
}
```

## Retries and idempotency

Rate limits (`429 rate_limit_exceeded`), server errors and network failures are retried up to `maxRetries` times (default 2), honouring `retry-after`. Sends are only retried with an idempotency key, so a retry can never deliver an email twice: the SDK generates one per call, or you can pass your own to make sends safe across process restarts too:

```ts
await agdir.emails.send(email, { idempotencyKey: `welcome/${user.id}` });
```

Quota errors are not retried.

## Configuration

```ts
new AgdirEmail({
  apiKey: 're_…', // default: AGDIR_EMAIL_API_KEY
  baseUrl: 'https://api.agdir.email', // default: AGDIR_EMAIL_BASE_URL, else this
  maxRetries: 2,
  timeoutMs: 30_000, // per attempt
  fetch: customFetch // e.g. for a proxy
});
```

Every method also accepts `{ signal }` (an `AbortSignal`) to cancel a request.

## Contributing

Issues and pull requests are welcome — see [CONTRIBUTING.md](../../CONTRIBUTING.md). Changes are listed in [CHANGELOG.md](CHANGELOG.md).

## License

[MIT](LICENSE) © [Agdir AS](https://agdir.no)
