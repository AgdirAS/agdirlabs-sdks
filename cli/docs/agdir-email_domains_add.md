## agdir-email domains add

Add a sending domain and print its DNS records

### Synopsis

Add a sending domain and print the DNS records to publish at your DNS provider.
Record names are relative to your registrable domain (e.g. "send" means send.acme.com).
Then run "agdir-email domains check <name>" to see which records are live.

```
agdir-email domains add <name> [flags]
```

### Examples

```
  agdir-email domains add acme.com
  agdir-email domains add mail.acme.com --json
```

### Options

```
  -h, --help            help for add
      --region string   sending region (default: the API's, eu-north-1)
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

* [agdir-email domains](agdir-email_domains.md)	 - Add, verify and check sending domains

