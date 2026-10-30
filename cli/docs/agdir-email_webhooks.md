## agdir-email webhooks

Manage webhook endpoints and their deliveries

### Synopsis

Manage webhook endpoints that receive email events, inspect what was delivered to them,
and verify signatures of captured requests.

Event types: email.sent, email.scheduled, email.delivered, email.delivery_delayed, email.complained, email.bounced, email.opened, email.clicked, email.failed.

```
agdir-email webhooks [flags]
```

### Options

```
  -h, --help   help for webhooks
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

* [agdir-email](agdir-email.md)	 - Send email and manage Agdir Email from the command line
* [agdir-email webhooks create](agdir-email_webhooks_create.md)	 - Register a webhook endpoint
* [agdir-email webhooks delete](agdir-email_webhooks_delete.md)	 - Delete a webhook endpoint
* [agdir-email webhooks deliveries](agdir-email_webhooks_deliveries.md)	 - Inspect and replay what was sent to a webhook endpoint
* [agdir-email webhooks get](agdir-email_webhooks_get.md)	 - Show a webhook endpoint, including its signing secret
* [agdir-email webhooks list](agdir-email_webhooks_list.md)	 - List webhook endpoints
* [agdir-email webhooks rotate-secret](agdir-email_webhooks_rotate-secret.md)	 - Replace a webhook's signing secret
* [agdir-email webhooks update](agdir-email_webhooks_update.md)	 - Change a webhook's endpoint, events or status
* [agdir-email webhooks verify](agdir-email_webhooks_verify.md)	 - Check the signature of a captured webhook request

