# Agdir Email API — OpenAPI description

`agdir-email.yaml` is the [OpenAPI 3.1](https://spec.openapis.org/oas/v3.1.0) description of the
Agdir Email REST API (`https://api.agdir.email`): emails, domains, webhooks and webhook
deliveries, the error format, rate-limit and idempotency headers, API-key permissions per
operation, and the signed webhook payload (`webhooks.emailEvent`).

Use it to generate clients, mock servers or API docs, or to check an SDK against the wire format.

## Source of truth

This file is a **copy**. The original lives in the Agdir Email service repository
(`packages/shared/openapi/agdir-email.yaml`), where a compat test calls every operation against
the real server and validates each response against the spec. The live version is always
served by the API itself:

- https://api.agdir.email/openapi.json
- https://api.agdir.email/openapi.yaml

Don't edit the copy here by hand — change the service, then refresh the copy.

## Refresh / compare

```bash
# Replace the copy with the published version
curl -fsS https://api.agdir.email/openapi.yaml -o openapi/agdir-email.yaml

# Or just check whether it is out of date
curl -fsS https://api.agdir.email/openapi.yaml | diff -u openapi/agdir-email.yaml - && echo "up to date"
```

## Lint

```bash
npx @redocly/cli@2.72.1 lint openapi/agdir-email.yaml --config openapi/redocly.yaml
```

CI (`.github/workflows/openapi.yml`) runs the same lint on every change under `openapi/`.
