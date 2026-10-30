package app

import (
	"strings"
	"testing"
)

func TestUsageErrorsExit2(t *testing.T) {
	e := newEnv(t).withKey(keyFull)
	e.run("frobnicate").expect(t, ExitUsage).errContains(t, `unknown command "frobnicate"`, "Run 'agdir-email --help' for usage.")
	e.run("domains", "frobnicate").expect(t, ExitUsage).errContains(t, `unknown command "frobnicate" for "agdir-email domains"`)
	e.run("emails", "get").expect(t, ExitUsage).errContains(t, "accepts 1 arg(s), received 0", "agdir-email emails get --help")
	e.run("--nope").expect(t, ExitUsage).errContains(t, "unknown flag: --nope")
	r := e.run("domains", "frobnicate", "--json").expect(t, ExitUsage)
	if !strings.Contains(r.stderr, `"code":"usage_error"`) {
		t.Fatalf("stderr = %s", r.stderr)
	}
}

func TestHelpAndGroups(t *testing.T) {
	e := newEnv(t)
	e.run().expect(t, 0).contains(t, "Email:", "send", "Domains and webhooks:", "Authentication:", "login")
	e.run("domains").expect(t, 0).contains(t, "check", "Check the domain's DNS records in live DNS")
	e.run("send", "--help").expect(t, 0).contains(t, "--schedule", "--idempotency-key", "Examples:")
}

func TestVersion(t *testing.T) {
	e := newEnv(t)
	e.run("version").expect(t, 0).contains(t, "agdir-email "+Version, "Go SDK")
	e.run("--version").expect(t, 0).contains(t, "agdir-email "+Version)
	m := e.run("version", "--json").expect(t, 0).json(t)
	if m["version"] != Version || m["sdk"] == "" || !strings.HasPrefix(m["user_agent"].(string), "agdirlabs-email-cli/") {
		t.Fatalf("json = %v", m)
	}
	if out := e.run("version", "-q").expect(t, 0).stdout; out != Version+"\n" {
		t.Fatalf("quiet = %q", out)
	}
}

func TestCompletion(t *testing.T) {
	e := newEnv(t)
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		if r := e.run("completion", shell).expect(t, 0); !strings.Contains(r.stdout, "agdir-email") {
			t.Fatalf("%s completion is empty", shell)
		}
	}
	e.run("completion", "tcsh").expect(t, ExitUsage)
}

func TestMaskKey(t *testing.T) {
	if got := maskKey("re_123"); got != "••••••" {
		t.Fatalf("short key = %q", got)
	}
	if got := maskKey("re_AbCdEfGhIjKlMn"); got != "re_AbCd…KlMn" {
		t.Fatalf("got %q", got)
	}
}
