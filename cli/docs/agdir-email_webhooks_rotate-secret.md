## agdir-email webhooks rotate-secret

Replace a webhook's signing secret

### Synopsis

Generate a new signing secret. Update your endpoint's secret right away: new events are signed with the new one.

```
agdir-email webhooks rotate-secret <webhook-id> [flags]
```

### Options

```
  -h, --help   help for rotate-secret
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

