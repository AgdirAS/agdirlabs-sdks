## agdir-email webhooks create

Register a webhook endpoint

```
agdir-email webhooks create [flags]
```

### Examples

```
  agdir-email webhooks create --endpoint https://acme.com/webhooks/email --event all
  agdir-email webhooks create --endpoint https://acme.com/hook --event delivered,bounced,complained
```

### Options

```
      --endpoint string   public https:// URL that receives events (required)
      --event strings     event to subscribe to, e.g. delivered or email.bounced (repeatable or comma-separated; "all" for every event) (required)
  -h, --help              help for create
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

