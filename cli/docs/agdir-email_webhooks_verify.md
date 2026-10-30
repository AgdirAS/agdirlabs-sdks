## agdir-email webhooks verify

Check the signature of a captured webhook request

### Synopsis

Verify a webhook request you captured (e.g. from your server's logs) the same way your
endpoint should: the raw body in <payload-file> ("-" for stdin) and the webhook-id,
webhook-timestamp and webhook-signature headers.

Headers come from --headers-file (lines of "Name: value", as copied from a request dump,
or a JSON object) and/or --header. The secret comes from --secret or
$AGDIR_EMAIL_WEBHOOK_SECRET. Exits with code 1 if the signature is invalid.

```
agdir-email webhooks verify <payload-file> [flags]
```

### Examples

```
  agdir-email webhooks verify --secret whsec_… --headers-file headers.txt body.json
  agdir-email webhooks verify --header "webhook-id: msg_1" --header "webhook-timestamp: 1761800000" \
    --header "webhook-signature: v1,K5o…" --ignore-timestamp body.json
```

### Options

```
      --header stringArray    a request header as "Name: value" (repeatable)
      --headers-file string   file with the request headers ("Name: value" lines or a JSON object; "-" for stdin)
  -h, --help                  help for verify
      --ignore-timestamp      skip the timestamp age check (for old captures)
      --secret string         the endpoint's signing secret, whsec_… (default: $AGDIR_EMAIL_WEBHOOK_SECRET)
      --tolerance duration    maximum age of the webhook-timestamp (default 5m0s)
```

### Options inherited from parent commands

```
      --api-key string    API key (default: $AGDIR_EMAIL_API_KEY, then the profile's key)
      --base-url string   API base URL (default: $AGDIR_EMAIL_BASE_URL, then the profile's, then https://api.agdir.email)
      --json              print machine-readable JSON
      --profile string    configuration profile to use (default: $AGDIR_EMAIL_PROFILE, then the current profile)
  -q, --quiet             print only IDs (or nothing)
```

### SEE ALSO

* [agdir-email webhooks](agdir-email_webhooks.md)	 - Manage webhook endpoints and their deliveries

