package app

import (
	"errors"
	"fmt"

	agdiremail "github.com/AgdirAS/agdirlabs-sdks/go"
	"github.com/spf13/cobra"
)

func (a *App) newLoginCmd() *cobra.Command {
	var noVerify bool
	c := &cobra.Command{
		Use:   "login",
		Short: "Store an API key in a profile",
		Long: `Store an API key in a profile of the configuration file
(<config dir>/agdir-email/config.json, readable only by you).

The key is read from --api-key, from stdin when it is piped, or from a hidden prompt.
It is checked with a cheap API request before it is saved. Use --profile to keep keys
for several teams or environments; the profile you log in to becomes the current one.`,
		Example: `  agdir-email login
  agdir-email login --profile staging --base-url https://api.staging.example
  echo "$AGDIR_EMAIL_API_KEY" | agdir-email login --profile ci`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, path, err := a.loadConfig()
			if err != nil {
				return err
			}
			name := a.profileName(cfg)
			if err := validProfileName(name); err != nil {
				return err
			}

			key := a.opts.apiKey
			if key == "" {
				if a.stdinIsTerminal() {
					fmt.Fprint(a.Stderr, "Paste your API key (create one at https://app.agdir.email): ")
					key, err = a.ReadSecret()
					fmt.Fprintln(a.Stderr)
				} else {
					key, err = readLine(a.Stdin)
				}
				if err != nil {
					return fmt.Errorf("reading the API key: %w", err)
				}
			}
			if key == "" {
				return usageErrorf("no API key given")
			}

			baseURL := a.opts.baseURL
			if baseURL == "" {
				baseURL = a.getenv(agdiremail.EnvBaseURL)
			}
			effectiveBase := baseURL
			if effectiveBase == "" {
				effectiveBase = agdiremail.DefaultBaseURL
			}

			acc := access("")
			if !noVerify {
				client, err := a.newClient(key, effectiveBase)
				if err != nil {
					return err
				}
				acc, err = checkAccess(cmd.Context(), client)
				var apiErr *agdiremail.Error
				if errors.As(err, &apiErr) && authCodes[apiErr.Code] {
					return authErrorf("the API key was rejected (%s): %s", apiErr.Code, apiErr.Message)
				}
				if err != nil {
					return err
				}
			}

			cfg.Profiles[name] = &Profile{APIKey: key, BaseURL: baseURL}
			cfg.CurrentProfile = name
			if err := saveConfig(path, cfg); err != nil {
				return err
			}

			if a.opts.json {
				return a.printJSON(map[string]any{
					"profile": name, "base_url": effectiveBase, "access": acc, "config_path": path,
				})
			}
			if acc == "" {
				a.printf("Saved the API key to profile %q (not verified).\n", name)
			} else {
				a.printf("Logged in to %s with profile %q (%s).\n", effectiveBase, name, acc.describe())
			}
			a.printf("Config: %s\n", path)
			return nil
		},
	}
	c.Flags().BoolVar(&noVerify, "no-verify", false, "save the key without checking it against the API")
	return c
}

