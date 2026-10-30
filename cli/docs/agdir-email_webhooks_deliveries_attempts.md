## agdir-email webhooks deliveries attempts

List the HTTP attempts of a delivery

```
agdir-email webhooks deliveries attempts <webhook-id> <delivery-id> [flags]
```

### Options

```
      --after string   cursor: list items after this ID (the previous page's next_cursor)
  -h, --help           help for attempts
      --limit int      fetch one page of at most this many items (1-100); default: all items
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

* [agdir-email webhooks deliveries](agdir-email_webhooks_deliveries.md)	 - Inspect and replay what was sent to a webhook endpoint

