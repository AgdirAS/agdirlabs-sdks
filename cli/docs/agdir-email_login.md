## agdir-email login

Store an API key in a profile

### Synopsis

Store an API key in a profile of the configuration file
(<config dir>/agdir-email/config.json, readable only by you).

The key is read from --api-key, from stdin when it is piped, or from a hidden prompt.
It is checked with a cheap API request before it is saved. Use --profile to keep keys
for several teams or environments; the profile you log in to becomes the current one.

```
agdir-email login [flags]
```

### Examples

```
  agdir-email login
  agdir-email login --profile staging --base-url https://api.staging.example
  echo "$AGDIR_EMAIL_API_KEY" | agdir-email login --profile ci
```

### Options

```
  -h, --help        help for login
      --no-verify   save the key without checking it against the API
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

