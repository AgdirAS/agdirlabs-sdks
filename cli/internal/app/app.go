// Package app implements the agdir-email command-line interface.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	agdiremail "github.com/AgdirAS/agdirlabs-sdks/go"
	"github.com/spf13/cobra"
)

// Exit codes.
const (
	ExitOK    = 0 // success
	ExitError = 1 // API error, failed check or other runtime failure
	ExitUsage = 2 // invalid command, flags or arguments
	ExitAuth  = 3 // no API key, or the key was rejected
)

// Environment variables read by the CLI (besides AGDIR_EMAIL_API_KEY and AGDIR_EMAIL_BASE_URL).
const (
	EnvProfile       = "AGDIR_EMAIL_PROFILE"
	EnvConfig        = "AGDIR_EMAIL_CONFIG"
	EnvWebhookSecret = "AGDIR_EMAIL_WEBHOOK_SECRET"
)

// App holds the CLI's I/O and injectable dependencies. The zero value is not usable; use [New].
type App struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	// Getenv reads environment variables (os.Getenv by default).
	Getenv func(string) string
	// HTTPClient is used for API requests; its transport gets the CLI's User-Agent.
	HTTPClient *http.Client
	// Resolver is used by `domains check` (net.DefaultResolver by default).
	Resolver Resolver
	// Now returns the current time (time.Now by default).
	Now func() time.Time
	// StdinIsTerminal reports whether Stdin is an interactive terminal.
	StdinIsTerminal func() bool
	// ReadSecret reads a line from the terminal without echoing it.
	ReadSecret func() (string, error)

	opts globalOptions
}

type globalOptions struct {
	apiKey  string
	baseURL string
	profile string
	json    bool
	quiet   bool
}

// New returns an App wired to the process's stdio and environment.
func New() *App {
	return &App{
		Stdin:           os.Stdin,
		Stdout:          os.Stdout,
		Stderr:          os.Stderr,
		Getenv:          os.Getenv,
		Now:             time.Now,
		StdinIsTerminal: func() bool { return isTerminal(os.Stdin) },
		ReadSecret:      readSecretFromTerminal,
	}
}

// Run executes the CLI with args (without the program name) and returns the exit code.
func (a *App) Run(args []string) int { return a.RunContext(context.Background(), args) }

// RunContext is Run with a context that cancels in-flight requests (e.g. on Ctrl-C).
func (a *App) RunContext(ctx context.Context, args []string) int {
	a.opts = globalOptions{}
	root := a.newRootCmd()
	root.SetArgs(args)
	cmd, err := root.ExecuteContextC(ctx)
	if err == nil {
		return ExitOK
	}
	return a.reportError(cmd, err)
}

