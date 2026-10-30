# agdirlabs/email

[![Packagist](https://img.shields.io/packagist/v/agdirlabs/email.svg)](https://packagist.org/packages/agdirlabs/email)
[![CI](https://github.com/AgdirAS/agdirlabs-sdks/actions/workflows/php.yml/badge.svg)](https://github.com/AgdirAS/agdirlabs-sdks/actions/workflows/php.yml)

The official PHP SDK for [Agdir Email](https://agdir.email) — programmatic SMTP for developers, hosted in the EU. Built by [Agdir Labs](https://agdirlabs.com) at [Agdir](https://agdir.no).

- PHP 8.1+, strict types, readonly result objects and enums
- Works with any PSR-18 HTTP client (Guzzle, Symfony HttpClient, …)
- Safe automatic retries with idempotency keys
- Lazy cursor pagination with generators
- Webhook signature verification built in
- **Laravel** mail transport, facade and config — `MAIL_MAILER=agdir`
- **Symfony Mailer** transport — `agdir+api://KEY@default`

## Install

```bash
composer require agdirlabs/email
```

The SDK talks HTTP through [PSR-18](https://www.php-fig.org/psr/psr-18/). If your project has no HTTP client yet, add one:

```bash
composer require guzzlehttp/guzzle      # or: composer require symfony/http-client
```

## Quick start

Create an API key at [app.agdir.email](https://app.agdir.email) and verify a sending domain, then:

```php
use AgdirLabs\Email\AgdirEmail;

$agdir = new AgdirEmail('re_…');

$result = $agdir->emails->send([
    'from' => 'Acme <hello@acme.com>',
    'to' => 'user@example.com',
    'subject' => 'Welcome to Acme',
    'html' => '<p>Thanks for signing up!</p>',
]);

echo $result->id;
```

`new AgdirEmail()` with no arguments reads `AGDIR_EMAIL_API_KEY` from the environment.

## Emails

An email is an array; results are typed objects.

```php
// Plain text, several recipients, reply-to, tags, custom headers
$agdir->emails->send([
    'from' => 'Acme <billing@acme.com>',
    'to' => ['ana@example.com', 'ben@example.com'],
    'cc' => 'finance@acme.com',
    'replyTo' => 'support@acme.com',
    'subject' => 'Your invoice',
    'text' => 'Invoice #1042 is ready in your dashboard.',
    'tags' => ['kind' => 'invoice'],            // or [['name' => 'kind', 'value' => 'invoice']]
    'headers' => ['X-Entity-Ref-ID' => 'inv-1042'],
]);

// Schedule for later, then cancel if plans change
$sent = $agdir->emails->send($email + ['scheduledAt' => new DateTimeImmutable('+1 hour')]);
$agdir->emails->cancel($sent->id);

// Delivery status
$email = $agdir->emails->get($sent->id);
$email->status;      // EmailStatus::Scheduled, ::Sent, ::Delivered, ::Bounced, ::Complained, …
$email->createdAt;   // DateTimeImmutable
$email->cc;          // list<string> (empty, never null)

// Up to 100 emails per request
$batch = $agdir->emails->sendBatch([$email1, $email2]);
foreach ($batch->data as $accepted) echo $accepted->id;

// Accept the valid ones, report the rest
$batch = $agdir->emails->sendBatch($emails, validation: 'permissive');
foreach ($batch->errors as $error) echo "#{$error->index}: {$error->message}";
```

Fields: `from`, `to`, `subject`, `html` and/or `text`, `cc`, `bcc`, `replyTo`, `headers`, `tags` (ASCII letters, digits, `_`, `-`), `scheduledAt` (a `DateTimeInterface` or an ISO 8601 string with offset). `reply_to` and `scheduled_at` work as aliases; unknown keys throw an `InvalidArgumentException`.

## Domains

```php
$domain = $agdir->domains->create('acme.com');
foreach ($domain->records as $r) {
    echo "{$r->purpose} {$r->type} {$r->name} {$r->value}\n";  // add these to your DNS
}

$agdir->domains->verify($domain->id);                // check DNS now instead of waiting
$agdir->domains->get($domain->id)->status;           // DomainStatus::Pending → DomainStatus::Verified

foreach ($agdir->domains->listAll() as $d) {         // fetches further pages as you iterate
    echo $d->name, ' ', $d->status->value, "\n";
}

$page = $agdir->domains->list(limit: 20);            // or one page at a time
$next = $agdir->domains->list(limit: 20, after: $page->nextCursor);

$agdir->domains->delete($domain->id);
```

## Webhooks

```php
use AgdirLabs\Email\Model\WebhookEventType;
use AgdirLabs\Email\Model\WebhookStatus;

$webhook = $agdir->webhooks->create('https://acme.com/webhooks/email', [
    WebhookEventType::Delivered,
    WebhookEventType::Bounced,
    'email.complained',                               // strings work too
]);
$webhook->signingSecret;                              // whsec_… — store it

$agdir->webhooks->update($webhook->id, status: WebhookStatus::Disabled);
$agdir->webhooks->rotateSecret($webhook->id);

// What was sent to your endpoint, and each HTTP attempt
$deliveries = $agdir->webhooks->deliveries->list($webhook->id);
$first = $deliveries->data[0];
$attempts = $agdir->webhooks->deliveries->attempts($webhook->id, $first->id);
$agdir->webhooks->deliveries->replay($webhook->id, $first->id);
```

### Verifying incoming events

Every request to your endpoint is signed. Verify it against the **raw** body before trusting it:

```php
use AgdirLabs\Email\Exception\WebhookVerificationException;
use AgdirLabs\Email\Webhook;

// Plain PHP
try {
    $event = Webhook::verify(
        payload: file_get_contents('php://input'),
        headers: getallheaders(),                  // or $_SERVER
        secret: getenv('AGDIR_EMAIL_WEBHOOK_SECRET'),
    );
} catch (WebhookVerificationException) {
    http_response_code(400);
    exit;
}

if ($event->type === WebhookEventType::Bounced) {
    markBounced($event->data->to, $event->data->bounce?->type);
}
http_response_code(204);
```

```php
// Laravel (exclude the route from CSRF protection) — Symfony works the same with $request->getContent()
Route::post('/webhooks/email', function (Illuminate\Http\Request $request) {
    try {
        $event = Webhook::verify($request->getContent(), $request->headers, config('services.agdir.webhook_secret'));
    } catch (WebhookVerificationException) {
        abort(400);
    }
    // $event->data->emailId, $event->data->tags, …
    return response()->noContent();
});
```

`headers` accepts an array (any case, `$_SERVER`-style `HTTP_WEBHOOK_ID` keys, list values), a Symfony/Laravel `HeaderBag` or a PSR-7 request. Messages older than 5 minutes are rejected (`tolerance:` to change). Failed deliveries are retried for about 18 hours; use `$event->data->emailId` to deduplicate.

## Laravel

The service provider and the `AgdirEmail` facade are auto-discovered. Add your key to `.env`:

```dotenv
AGDIR_EMAIL_API_KEY=re_…
```

### Sending Laravel mail through Agdir Email

```dotenv
MAIL_MAILER=agdir
```

That's all: the package adds an `agdir` mailer unless your `config/mail.php` defines one. Define it yourself to give it a different API key:

```php
'mailers' => [
    'agdir' => [
        'transport' => 'agdir',
        'key' => env('OTHER_AGDIR_KEY'),   // optional; defaults to AGDIR_EMAIL_API_KEY
    ],
],
```

Mailables, notifications and `Mail::raw()` now go through the API. `Mail::send()` returns the Agdir Email id as `$sent->getMessageId()`.

| Laravel / Symfony `Email` | API field |
|---|---|
| from, to, cc, bcc, reply-to, subject | same |
| HTML view / text view | `html` / `text` |
| `Envelope(tags: ['welcome'])`, `->tag('welcome')` | tag `welcome` = `true` |
| `Envelope(metadata: ['user_id' => '42'])`, `->metadata(…)` | tag `user_id` = `42` |
| header `X-Agdir-Tag-<name>: <value>` | tag `<name>` = `<value>` |
| any other custom header (`X-Entity-Ref-ID`, `List-Unsubscribe`, …) | `headers` |
| attachments, embedded images | not supported yet — throws `UnsupportedAttachmentException` |

Tag names and values may only contain ASCII letters, digits, `_` and `-`.

### Using the client directly

```php
use AgdirLabs\Email\AgdirEmail;
use AgdirLabs\Email\Laravel\Facades\AgdirEmail as Agdir;

Agdir::emails()->send([...]);                   // facade
app(AgdirEmail::class)->domains->listAll();     // or inject AgdirEmail into your class
```

Publish the config to change retries or timeouts:

```bash
php artisan vendor:publish --tag=agdir-email-config
```

| `config/agdir-email.php` | env | default |
|---|---|---|
| `api_key` | `AGDIR_EMAIL_API_KEY` | — |
| `base_url` | `AGDIR_EMAIL_BASE_URL` | `https://api.agdir.email` |
| `max_retries` | `AGDIR_EMAIL_MAX_RETRIES` | 2 |
| `timeout` | `AGDIR_EMAIL_TIMEOUT` | 30 (seconds) |

## Symfony Mailer

The transport works with Symfony Mailer anywhere — Symfony apps, Laravel, or standalone (`composer require symfony/mailer`).

```dotenv
MAILER_DSN=agdir+api://re_…@default
```

Register the factory in a Symfony app (`config/services.yaml`):

```yaml
services:
    AgdirLabs\Email\Mailer\AgdirTransportFactory:
        parent: mailer.transport_factory.abstract
        tags: ['mailer.transport_factory']
```

Standalone:

```php
use AgdirLabs\Email\Mailer\AgdirTransportFactory;
use Symfony\Component\Mailer\Mailer;
use Symfony\Component\Mailer\Transport;
use Symfony\Component\Mime\Email;

$transport = (new Transport([new AgdirTransportFactory()]))->fromString('agdir+api://re_…@default');
$sent = (new Mailer($transport))->send((new Email())->from('hello@acme.com')->to('user@example.com')
    ->subject('Hi')->html('<p>Hi</p>'));

// or wrap an existing client
$transport = new AgdirLabs\Email\Mailer\AgdirTransport(new AgdirEmail('re_…'));
```

DSN options: `agdir+api://KEY@default?max_retries=2&timeout=30`; `default` means `api.agdir.email`. The mapping is the table above.

## Errors

Every failed call throws an `AgdirEmailException`:

```php
use AgdirLabs\Email\Exception\AgdirEmailException;

try {
    $agdir->emails->send($email);
} catch (AgdirEmailException $e) {
    $e->errorCode;   // 'validation_error' | 'daily_quota_exceeded' | 'rate_limit_exceeded' | 'network_error' | 'timeout' | …
    $e->status;      // HTTP status, 0 if no response (also $e->getCode())
    $e->getMessage(); // human-readable explanation
    $e->retryAfter;  // seconds, when the API sent retry-after
}
```

The codes are available as constants, e.g. `AgdirEmailException::MONTHLY_QUOTA_EXCEEDED`. Invalid arguments (unknown email fields, a missing API key) throw `InvalidArgumentException`.

## Retries and idempotency

Rate limits (`429 rate_limit_exceeded`), server errors and network failures are retried up to `maxRetries` times (default 2), honouring `retry-after`. Sends are only retried with an idempotency key, so a retry can never deliver an email twice: the SDK generates one per call, or you can pass your own to make sends safe across process restarts too:

```php
$agdir->emails->send($email, idempotencyKey: "welcome/{$user->id}");
```

Quota errors are not retried.

## Configuration

```php
new AgdirEmail(
    apiKey: 're_…',                          // default: AGDIR_EMAIL_API_KEY
    baseUrl: 'https://api.agdir.email',      // default: AGDIR_EMAIL_BASE_URL, else this
    maxRetries: 2,
    timeout: 30.0,                           // seconds per attempt
    httpClient: $psr18Client,                // optional: any PSR-18 client (proxies, tests)
    requestFactory: $psr17Factory,           // optional
    streamFactory: $psr17Factory,            // optional
);
```

Without `httpClient` the SDK uses Symfony HttpClient or Guzzle (whichever is installed) with `timeout` applied, else any PSR-18 client found by `php-http/discovery`. A client you pass in keeps its own timeout settings. PHP has no request cancellation primitive; use the client's timeout.

## Development

```bash
composer install
composer test        # PHPUnit unit tests (HTTP is mocked)
composer analyse     # PHPStan level 8
```

The live suite runs against a real API and is skipped unless configured — never point it at production:

```bash
AGDIR_EMAIL_LIVE_BASE_URL=https://your-test-api.example \
AGDIR_EMAIL_LIVE_API_KEY=re_… AGDIR_EMAIL_LIVE_FROM=hi@your-verified-domain.com \
composer test:live
```

No PHP locally? `docker run --rm -v "$PWD":/app -w /app composer:2 sh -c 'composer install && composer test'`.

## Releasing

Packagist needs `composer.json` at the root of a git repository, so `php/` is mirrored to the read-only repository **`AgdirAS/agdirlabs-email-php`** by [`.github/workflows/php-split.yml`](../.github/workflows/php-split.yml), and Packagist reads that mirror. Composer versions come from git tags; there is no version field in `composer.json`.

To release:

1. Update `AgdirEmail::VERSION` and `CHANGELOG.md` (the unit tests check they match), merge to `main`.
2. Tag the monorepo: `git tag php/v0.1.0 && git push origin php/v0.1.0`.
3. The workflow splits `php/` into its own history, pushes it to `main` of the mirror and tags it `v0.1.0`. Packagist picks the tag up through its GitHub hook.

One-time setup (repository owner):

1. Create the empty public repository `AgdirAS/agdirlabs-email-php` (description: "Read-only mirror of php/ in AgdirAS/agdirlabs-sdks"; disable issues, wiki and PRs in favour of the monorepo).
2. Create a deploy key: `ssh-keygen -t ed25519 -C php-split -f php-split -N ''`. Add `php-split.pub` to the mirror under *Settings → Deploy keys* **with write access**, and the private key `php-split` to `AgdirAS/agdirlabs-sdks` under *Settings → Secrets and variables → Actions* as `PHP_SPLIT_DEPLOY_KEY`. Delete both local files.
3. Push the first tag (above) so the mirror has content.
4. Sign in to [packagist.org](https://packagist.org) with GitHub as an AgdirAS maintainer, *Submit* `https://github.com/AgdirAS/agdirlabs-email-php`, and claim the `agdirlabs` vendor name. Packagist's GitHub integration then updates the package on every push/tag (check *Settings → Webhooks* on the mirror, or enable it from the Packagist package page).

## Contributing

Issues and pull requests are welcome in [AgdirAS/agdirlabs-sdks](https://github.com/AgdirAS/agdirlabs-sdks) — see [CONTRIBUTING.md](../CONTRIBUTING.md). Changes are listed in [CHANGELOG.md](CHANGELOG.md).

## License

[MIT](LICENSE) © [Agdir AS](https://agdir.no)
