## agdir-email completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(agdir-email completion zsh)

To load completions for every new session, execute once:

#### Linux:

	agdir-email completion zsh > "${fpath[1]}/_agdir-email"

#### macOS:

	agdir-email completion zsh > $(brew --prefix)/share/zsh/site-functions/_agdir-email

You will need to start a new shell for this setup to take effect.


```
agdir-email completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
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

