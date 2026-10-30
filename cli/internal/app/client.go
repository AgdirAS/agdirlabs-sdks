package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	agdiremail "github.com/AgdirAS/agdirlabs-sdks/go"
	"golang.org/x/term"
)

// Key sources reported by `whoami`.
const (
	sourceFlag    = "flag"
	sourceEnv     = "env"
	sourceProfile = "profile"
)

// credentials are the resolved API key and base URL.
type credentials struct {
	apiKey    string
	keySource string
	baseURL   string
	profile   string
	cfgPath   string
}

// resolveCredentials applies the precedence --api-key > $AGDIR_EMAIL_API_KEY > profile, and
// --base-url > $AGDIR_EMAIL_BASE_URL > profile > default. A missing key is not an error here.
func (a *App) resolveCredentials() (*credentials, error) {
	cfg, path, err := a.loadConfig()
	if err != nil {
		return nil, err
	}
	c := &credentials{profile: a.profileName(cfg), cfgPath: path}
	prof := cfg.Profiles[c.profile]
	if a.opts.profile != "" && prof == nil && a.opts.apiKey == "" && a.getenv(agdiremail.EnvAPIKey) == "" {
		return nil, authErrorf("profile %q is not logged in; run \"agdir-email login --profile %s\"", c.profile, c.profile)
	}

	switch {
	case a.opts.apiKey != "":
		c.apiKey, c.keySource = a.opts.apiKey, sourceFlag
	case a.getenv(agdiremail.EnvAPIKey) != "":
		c.apiKey, c.keySource = a.getenv(agdiremail.EnvAPIKey), sourceEnv
	case prof != nil && prof.APIKey != "":
		c.apiKey, c.keySource = prof.APIKey, sourceProfile
	}

	switch {
	case a.opts.baseURL != "":
		c.baseURL = a.opts.baseURL
	case a.getenv(agdiremail.EnvBaseURL) != "":
		c.baseURL = a.getenv(agdiremail.EnvBaseURL)
	case prof != nil && prof.BaseURL != "":
		c.baseURL = prof.BaseURL
	default:
		c.baseURL = agdiremail.DefaultBaseURL
	}
	c.baseURL = strings.TrimRight(c.baseURL, "/")
	return c, nil
}

// client returns an API client, or an auth error when no key is configured.
func (a *App) client() (*agdiremail.Client, error) {
	c, err := a.resolveCredentials()
	if err != nil {
		return nil, err
	}
	if c.apiKey == "" {
		return nil, authErrorf("no API key: run \"agdir-email login\", set %s or pass --api-key (create a key at https://app.agdir.email)", agdiremail.EnvAPIKey)
	}
	return a.newClient(c.apiKey, c.baseURL)
}

func (a *App) newClient(apiKey, baseURL string) (*agdiremail.Client, error) {
	base := http.DefaultTransport
	hc := &http.Client{}
	if a.HTTPClient != nil {
		*hc = *a.HTTPClient
		if a.HTTPClient.Transport != nil {
			base = a.HTTPClient.Transport
		}
	}
	hc.Transport = userAgentTransport{base: base}
	return agdiremail.NewClient(
		agdiremail.WithAPIKey(apiKey),
		agdiremail.WithBaseURL(baseURL),
		agdiremail.WithHTTPClient(hc),
	)
}

// userAgent identifies the CLI (and the SDK it is built on) to the API.
func userAgent() string {
	return "agdirlabs-email-cli/" + Version + " " + agdiremail.UserAgent
}

type userAgentTransport struct{ base http.RoundTripper }

func (t userAgentTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("User-Agent", userAgent())
	return t.base.RoundTrip(r)
}

// access describes what an API key may do: "full" or "sending".
type access string

const (
	accessFull    access = "full"
	accessSending access = "sending"
)

func (k access) describe() string {
	if k == accessSending {
		return "sending access (can send email; cannot read or manage resources)"
	}
	return "full access"
}

// checkAccess validates a key with a cheap request (the first page of domains, one item).
func checkAccess(ctx context.Context, client *agdiremail.Client) (access, error) {
	_, err := client.Domains.List(ctx, &agdiremail.ListParams{Limit: 1})
	var apiErr *agdiremail.Error
	if errors.As(err, &apiErr) && apiErr.Code == agdiremail.CodeRestrictedAPIKey {
		return accessSending, nil
	}
	if err != nil {
		return "", err
	}
	return accessFull, nil
}

// maskKey shows enough of a key to recognise it: re_AbCd…wxyz.
func maskKey(key string) string {
	if len(key) <= 12 {
		return strings.Repeat("•", len(key))
	}
	return key[:7] + "…" + key[len(key)-4:]
}

func isTerminal(f *os.File) bool { return term.IsTerminal(int(f.Fd())) }

func readSecretFromTerminal() (string, error) {
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	return string(b), err
}

// readLine reads the first line of r.
func readLine(r io.Reader) (string, error) {
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// readInput reads a file, or stdin for "-".
func (a *App) readInput(path string) ([]byte, error) {
	if path == "-" {
		b, err := io.ReadAll(a.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		return b, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, usageError(err)
	}
	return b, nil
}
