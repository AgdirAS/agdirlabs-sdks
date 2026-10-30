## agdir-email whoami

Show the active profile and check its API key

### Synopsis

Show which API key and base URL commands use, where they come from, and whether
the API accepts the key. Exits with code 3 when no key is configured or it is rejected.

```
agdir-email whoami [flags]
```

### Options

```
  -h, --help   help for whoami
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

