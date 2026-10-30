## agdir-email domains

Add, verify and check sending domains

### Synopsis

Manage sending domains. Commands that take a <domain> accept its ID or its name.

Typical setup:
  agdir-email domains add acme.com       # prints the DNS records to publish
  agdir-email domains check acme.com     # checks them in live DNS
  agdir-email domains verify acme.com    # asks Agdir Email to verify now

```
agdir-email domains [flags]
```

### Options

```
  -h, --help   help for domains
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
* [agdir-email domains add](agdir-email_domains_add.md)	 - Add a sending domain and print its DNS records
* [agdir-email domains check](agdir-email_domains_check.md)	 - Check the domain's DNS records in live DNS
* [agdir-email domains delete](agdir-email_domains_delete.md)	 - Delete a domain
* [agdir-email domains get](agdir-email_domains_get.md)	 - Show a domain and the status of its DNS records
* [agdir-email domains list](agdir-email_domains_list.md)	 - List domains
* [agdir-email domains verify](agdir-email_domains_verify.md)	 - Ask Agdir Email to check the domain's DNS records now

