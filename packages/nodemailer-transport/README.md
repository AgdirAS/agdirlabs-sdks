# @agdirlabs/nodemailer-transport

[![npm](https://img.shields.io/npm/v/@agdirlabs/nodemailer-transport.svg)](https://www.npmjs.com/package/@agdirlabs/nodemailer-transport)
[![CI](https://github.com/AgdirAS/agdirlabs-sdks/actions/workflows/node.yml/badge.svg)](https://github.com/AgdirAS/agdirlabs-sdks/actions/workflows/node.yml)

A [Nodemailer](https://nodemailer.com) transport for [Agdir Email](https://agdir.email) — programmatic SMTP for developers, hosted in the EU. Keep your existing `transporter.sendMail()` code and send over the Agdir Email API instead of SMTP. Built by [Agdir Labs](https://agdirlabs.com) at [Agdir](https://agdir.no) on top of the official SDK, [`@agdirlabs/email`](https://www.npmjs.com/package/@agdirlabs/email).

- Drop-in: any Nodemailer address form, Buffers and streams for `html`/`text`, custom headers
- Tags, scheduled sending and idempotency keys
- Safe automatic retries (rate limits, server and network errors) from the SDK
- Fully typed, ESM and CommonJS, Nodemailer 6 and newer, Node.js 20+

## Install

```bash
npm install @agdirlabs/nodemailer-transport nodemailer
```

## Quick start

Create an API key at [app.agdir.email](https://app.agdir.email) and verify a sending domain, then:

```ts
import nodemailer from 'nodemailer';
import { agdirTransport } from '@agdirlabs/nodemailer-transport';

const transporter = nodemailer.createTransport(agdirTransport({ apiKey: process.env.AGDIR_EMAIL_API_KEY }));

const info = await transporter.sendMail({
  from: 'Acme <hello@acme.com>',
  to: 'user@example.com',
  subject: 'Welcome to Acme',
  html: '<p>Thanks for signing up!</p>'
});

info.messageId; // the Agdir Email id, e.g. for agdir.emails.get(info.messageId)
```

`agdirTransport()` with no arguments reads `AGDIR_EMAIL_API_KEY` (and `AGDIR_EMAIL_BASE_URL`) from the environment.

CommonJS works the same way:

```js
const nodemailer = require('nodemailer');
const { agdirTransport } = require('@agdirlabs/nodemailer-transport');
```

## Options

```ts
agdirTransport({
  apiKey: 're_…', // default: AGDIR_EMAIL_API_KEY
  baseUrl: 'https://api.agdir.email', // default: AGDIR_EMAIL_BASE_URL, else this
  maxRetries: 2, // retries for rate limits, server errors and network failures; 0 disables
  timeoutMs: 30_000, // per attempt
  fetch: customFetch // e.g. for a proxy
});

// Or reuse a client you already have
import { AgdirEmail } from '@agdirlabs/email';
const agdir = new AgdirEmail();
const transporter = nodemailer.createTransport(agdirTransport({ client: agdir }));
```

## Messages

All standard address fields work — `from`, `to`, `cc`, `bcc`, `replyTo` — as strings (`'Ana <ana@example.com>, ben@example.com'`), `{ name, address }` objects, or arrays of either. `html` and `text` can be strings, Buffers, streams, or `{ path }` / `{ href }` (resolved by Nodemailer, subject to its `disableFileAccess` / `disableUrlAccess` settings). At least one of `html` and `text` is required.

Custom `headers` (object or `[{ key, value }]` form) are sent with the email; `inReplyTo`, `references`, `list` and `priority` become their usual headers.

### Tags, scheduling and idempotency

Agdir-specific options can be given as message fields or, if you'd rather not touch your message objects (or their TypeScript types), as `X-Agdir-*` headers. The transport reads these headers and does **not** send them.

| | Message field | Header |
|---|---|---|
| Tags (echoed back in webhook events) | `tags: [{ name, value }]` or `tags: { name: value }` | `X-Agdir-Tags: category=welcome, plan=pro` |
| Deliver later | `scheduledAt: Date \| ISO 8601 string` | `X-Agdir-Scheduled-At: 2026-11-01T09:00:00Z` |
| Idempotency key | `idempotencyKey: 'welcome/42'` | `X-Agdir-Idempotency-Key: welcome/42` |

Tag names and values may contain ASCII letters, digits, `_` and `-`. Tags from both places are combined; for the other two the message field wins.

```ts
// Headers: works with any typing
await transporter.sendMail({
  from, to, subject, html,
  headers: {
    'X-Agdir-Tags': 'category=welcome',
    'X-Agdir-Scheduled-At': new Date(Date.now() + 3600_000).toISOString(),
    'X-Agdir-Idempotency-Key': `welcome/${user.id}`
  }
});

// Message fields: build the message as a variable (Nodemailer's types don't declare these fields,
// so an inline object literal would fail TypeScript's excess-property check)
import type { AgdirMailOptions } from '@agdirlabs/nodemailer-transport';
import type { SendMailOptions } from 'nodemailer';

const message: SendMailOptions & AgdirMailOptions = {
  from, to, subject, html,
  tags: { category: 'welcome' },
  scheduledAt: new Date(Date.now() + 3600_000),
  idempotencyKey: `welcome/${user.id}`
};
await transporter.sendMail(message);
```

Scheduled emails can be cancelled with the SDK: `await agdir.emails.cancel(info.messageId)`.

### Result

`sendMail` resolves (or calls back) with:

```ts
{
  messageId: '4ef9a3c2-…', // Agdir Email id
  envelope: { from: 'hello@acme.com', to: ['user@example.com'] },
  accepted: ['user@example.com'], // every envelope recipient: the API accepts the email as a whole
  rejected: [],
  pending: [],
  response: 'Accepted 4ef9a3c2-…'
}
```

Delivery happens asynchronously; follow it with `agdir.emails.get(id)` or [webhooks](https://www.npmjs.com/package/@agdirlabs/email#webhooks).

### Not supported (yet)

The Agdir Email API does not take attachments yet, so a message with `attachments` (or `raw`, `alternatives`, `icalEvent`) fails with an `AgdirEmailError` whose `code` is `unsupported` — link to files from the body instead. A custom SMTP `envelope` is rejected the same way, because the email always goes to its `to`, `cc` and `bcc` recipients. `amp` and `watchHtml` are ignored, as is `verify()` (there is no connection to check).

## Errors

Failures are reported as an `AgdirEmailError` (re-exported from this package):

```ts
import { AgdirEmailError } from '@agdirlabs/nodemailer-transport';

try {
  await transporter.sendMail(message);
} catch (err) {
  if (err instanceof AgdirEmailError) {
    err.code; // 'validation_error' | 'daily_quota_exceeded' | 'rate_limit_exceeded' | 'network_error' | 'unsupported' | …
    err.status; // HTTP status, 0 if no response (or the message was rejected before sending)
  }
}
```

Like Nodemailer's SMTP transports, the error also carries `responseCode` (the HTTP status) when the API answered.

## Retries and idempotency

Rate limits (`429 rate_limit_exceeded`), server errors and network failures are retried by the SDK up to `maxRetries` times, honouring `retry-after`. Each send gets a generated idempotency key, so a retry can never deliver an email twice; pass your own (`idempotencyKey` / `X-Agdir-Idempotency-Key`) to make sends safe across process restarts too. Quota errors are not retried.

## Releasing

Push a tag `nodemailer-transport@<version>` matching `package.json` — `.github/workflows/release.yml` publishes it to npm with Trusted Publishing. One-time setup: on npmjs.com, add `AgdirAS/agdirlabs-sdks` with workflow `release.yml` as a trusted publisher for `@agdirlabs/nodemailer-transport` (for a brand-new package, the first version may have to be published by hand before the trusted publisher can be configured).

## Contributing

Issues and pull requests are welcome — see [CONTRIBUTING.md](../../CONTRIBUTING.md). Changes are listed in [CHANGELOG.md](CHANGELOG.md). The opt-in live test runs against a real API:

```bash
AGDIR_EMAIL_LIVE_BASE_URL=… AGDIR_EMAIL_LIVE_API_KEY=re_… AGDIR_EMAIL_LIVE_FROM=hi@your-domain.com pnpm test
```

## License

[MIT](LICENSE) © [Agdir AS](https://agdir.no)
