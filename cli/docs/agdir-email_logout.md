## agdir-email logout

Remove a stored API key

### Synopsis

Remove the current profile (or the one given with --profile) from the configuration file.

```
agdir-email logout [flags]
```

### Options

```
      --all    remove every profile
  -h, --help   help for logout
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

