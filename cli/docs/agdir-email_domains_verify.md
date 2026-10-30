## agdir-email domains verify

Ask Agdir Email to check the domain's DNS records now

### Synopsis

Start verification of a domain now instead of waiting for the periodic check.
Verification runs in the background: follow it with "agdir-email domains get <domain>".
To see what is published in DNS right now, use "agdir-email domains check <domain>".

```
agdir-email domains verify <domain> [flags]
```

### Options

```
  -h, --help   help for verify
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

