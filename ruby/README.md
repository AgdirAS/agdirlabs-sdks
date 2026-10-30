# agdirlabs-email

[![Gem](https://img.shields.io/gem/v/agdirlabs-email.svg)](https://rubygems.org/gems/agdirlabs-email)
[![CI](https://github.com/AgdirAS/agdirlabs-sdks/actions/workflows/ruby.yml/badge.svg)](https://github.com/AgdirAS/agdirlabs-sdks/actions/workflows/ruby.yml)

The official Ruby SDK for [Agdir Email](https://agdir.email) — programmatic SMTP for developers, hosted in the EU. Built by [Agdir Labs](https://agdirlabs.com) at [Agdir](https://agdir.no).

- No runtime dependencies (Net::HTTP, JSON, OpenSSL from the standard library)
- Ruby 3.1+, thread-safe client, RBS signatures and YARD docs
- Rails ActionMailer delivery method (`:agdir_email`), also usable with the `mail` gem
- Safe automatic retries with idempotency keys
- Lazy cursor pagination with `Enumerator`
- Webhook signature verification built in

## Install

```ruby
# Gemfile
gem "agdirlabs-email"
```

or `gem install agdirlabs-email`.

## Quick start

Create an API key at [app.agdir.email](https://app.agdir.email) and verify a sending domain, then:

```ruby
require "agdirlabs/email"

client = AgdirLabs::Email::Client.new(api_key: ENV["AGDIR_EMAIL_API_KEY"])

result = client.emails.send(
  from: "Acme <hello@acme.com>",
  to: "user@example.com",
  subject: "Welcome to Acme",
  html: "<p>Thanks for signing up!</p>"
)
result.id # => "4ef9a417-…"
```

`AgdirLabs::Email::Client.new` with no arguments reads `AGDIR_EMAIL_API_KEY` from the environment. Create one client and share it; it is thread-safe.

## Emails

```ruby
# Plain text, several recipients, reply-to, tags, custom headers
client.emails.send(
  from: "Acme <billing@acme.com>",
  to: ["ana@example.com", "ben@example.com"],
  cc: "finance@acme.com",
  reply_to: "support@acme.com",
  subject: "Your invoice",
  text: "Invoice #1042 is ready in your dashboard.",
  tags: { kind: "invoice" },                    # or [{ name: "kind", value: "invoice" }]
  headers: { "X-Entity-Ref-ID" => "inv-1042" }
)

# Schedule for later, then cancel if plans change
sent = client.emails.send(**email, scheduled_at: Time.now + 3600)
client.emails.cancel(sent.id)

# Delivery status
email = client.emails.get(sent.id)
email.status     # => "queued" | "scheduled" | "sent" | "delivered" | "bounced" | "complained" | …
email.created_at # => Time

# Up to 100 emails per request (all-or-nothing by default)
batch = client.emails.send_batch([email1, email2])
batch.data.map(&:id)

# Accept the valid ones, report the rest
batch = client.emails.send_batch(emails, validation: :permissive)
batch.errors.each { |e| warn "email #{e.index}: #{e.message}" }
```

Responses are frozen, keyword-initialized structs (`SendResult`, `EmailDetails`, `Domain`, …) with `Time` timestamps; they support `to_h`, `==` and pattern matching:

```ruby
case client.emails.get(id)
in { status: "bounced" | "complained" } then suppress!(id)
in { status: "delivered" } then nil
end
```

## Domains

```ruby
domain = client.domains.create(name: "acme.com")
domain.records.each { |r| puts "#{r.type} #{r.name} #{r.value}" } # add these to your DNS (r.purpose: DKIM/SPF/DMARC)

client.domains.verify(domain.id)          # check DNS now instead of waiting
client.domains.get(domain.id).status      # "pending" → "verified"

page = client.domains.list(limit: 20)     # one page: page.data, page.has_more?, page.next_cursor
client.domains.list(after: page.next_cursor) if page.has_more?

client.domains.list_all.each { |d| puts "#{d.name} #{d.status}" } # every domain, fetched lazily page by page
client.domains.list_all.lazy.select(&:verified?).first(5)

client.domains.delete(domain.id)
```

## Webhooks

```ruby
hook = client.webhooks.create(
  endpoint: "https://acme.com/webhooks/email",
  events: %w[email.delivered email.bounced email.complained]
)
hook.signing_secret # => "whsec_…" — store it to verify incoming events

client.webhooks.update(hook.id, status: :disabled)
client.webhooks.rotate_secret(hook.id)

# What was sent to your endpoint, and each HTTP attempt
deliveries = client.webhooks.deliveries.list(hook.id)
attempts = client.webhooks.deliveries.attempts(hook.id, deliveries.first.id)
client.webhooks.deliveries.replay(hook.id, deliveries.first.id)
```

### Verifying incoming events

Every request to your endpoint is signed ([Standard Webhooks](https://www.standardwebhooks.com)). Verify it against the **raw** body before trusting it:

```ruby
# Rails
class EmailWebhooksController < ActionController::API
  def create
    event = AgdirLabs::Email::Webhook.verify(
      payload: request.raw_post,
      headers: request.headers,
      secret: ENV.fetch("AGDIR_EMAIL_WEBHOOK_SECRET")
    )
    mark_bounced(event.data.to, event.data.bounce) if event.type == "email.bounced"
    head :no_content
  rescue AgdirLabs::Email::WebhookVerificationError
    head :bad_request
  end
end

# Sinatra / Rack
post "/webhooks/email" do
  event = AgdirLabs::Email::Webhook.verify(payload: request.body.read, headers: request.env, secret: SECRET)
  # …
  204
end
```

`headers:` accepts a Hash (any key case, Rack `HTTP_*` keys work too) or anything with a `[]` reader, like Rails' `request.headers`. Messages older than 5 minutes are rejected (`tolerance:` to change). Failed deliveries are retried for about 18 hours; use `event.data.email_id` to deduplicate.

## Rails (ActionMailer)

Requiring the gem in a Rails app registers the `:agdir_email` delivery method:

```ruby
# config/environments/production.rb
config.action_mailer.delivery_method = :agdir_email
config.action_mailer.agdir_email_settings = {
  api_key: Rails.application.credentials.agdir_email_api_key # default: ENV["AGDIR_EMAIL_API_KEY"]
  # base_url:, max_retries:, timeout: — see Configuration
}
```

Your mailers stay as they are. From, To, Cc, Bcc, Reply-To, Subject, the HTML and text parts, and every `X-…` header are sent. A few headers control the API and are not sent:

| Header | Effect |
| --- | --- |
| `X-Agdir-Tags: kind=welcome, plan=pro` | tags (echoed back in webhook events) |
| `X-Agdir-Scheduled-At: 2026-11-01T09:00:00+01:00` | deliver later |
| `X-Agdir-Idempotency-Key: welcome/42` | idempotency key (one is generated otherwise) |

```ruby
class UserMailer < ApplicationMailer
  def welcome(user)
    headers["X-Agdir-Tags"] = "kind=welcome"
    headers["X-Agdir-Idempotency-Key"] = "welcome/#{user.id}"
    mail(to: user.email, subject: "Welcome!")
  end
end

message = UserMailer.welcome(user).deliver_now
message["X-Agdir-Email-Id"].value # the Agdir Email id, for client.emails.get
```

Attachments are not supported by the API yet: a message with attachments raises `AgdirLabs::Email::UnsupportedMessageError` (link to the file instead). API errors raise `AgdirLabs::Email::Error` subclasses, so `raise_delivery_errors` and `deliver_later` retries work as usual.

Without Rails, use it with the [`mail`](https://github.com/mikel/mail) gem:

```ruby
Mail.defaults { delivery_method AgdirLabs::Email::DeliveryMethod, api_key: ENV["AGDIR_EMAIL_API_KEY"] }
```

## Errors

Every failed call raises an `AgdirLabs::Email::Error`:

```ruby
begin
  client.emails.send(**email)
rescue AgdirLabs::Email::QuotaExceededError => e
  # daily_quota_exceeded / monthly_quota_exceeded — never retried
rescue AgdirLabs::Email::Error => e
  e.code        # "validation_error" | "rate_limit_exceeded" | "network_error" | "timeout" | …
  e.status      # HTTP status, 0 if no response
  e.message     # human-readable explanation
  e.retry_after # seconds from retry-after, if any
end
```

| Class | Codes |
| --- | --- |
| `AuthenticationError` | `missing_api_key`, `invalid_api_key`, `restricted_api_key` |
| `ValidationError` | `validation_error` (422; 403 for an unverified domain, paused team or domain limit) |
| `NotFoundError` | `not_found` |
| `IdempotencyError` | `invalid_idempotent_request`, `concurrent_idempotent_requests` |
| `RateLimitError` | `rate_limit_exceeded` (after retries) |
| `QuotaExceededError` | `daily_quota_exceeded`, `monthly_quota_exceeded` |
| `ServerError` | 5xx |
| `ConnectionError` / `TimeoutError` | `network_error` / `timeout` (no response) |

A missing API key raises `AgdirLabs::Email::ConfigurationError` when the client is created.

## Retries and idempotency

Rate limits (`429 rate_limit_exceeded`), server errors (408/500/502/503/504) and network failures are retried up to `max_retries` times (default 2) with exponential backoff, honouring `retry-after`. Sends are only retried with an idempotency key, so a retry can never deliver an email twice: the SDK generates one per call, or you can pass your own to make sends safe across process restarts too:

```ruby
client.emails.send(**email, idempotency_key: "welcome/#{user.id}")
```

Quota errors are not retried.

## Configuration

```ruby
AgdirLabs::Email::Client.new(
  api_key: "re_…",                       # default: ENV["AGDIR_EMAIL_API_KEY"]
  base_url: "https://api.agdir.email",   # default: ENV["AGDIR_EMAIL_BASE_URL"], else this
  max_retries: 2,
  timeout: 30,                           # seconds, per attempt
  http_client: nil                       # custom transport (see below)
)
```

Requests use Net::HTTP and honour the standard `https_proxy` / `no_proxy` environment variables. To use another HTTP library, pass any object with `call(method:, url:, headers:, body:, timeout:)` returning an `AgdirLabs::Email::HttpResponse` (`status`, `headers`, `body`). To cancel a long-running call, run it in a thread or use `timeout:`.

## Development

```bash
bundle install
bundle exec rake          # tests, RuboCop, RBS validation
bundle exec rake test
```

Live tests run against a real API only when configured (never against production):

```bash
AGDIR_EMAIL_LIVE_BASE_URL=http://localhost:8080/api \
AGDIR_EMAIL_LIVE_API_KEY=re_… \
AGDIR_EMAIL_LIVE_FROM=hi@your-verified-domain.example \
bundle exec ruby -Ilib -Itest test/live_test.rb
```

## Releasing

Releases are published from GitHub Actions with [RubyGems Trusted Publishing](https://guides.rubygems.org/trusted-publishing/) (OIDC): no API key is stored anywhere.

1. Bump `AgdirLabs::Email::VERSION` in `lib/agdirlabs/email/version.rb` and add a `CHANGELOG.md` entry; merge to `main`.
2. Push a tag `ruby/v<version>`, e.g. `git tag ruby/v0.1.0 && git push origin ruby/v0.1.0`.
3. `.github/workflows/ruby-release.yml` checks the tag against `VERSION`, runs the tests and publishes with `rubygems/release-gem` (`rake release` from `ruby/`; the Rakefile disables its own `v*` git tag push).

One-time setup on rubygems.org (by a maintainer of the AgdirAS organisation account):

- **Before the first release** (the gem does not exist yet): rubygems.org → your profile → *Trusted publishers* → *Create pending trusted publisher*: gem name `agdirlabs-email`, repository owner `AgdirAS`, repository name `agdirlabs-sdks`, workflow filename `ruby-release.yml`, environment empty.
- **After the first release**: the pending publisher becomes the gem's trusted publisher (gem page → *Trusted publishers*). Add the other maintainers as gem owners.

## Contributing

Issues and pull requests are welcome — see [CONTRIBUTING.md](../CONTRIBUTING.md). Changes are listed in [CHANGELOG.md](CHANGELOG.md).

## License

[MIT](LICENSE) © [Agdir AS](https://agdir.no)
