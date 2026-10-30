## agdir-email domains check

Check the domain's DNS records in live DNS

### Synopsis

Look up each DNS record the domain needs and compare it with the expected value,
so you can see exactly what is missing or wrong before verifying. This queries DNS
from your machine (use --nameserver to ask a specific server and skip local caches).

DKIM and SPF records are required; DMARC is recommended. Exits with code 1 if a required
record is missing or wrong.

```
agdir-email domains check <domain> [flags]
```

### Examples

```
  agdir-email domains check acme.com
  agdir-email domains check acme.com --nameserver 1.1.1.1
```

### Options

```
  -h, --help                help for check
      --nameserver string   DNS server to query, e.g. 1.1.1.1 or 8.8.8.8:53 (default: the system resolver)
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

