## agdir-email completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(agdir-email completion bash)

To load completions for every new session, execute once:

#### Linux:

	agdir-email completion bash > /etc/bash_completion.d/agdir-email

#### macOS:

	agdir-email completion bash > $(brew --prefix)/etc/bash_completion.d/agdir-email

You will need to start a new shell for this setup to take effect.


```
agdir-email completion bash
```

### Options

```
  -h, --help              help for bash
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

