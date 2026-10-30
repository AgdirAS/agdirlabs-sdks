## agdir-email webhooks delete

Delete a webhook endpoint

```
agdir-email webhooks delete <webhook-id> [flags]
```

### Options

```
  -h, --help   help for delete
  -y, --yes    don't ask for confirmation (required when not interactive)
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

