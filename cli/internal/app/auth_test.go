package app

import (
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"testing"
)

func readConfig(t *testing.T, path string) Config {
	t.Helper()
	var c Config
	if err := json.Unmarshal([]byte(mustRead(t, path)), &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestLoginStoresTheKeyPrivately(t *testing.T) {
	e := newEnv(t)
	e.stdin = keyFull + "\n"
	e.run("login").expect(t, 0).contains(t, `profile "default"`, "full access", e.cfgPath)

	cfg := readConfig(t, e.cfgPath)
	if cfg.CurrentProfile != "default" || cfg.Profiles["default"].APIKey != keyFull || cfg.Profiles["default"].BaseURL != e.api.srv.URL {
		t.Fatalf("config = %+v", cfg)
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(e.cfgPath)
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("config permissions %o, want 600", perm)
		}
	}
	// The key was validated with a cheap request.
	if q := e.api.lastRequest("GET", "/domains").Query; q != "limit=1" {
		t.Fatalf("validation query = %q", q)
	}

	// Commands now use the stored key.
	e.run("send", "--from", "hi@acme.com", "--to", "a@example.com", "--subject", "S", "--text", "T").expect(t, 0)
}

func TestLoginPromptsOnATerminal(t *testing.T) {
	e := newEnv(t)
	e.tty, e.secret = true, keySending
	e.run("login", "--profile", "ci").expect(t, 0).contains(t, `profile "ci"`, "sending access").errContains(t, "Paste your API key")
	if readConfig(t, e.cfgPath).Profiles["ci"].APIKey != keySending {
		t.Fatal("key not stored")
	}
}

func TestLoginRejectsABadKey(t *testing.T) {
	e := newEnv(t)
	e.stdin = "re_invalid\n"
	e.run("login").expect(t, ExitAuth).errContains(t, "invalid_api_key")
	if _, err := os.Stat(e.cfgPath); !os.IsNotExist(err) {
		t.Fatal("config written for a rejected key")
	}
	e.run("login").expect(t, ExitUsage).errContains(t, "no API key given")

	// --no-verify stores without a request.
	e.run("login", "--api-key", "re_offline", "--no-verify").expect(t, 0).contains(t, "not verified")
}

func TestKeyPrecedence(t *testing.T) {
	e := newEnv(t)
	e.stdin = keySending
	e.run("login").expect(t, 0)
	send := []string{"send", "--from", "hi@acme.com", "--to", "a@example.com", "--subject", "S", "--text", "T"}
	auth := func() string { return e.api.lastRequest("POST", "/emails").Header.Get("Authorization") }

	e.run(send...).expect(t, 0)
	if auth() != "Bearer "+keySending {
		t.Fatalf("profile key not used: %s", auth())
	}
	e.withKey(keyFull).run(send...).expect(t, 0)
	if auth() != "Bearer "+keyFull {
		t.Fatalf("env key not preferred: %s", auth())
	}
	e.run(append(send, "--api-key", "re_flag_wins")...).expect(t, ExitAuth)
	if auth() != "Bearer re_flag_wins" {
		t.Fatalf("flag key not preferred: %s", auth())
	}
}

func TestBaseURLPrecedence(t *testing.T) {
	e := newEnv(t).withKey(keyFull)
	other := newFakeAPI(t)
	e.run("send", "--base-url", other.srv.URL+"/", "--from", "hi@acme.com", "--to", "a@example.com", "--subject", "S", "--text", "T").expect(t, 0)
	if other.count("POST", "/emails") != 1 || e.api.count("POST", "/emails") != 0 {
		t.Fatal("--base-url not used")
	}
}

func TestNoKeyIsAnAuthError(t *testing.T) {
	e := newEnv(t)
	e.run("domains", "list").expect(t, ExitAuth).errContains(t, "no API key", "agdir-email login", "AGDIR_EMAIL_API_KEY")
	e.run("whoami").expect(t, ExitAuth).errContains(t, "not logged in")
	e.run("domains", "list", "--profile", "nope").expect(t, ExitAuth).errContains(t, `profile "nope" is not logged in`)
	r := e.run("domains", "list", "--json").expect(t, ExitAuth)
	if !strings.Contains(r.stderr, `"code":"auth_error"`) {
		t.Fatalf("stderr = %s", r.stderr)
	}
}

func TestRejectedKeys(t *testing.T) {
	e := newEnv(t).withKey("re_revoked")
	e.run("domains", "list").expect(t, ExitAuth).errContains(t, "invalid_api_key: API key is invalid (HTTP 403)", "agdir-email login")
	e.withKey(keySending).run("domains", "list").expect(t, ExitAuth).errContains(t, "restricted_api_key", "can only send email")
}

func TestWhoami(t *testing.T) {
	e := newEnv(t)
	e.stdin = keyFull
	e.run("login").expect(t, 0)
	r := e.run("whoami").expect(t, 0).contains(t, "re_full…1234", `profile "default"`, e.api.srv.URL, "full access")
	if strings.Contains(r.stdout, keyFull) {
		t.Fatal("whoami printed the full key")
	}
	m := e.withKey(keySending).run("status", "--json").expect(t, 0).json(t)
	if m["key_source"] != "env" || m["access"] != "sending" || m["api_key"] != "re_send…5678" {
		t.Fatalf("json = %v", m)
	}
	e.withKey("re_nope_nope_nope").run("whoami").expect(t, ExitAuth)
}

func TestProfilesAndLogout(t *testing.T) {
	e := newEnv(t)
	e.stdin = keyFull
	e.run("login").expect(t, 0)
	e.stdin = keySending
	e.run("login", "--profile", "staging").expect(t, 0)

	e.run("profiles", "list").expect(t, 0).contains(t, "*  staging", "default")
	e.run("profiles", "use", "default").expect(t, 0).contains(t, `Now using profile "default"`)
	if readConfig(t, e.cfgPath).CurrentProfile != "default" {
		t.Fatal("current profile not switched")
	}
	e.run("profiles", "use", "missing").expect(t, ExitError)

	// --profile and $AGDIR_EMAIL_PROFILE select a profile for one command.
	e.run("whoami", "--profile", "staging").expect(t, 0).contains(t, "sending access")
	e.vars[EnvProfile] = "staging"
	e.run("whoami").expect(t, 0).contains(t, "sending access")
	delete(e.vars, EnvProfile)

	e.run("logout", "--profile", "staging").expect(t, 0).contains(t, `Logged out of profile "staging"`)
	if _, ok := readConfig(t, e.cfgPath).Profiles["staging"]; ok {
		t.Fatal("profile not removed")
	}
	e.run("logout", "--profile", "staging").expect(t, 0).contains(t, "not logged in")
	m := e.run("logout", "--all", "--json").expect(t, 0).json(t)
	if len(m["removed"].([]any)) != 1 {
		t.Fatalf("json = %v", m)
	}
	e.run("whoami").expect(t, ExitAuth)
}