func (a *App) newLogoutCmd() *cobra.Command {
	var all bool
	c := &cobra.Command{
		Use:   "logout",
		Short: "Remove a stored API key",
		Long:  "Remove the current profile (or the one given with --profile) from the configuration file.",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, path, err := a.loadConfig()
			if err != nil {
				return err
			}
			var removed []string
			if all {
				removed = cfg.names()
				cfg.Profiles = map[string]*Profile{}
				cfg.CurrentProfile = ""
			} else {
				name := a.profileName(cfg)
				if _, ok := cfg.Profiles[name]; ok {
					delete(cfg.Profiles, name)
					removed = append(removed, name)
				}
				if cfg.CurrentProfile == name {
					cfg.CurrentProfile = ""
				}
			}
			if len(removed) > 0 {
				if err := saveConfig(path, cfg); err != nil {
					return err
				}
			}
			if a.opts.json {
				return a.printJSON(map[string]any{"removed": nonNil(removed)})
			}
			if len(removed) == 0 {
				a.printf("Profile %q is not logged in.\n", a.profileName(nil))
				return nil
			}
			for _, n := range removed {
				a.printf("Logged out of profile %q.\n", n)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&all, "all", false, "remove every profile")
	return c
}

func (a *App) newWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "whoami",
		Aliases: []string{"status"},
		Short:   "Show the active profile and check its API key",
		Long: `Show which API key and base URL commands use, where they come from, and whether
the API accepts the key. Exits with code 3 when no key is configured or it is rejected.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := a.resolveCredentials()
			if err != nil {
				return err
			}
			if c.apiKey == "" {
				return authErrorf("not logged in: run \"agdir-email login\", set %s or pass --api-key", agdiremail.EnvAPIKey)
			}
			client, err := a.newClient(c.apiKey, c.baseURL)
			if err != nil {
				return err
			}
			acc, err := checkAccess(cmd.Context(), client)
			if err != nil {
				return err
			}
			profile := c.profile
			if c.keySource != sourceProfile {
				profile = ""
			}
			if a.opts.json {
				return a.printJSON(map[string]any{
					"profile": profile, "api_key": maskKey(c.apiKey), "key_source": c.keySource,
					"base_url": c.baseURL, "access": acc, "config_path": c.cfgPath,
				})
			}
			if a.opts.quiet {
				return nil
			}
			source := map[string]string{
				sourceFlag: "--api-key flag", sourceEnv: "$" + agdiremail.EnvAPIKey, sourceProfile: fmt.Sprintf("profile %q", c.profile),
			}[c.keySource]
			fields(a.Stdout, [][2]string{
				{"API key", fmt.Sprintf("%s (from %s)", maskKey(c.apiKey), source)},
				{"Base URL", c.baseURL},
				{"Access", acc.describe()},
				{"Config", c.cfgPath},
			})
			return nil
		},
	}
}

func (a *App) newProfilesCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "profiles",
		Short: "List and switch configuration profiles",
	}
	c.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List stored profiles",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, _, err := a.loadConfig()
			if err != nil {
				return err
			}
			current := a.profileName(cfg)
			type row struct {
				Name    string `json:"name"`
				Current bool   `json:"current"`
				APIKey  string `json:"api_key"`
				BaseURL string `json:"base_url"`
			}
			rows := []row{}
			for _, n := range cfg.names() {
				p := cfg.Profiles[n]
				base := p.BaseURL
				if base == "" {
					base = agdiremail.DefaultBaseURL
				}
				rows = append(rows, row{n, n == current, maskKey(p.APIKey), base})
			}
			if a.opts.json {
				return a.printJSON(map[string]any{"data": rows})
			}
			if a.opts.quiet {
				for _, r := range rows {
					a.printIDs(r.Name)
				}
				return nil
			}
			if len(rows) == 0 {
				a.printf("No profiles. Run \"agdir-email login\" to add one.\n")
				return nil
			}
			var t [][]string
			for _, r := range rows {
				mark := ""
				if r.Current {
					mark = "*"
				}
				t = append(t, []string{mark, r.Name, r.APIKey, r.BaseURL})
			}
			table(a.Stdout, []string{"", "PROFILE", "API KEY", "BASE URL"}, t)
			return nil
		},
	}, &cobra.Command{
		Use:   "use <profile>",
		Short: "Make a profile the current one",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, path, err := a.loadConfig()
			if err != nil {
				return err
			}
			if _, ok := cfg.Profiles[args[0]]; !ok {
				return failf("no profile %q; run \"agdir-email login --profile %s\"", args[0], args[0])
			}
			cfg.CurrentProfile = args[0]
			if err := saveConfig(path, cfg); err != nil {
				return err
			}
			if a.opts.json {
				return a.printJSON(map[string]any{"current_profile": args[0]})
			}
			a.printf("Now using profile %q.\n", args[0])
			return nil
		},
	})
	return c
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
