## agdir-email

Send email and manage Agdir Email from the command line

### Synopsis

agdir-email is the official command-line interface for Agdir Email (https://agdir.email),
the EU-hosted transactional email API: send email, set up sending domains, check their DNS,
and manage webhooks.

Get an API key at https://app.agdir.email and run "agdir-email login" to get started.

```
agdir-email [flags]
```

### Options

```
      --api-key string    API key (default: $AGDIR_EMAIL_API_KEY, then the profile's key)
      --base-url string   API base URL (default: $AGDIR_EMAIL_BASE_URL, then the profile's, then https://api.agdir.email)
  -h, --help              help for agdir-email
      --json              print machine-readable JSON
      --profile string    configuration profile to use (default: $AGDIR_EMAIL_PROFILE, then the current profile)
  -q, --quiet             print only IDs (or nothing)
```

### SEE ALSO

* [agdir-email batch](agdir-email_batch.md)	 - Send emails in bulk
* [agdir-email completion](agdir-email_completion.md)	 - Generate the autocompletion script for the specified shell
* [agdir-email domains](agdir-email_domains.md)	 - Add, verify and check sending domains
* [agdir-email emails](agdir-email_emails.md)	 - Inspect and cancel sent emails
* [agdir-email login](agdir-email_login.md)	 - Store an API key in a profile
* [agdir-email logout](agdir-email_logout.md)	 - Remove a stored API key
* [agdir-email profiles](agdir-email_profiles.md)	 - List and switch configuration profiles
* [agdir-email send](agdir-email_send.md)	 - Send an email
* [agdir-email version](agdir-email_version.md)	 - Print the version
* [agdir-email webhooks](agdir-email_webhooks.md)	 - Manage webhook endpoints and their deliveries
* [agdir-email whoami](agdir-email_whoami.md)	 - Show the active profile and check its API key

