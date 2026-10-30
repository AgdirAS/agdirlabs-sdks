## agdir-email batch send

Send up to 100 emails in one request

### Synopsis

Send up to 100 emails from a JSON file ("-" for stdin): an array of email objects in the
API's format (from, to, subject, html, text, cc, bcc, reply_to, headers, tags, scheduled_at).

By default one invalid email rejects the whole batch. With --permissive the valid emails are
sent and the rejected ones are reported; the command then exits with code 1 if any were rejected.

```
agdir-email batch send <file.json> [flags]
```

### Examples

```
  agdir-email batch send emails.json
  jq '[.users[] | {from: "hi@acme.com", to: .email, subject: "Hi", text: "Hello!"}]' users.json \
    | agdir-email batch send - --permissive --json
```

### Options

```
  -h, --help                     help for send
      --idempotency-key string   make the batch safe to repeat
      --permissive               send the valid emails and report the invalid ones instead of rejecting the batch
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

* [agdir-email batch](agdir-email_batch.md)	 - Send emails in bulk

