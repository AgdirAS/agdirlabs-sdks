## agdir-email webhooks deliveries

Inspect and replay what was sent to a webhook endpoint

```
agdir-email webhooks deliveries [flags]
```

### Options

```
  -h, --help   help for deliveries
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
* [agdir-email webhooks deliveries attempts](agdir-email_webhooks_deliveries_attempts.md)	 - List the HTTP attempts of a delivery
* [agdir-email webhooks deliveries get](agdir-email_webhooks_deliveries_get.md)	 - Show a delivery and the exact payload sent
* [agdir-email webhooks deliveries list](agdir-email_webhooks_deliveries_list.md)	 - List the events sent (or queued) to an endpoint
* [agdir-email webhooks deliveries replay](agdir-email_webhooks_deliveries_replay.md)	 - Send a delivery to the endpoint again

