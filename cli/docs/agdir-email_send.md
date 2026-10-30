## agdir-email send

Send an email

### Synopsis

Send an email (or schedule it with --schedule).

The body comes from --html/--text, from files (--html-file/--text-file, "-" for stdin),
or, when no body flag is given, from stdin as plain text. Give both an HTML and a text
part for the best deliverability. The sender's domain must be verified.

```
agdir-email send [flags]
```

### Examples

```
  agdir-email send --from "Acme <hi@acme.com>" --to ana@example.com \
    --subject "Welcome" --html "<p>Thanks for signing up!</p>"

  # Body from a file, plain-text part from stdin, tags and a custom header
  ./render-report | agdir-email send --from reports@acme.com --to team@acme.com \
    --subject "Weekly report" --html-file report.html --text-file - \
    --tag kind=report --header "X-Entity-Ref-ID: week-44"

  # Schedule for later (RFC 3339 or a relative time)
  agdir-email send --from hi@acme.com --to ana@example.com --subject Reminder \
    --text "See you tomorrow" --schedule "in 2h"
```

### Options

```
      --bcc stringArray          Bcc recipient (repeatable)
      --cc stringArray           Cc recipient (repeatable)
      --from string              sender, "Name <addr@your-domain>" or a bare address (required)
      --header stringArray       custom header as "Name: value" (repeatable)
  -h, --help                     help for send
      --html string              HTML body
      --html-file string         read the HTML body from a file ("-" for stdin)
      --idempotency-key string   make the send safe to repeat: the same key returns the original email instead of sending again
      --reply-to stringArray     Reply-To address (repeatable)
      --schedule string          send later: RFC 3339 time ("2026-11-02T09:00:00+01:00") or relative ("in 30m", "in 2 days")
      --subject string           subject line (required)
      --tag stringArray          tag as name=value, echoed in webhook events (repeatable)
      --text string              plain-text body
      --text-file string         read the plain-text body from a file ("-" for stdin)
      --to stringArray           recipient (repeatable, up to 50) (required)
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

