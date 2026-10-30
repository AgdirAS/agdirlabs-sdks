## agdir-email completion powershell

Generate the autocompletion script for powershell

### Synopsis

Generate the autocompletion script for powershell.

To load completions in your current shell session:

	agdir-email completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of the above command
to your powershell profile.


```
agdir-email completion powershell [flags]
```

### Options

```
  -h, --help              help for powershell
      --no-descriptions   disable completion descriptions
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

* [agdir-email completion](agdir-email_completion.md)	 - Generate the autocompletion script for the specified shell