func (a *App) newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "agdir-email",
		Short: "Send email and manage Agdir Email from the command line",
		Long: `agdir-email is the official command-line interface for Agdir Email (https://agdir.email),
the EU-hosted transactional email API: send email, set up sending domains, check their DNS,
and manage webhooks.

Get an API key at https://app.agdir.email and run "agdir-email login" to get started.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version,
	}
	root.SetIn(a.Stdin)
	root.SetOut(a.Stdout)
	root.SetErr(a.Stderr)
	root.SetVersionTemplate("agdir-email {{.Version}}\n")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return usageError(err) })

	pf := root.PersistentFlags()
	pf.StringVar(&a.opts.apiKey, "api-key", "", "API key (default: $AGDIR_EMAIL_API_KEY, then the profile's key)")
	pf.StringVar(&a.opts.baseURL, "base-url", "", "API base URL (default: $AGDIR_EMAIL_BASE_URL, then the profile's, then "+agdiremail.DefaultBaseURL+")")
	pf.StringVar(&a.opts.profile, "profile", "", "configuration profile to use (default: $AGDIR_EMAIL_PROFILE, then the current profile)")
	pf.BoolVar(&a.opts.json, "json", false, "print machine-readable JSON")
	pf.BoolVarP(&a.opts.quiet, "quiet", "q", false, "print only IDs (or nothing)")

	root.AddGroup(
		&cobra.Group{ID: "email", Title: "Email:"},
		&cobra.Group{ID: "setup", Title: "Domains and webhooks:"},
		&cobra.Group{ID: "auth", Title: "Authentication:"},
	)
	root.SetHelpCommandGroupID("")
	root.SetCompletionCommandGroupID("")

	root.AddCommand(
		withGroup("email", a.newSendCmd()),
		withGroup("email", a.newEmailsCmd()),
		withGroup("email", a.newBatchCmd()),
		withGroup("setup", a.newDomainsCmd()),
		withGroup("setup", a.newWebhooksCmd()),
		withGroup("auth", a.newLoginCmd()),
		withGroup("auth", a.newLogoutCmd()),
		withGroup("auth", a.newWhoamiCmd()),
		withGroup("auth", a.newProfilesCmd()),
		a.newVersionCmd(),
	)
	root.InitDefaultCompletionCmd()
	markRuntimeErrors(root)
	return root
}

// RootCommand returns the command tree, including the completion command (for docs).
func (a *App) RootCommand() *cobra.Command {
	root := a.newRootCmd()
	root.InitDefaultHelpCmd()
	return root
}

func withGroup(id string, c *cobra.Command) *cobra.Command {
	c.GroupID = id
	return c
}

// markRuntimeErrors wraps every RunE so that errors returned while running a command exit
// with ExitError, while errors cobra raises before running it (unknown command, bad flags,
// wrong number of arguments) exit with ExitUsage.
func markRuntimeErrors(c *cobra.Command) {
	if c.RunE == nil && c.Run == nil && c.HasSubCommands() {
		// A command group: show help, and reject unknown subcommands with a usage error.
		c.RunE = func(cmd *cobra.Command, _ []string) error { return cmd.Help() }
	}
	if run := c.RunE; run != nil {
		c.RunE = func(cmd *cobra.Command, args []string) error {
			err := run(cmd, args)
			if err == nil {
				return nil
			}
			var ee *exitError
			var apiErr *agdiremail.Error
			if errors.As(err, &ee) || errors.As(err, &apiErr) {
				return err
			}
			return &exitError{code: ExitError, err: err}
		}
	}
	if c.Args == nil {
		c.Args = cobra.NoArgs
	}
	for _, sub := range c.Commands() {
		markRuntimeErrors(sub)
	}
}

// exitError carries an exit code. silent errors have already been reported.
type exitError struct {
	code   int
	err    error
	silent bool
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func usageError(err error) error { return &exitError{code: ExitUsage, err: err} }

func usageErrorf(format string, args ...any) error {
	return usageError(fmt.Errorf(format, args...))
}

func authErrorf(format string, args ...any) error {
	return &exitError{code: ExitAuth, err: fmt.Errorf(format, args...)}
}

func failf(format string, args ...any) error {
	return &exitError{code: ExitError, err: fmt.Errorf(format, args...)}
}

// silentFailure exits with ExitError without printing anything more.
func silentFailure(err error) error { return &exitError{code: ExitError, err: err, silent: true} }

var authCodes = map[agdiremail.ErrorCode]bool{
	agdiremail.CodeMissingAPIKey:    true,
	agdiremail.CodeInvalidAPIKey:    true,
	agdiremail.CodeRestrictedAPIKey: true,
}

// reportError prints err to stderr and returns the exit code.
func (a *App) reportError(cmd *cobra.Command, err error) int {
	code, errCode, status, message, hint := ExitUsage, "usage_error", 0, err.Error(), ""

	var ee *exitError
	var apiErr *agdiremail.Error
	switch {
	case errors.As(err, &apiErr):
		code, errCode, status, message = ExitError, string(apiErr.Code), apiErr.Status, apiErr.Message
		if authCodes[apiErr.Code] {
			code = ExitAuth
			switch apiErr.Code {
			case agdiremail.CodeRestrictedAPIKey:
				hint = "This API key can only send email; use a full-access key from https://app.agdir.email."
			default:
				hint = `Run "agdir-email login" to store a valid API key.`
			}
		}
	case errors.As(err, &ee):
		if ee.silent {
			return ee.code
		}
		code, message = ee.code, ee.err.Error()
		switch code {
		case ExitAuth:
			errCode = "auth_error"
		case ExitError:
			errCode = "error"
		}
	case errors.Is(err, context.Canceled):
		code, errCode = ExitError, "canceled"
	}
	if code == ExitUsage && cmd != nil {
		hint = fmt.Sprintf("Run '%s --help' for usage.", cmd.CommandPath())
	}

	if a.opts.json {
		body := map[string]any{"code": errCode, "message": message}
		if status != 0 {
			body["status"] = status
		}
		enc := json.NewEncoder(a.Stderr)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(map[string]any{"error": body})
		return code
	}
	if errCode == "usage_error" || errCode == "error" || errCode == "auth_error" || errCode == "canceled" {
		fmt.Fprintf(a.Stderr, "Error: %s\n", message)
	} else if status != 0 {
		fmt.Fprintf(a.Stderr, "Error: %s: %s (HTTP %d)\n", errCode, message, status)
	} else {
		fmt.Fprintf(a.Stderr, "Error: %s: %s\n", errCode, message)
	}
	if hint != "" {
		fmt.Fprintln(a.Stderr, hint)
	}
	return code
}

// getenv returns the environment variable, treating empty values as unset.
func (a *App) getenv(key string) string {
	if a.Getenv == nil {
		return os.Getenv(key)
	}
	return strings.TrimSpace(a.Getenv(key))
}

func (a *App) now() time.Time {
	if a.Now == nil {
		return time.Now()
	}
	return a.Now()
}

func (a *App) stdinIsTerminal() bool {
	if a.StdinIsTerminal == nil {
		return false
	}
	return a.StdinIsTerminal()
}
