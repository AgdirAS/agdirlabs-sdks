## agdir-email webhooks update

Change a webhook's endpoint, events or status

```
agdir-email webhooks update <webhook-id> [flags]
```

### Examples

```
  agdir-email webhooks update wh_123 --status disabled
  agdir-email webhooks update wh_123 --event all
```

### Options

```
      --endpoint string   new https:// endpoint URL
      --event strings     replace the subscribed events (repeatable or comma-separated; "all")
  -h, --help              help for update
      --status string     enabled or disabled
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

