package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLive runs the CLI against a real Agdir Email API (never production). Opt in with:
//
//	AGDIR_EMAIL_LIVE_BASE_URL=https://your-test-instance.example \
//	AGDIR_EMAIL_LIVE_API_KEY=re_… AGDIR_EMAIL_LIVE_FROM=hi@your-verified-domain.example \
//	go test -run TestLive -v ./...
func TestLive(t *testing.T) {
	base, key, from := os.Getenv("AGDIR_EMAIL_LIVE_BASE_URL"), os.Getenv("AGDIR_EMAIL_LIVE_API_KEY"), os.Getenv("AGDIR_EMAIL_LIVE_FROM")
	if base == "" || key == "" || from == "" {
		t.Skip("set AGDIR_EMAIL_LIVE_BASE_URL, AGDIR_EMAIL_LIVE_API_KEY and AGDIR_EMAIL_LIVE_FROM to run")
	}
	cfg := filepath.Join(t.TempDir(), "config.json")
	vars := map[string]string{EnvConfig: cfg}
	stdin := ""
	run := func(args ...string) result {
		t.Helper()
		var out, errb bytes.Buffer
		a := &App{
			Stdin: strings.NewReader(stdin), Stdout: &out, Stderr: &errb,
			Getenv: func(k string) string { return vars[k] }, Now: time.Now,
			StdinIsTerminal: func() bool { return false },
		}
		code := a.Run(append([]string{"--base-url", base}, args...))
		stdin = ""
		return result{out.String(), errb.String(), code}
	}
	jsonOf := func(r result) map[string]any {
		t.Helper()
		var m map[string]any
		if err := json.Unmarshal([]byte(r.stdout), &m); err != nil {
			t.Fatalf("not JSON: %v\n%s\n%s", err, r.stdout, r.stderr)
		}
		return m
	}

	// Auth: a bad key is rejected with exit code 3; the real key logs in.
	stdin = "re_invalid\n"
	run("login").expect(t, ExitAuth).errContains(t, "invalid_api_key")
	stdin = key + "\n"
	run("login", "--profile", "live").expect(t, 0).contains(t, "full access")
	run("whoami").expect(t, 0).contains(t, `profile "live"`, "full access")
	run("--api-key", "re_invalid", "domains", "list").expect(t, ExitAuth)

	// send (scheduled) → get → cancel → get
	id := strings.TrimSpace(run("send", "-q", "--from", "CLI live test <"+from+">", "--to", "delivered@example.com",
		"--subject", "CLI live test", "--text", "Scheduled by the agdir-email CLI live test.", "--html", "<p>Scheduled</p>",
		"--tag", "suite=cli", "--header", "X-Test: live", "--schedule", "in 1h").expect(t, 0).stdout)
	if id == "" {
		t.Fatal("no email id")
	}
	if s := jsonOf(run("emails", "get", id, "--json").expect(t, 0))["status"]; s != "scheduled" {
		t.Fatalf("status %v, want scheduled", s)
	}
	run("emails", "get", id).expect(t, 0).contains(t, "scheduled", "CLI live test")
	run("emails", "cancel", id).expect(t, 0).contains(t, "Canceled email "+id)
	if s := jsonOf(run("emails", "get", id, "--json").expect(t, 0))["status"]; s != "canceled" {
		t.Fatalf("status %v, want canceled", s)
	}

	// Idempotent replay returns the same email.
	idem := fmt.Sprintf("cli-live-%d", time.Now().UnixNano())
	send := []string{"send", "--json", "--from", from, "--to", "delivered@example.com", "--subject", "Idempotent", "--text", "once", "--schedule", "in 2h", "--idempotency-key", idem}
	first := jsonOf(run(send...).expect(t, 0))["id"]
	second := jsonOf(run(send...).expect(t, 0))["id"]
	if first == nil || first != second {
		t.Fatalf("idempotent replay: %v != %v", first, second)
	}
	run("emails", "cancel", first.(string)).expect(t, 0)

	// Errors: validation_error exits 1 with the API's code.
	run("send", "--from", from, "--to", "not-an-address", "--subject", "x", "--text", "x").expect(t, ExitError).errContains(t, "validation_error")

	// Permissive batch with one bad recipient.
	batch := fmt.Sprintf(`[{"from":%q,"to":"delivered@example.com","subject":"Batch 1","text":"1","scheduled_at":%q},
		{"from":%q,"to":"bad-address","subject":"Batch 2","text":"2"}]`, from, time.Now().Add(time.Hour).UTC().Format(time.RFC3339), from)
	bm := jsonOf(run("batch", "send", writeFile(t, "batch.json", batch), "--permissive", "--json").expect(t, ExitError))
	if len(bm["data"].([]any)) != 1 || len(bm["errors"].([]any)) != 1 {
		t.Fatalf("batch = %v", bm)
	}
	run("emails", "cancel", bm["data"].([]any)[0].(map[string]any)["id"].(string)).expect(t, 0)

	// domains add → get → verify → list → check → delete → get (not found)
	name := fmt.Sprintf("cli-%d.test", time.Now().UnixNano())
	d := jsonOf(run("domains", "add", name, "--json").expect(t, 0))
	did := d["id"].(string)
	t.Cleanup(func() { run("domains", "delete", did, "--yes") })
	if len(d["records"].([]any)) == 0 {
		t.Fatalf("no records: %v", d)
	}
	run("domains", "get", name).expect(t, 0).contains(t, did, "agdir._domainkey")
	run("domains", "verify", name).expect(t, 0)
	lm := jsonOf(run("domains", "list", "--json").expect(t, 0))
	found := false
	for _, x := range lm["data"].([]any) {
		found = found || x.(map[string]any)["id"] == did
	}
	if !found {
		t.Fatal("domain not listed")
	}
	run("domains", "list", "--limit", "1").expect(t, 0)
	// The .test domain has no DNS: every required record is reported missing.
	cm := jsonOf(run("domains", "check", did, "--json").expect(t, ExitError))
	if cm["ok"] != false || len(cm["records"].([]any)) != len(d["records"].([]any)) {
		t.Fatalf("check = %v", cm)
	}
	run("domains", "delete", name, "--yes").expect(t, 0)
	run("domains", "get", did).expect(t, ExitError).errContains(t, "not_found")

	// webhooks create → get → update → rotate → deliveries → delete
	wh := jsonOf(run("webhooks", "create", "--endpoint", "https://93.184.216.34/hook", "--event", "delivered,bounced", "--json").expect(t, 0))
	wid := wh["id"].(string)
	t.Cleanup(func() { run("webhooks", "delete", wid, "--yes") })
	run("webhooks", "get", wid).expect(t, 0).contains(t, "https://93.184.216.34/hook", "whsec_")
	run("webhooks", "update", wid, "--status", "disabled").expect(t, 0)
	if s := jsonOf(run("webhooks", "get", wid, "--json").expect(t, 0))["status"]; s != "disabled" {
		t.Fatalf("status %v", s)
	}
	rot := jsonOf(run("webhooks", "rotate-secret", wid, "--json").expect(t, 0))
	if rot["signing_secret"] == wh["signing_secret"] {
		t.Fatal("secret not rotated")
	}
	if n := len(jsonOf(run("webhooks", "deliveries", "list", wid, "--json").expect(t, 0))["data"].([]any)); n != 0 {
		t.Fatalf("%d deliveries", n)
	}
	run("webhooks", "delete", wid, "--yes").expect(t, 0)
	run("webhooks", "get", wid).expect(t, ExitError).errContains(t, "not_found")

	run("logout", "--all").expect(t, 0)
}
