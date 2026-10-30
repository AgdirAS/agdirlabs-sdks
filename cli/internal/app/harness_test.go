package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixedNow is the clock of command tests.
var fixedNow = time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)

func init() { time.Local = time.UTC }

type testEnv struct {
	t        *testing.T
	api      *fakeAPI
	vars     map[string]string
	cfgPath  string
	resolver Resolver
	tty      bool
	stdin    string
	secret   string // returned by the hidden prompt
}

type result struct {
	stdout, stderr string
	code           int
}

// newEnv returns a test environment pointing at a fresh fake API, with no key configured.
func newEnv(t *testing.T) *testEnv {
	t.Helper()
	api := newFakeAPI(t)
	cfg := filepath.Join(t.TempDir(), "agdir-email", "config.json")
	return &testEnv{
		t: t, api: api, cfgPath: cfg,
		vars: map[string]string{EnvConfig: cfg, "AGDIR_EMAIL_BASE_URL": api.srv.URL},
	}
}

// withKey sets AGDIR_EMAIL_API_KEY.
func (e *testEnv) withKey(key string) *testEnv {
	e.vars["AGDIR_EMAIL_API_KEY"] = key
	return e
}

func (e *testEnv) run(args ...string) result {
	e.t.Helper()
	var out, errb bytes.Buffer
	a := &App{
		Stdin:           strings.NewReader(e.stdin),
		Stdout:          &out,
		Stderr:          &errb,
		Getenv:          func(k string) string { return e.vars[k] },
		Resolver:        e.resolver,
		Now:             func() time.Time { return fixedNow },
		StdinIsTerminal: func() bool { return e.tty },
		ReadSecret:      func() (string, error) { return e.secret, nil },
	}
	code := a.Run(args)
	e.stdin = ""
	return result{out.String(), errb.String(), code}
}

func (r result) expect(t *testing.T, code int) result {
	t.Helper()
	if r.code != code {
		t.Fatalf("exit code %d, want %d\nstdout:\n%s\nstderr:\n%s", r.code, code, r.stdout, r.stderr)
	}
	return r
}

func (r result) contains(t *testing.T, parts ...string) result {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(r.stdout, p) {
			t.Fatalf("stdout does not contain %q:\n%s", p, r.stdout)
		}
	}
	return r
}

func (r result) errContains(t *testing.T, parts ...string) result {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(r.stderr, p) {
			t.Fatalf("stderr does not contain %q:\n%s", p, r.stderr)
		}
	}
	return r
}

// json decodes stdout.
func (r result) json(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(r.stdout), &m); err != nil {
		t.Fatalf("stdout is not a JSON object: %v\n%s", err, r.stdout)
	}
	return m
}

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
