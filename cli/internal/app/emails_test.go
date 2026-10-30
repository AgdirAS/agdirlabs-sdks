package app

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSendBuildsTheRequest(t *testing.T) {
	e := newEnv(t).withKey(keyFull)
	r := e.run("send", "--from", "Acme <hi@acme.com>", "--to", "ana@example.com", "--to", "Ben, Jr. <ben@example.com>",
		"--subject", "Welcome", "--html", "<p>Hi</p>", "--text", "Hi",
		"--cc", "cc@example.com", "--bcc", "bcc@example.com", "--reply-to", "support@acme.com",
		"--tag", "kind=welcome", "--tag", "plan=pro", "--header", "X-Entity-Ref-ID: 42", "--idempotency-key", "welcome/1").expect(t, 0)
	r.contains(t, "Sent email e-0001")

	req := e.api.lastRequest("POST", "/emails")
	want := map[string]any{
		"from": "Acme <hi@acme.com>", "to": []any{"ana@example.com", "Ben, Jr. <ben@example.com>"},
		"subject": "Welcome", "html": "<p>Hi</p>", "text": "Hi",
		"cc": []any{"cc@example.com"}, "bcc": []any{"bcc@example.com"}, "reply_to": []any{"support@acme.com"},
		"tags":    []any{map[string]any{"name": "kind", "value": "welcome"}, map[string]any{"name": "plan", "value": "pro"}},
		"headers": map[string]any{"X-Entity-Ref-ID": "42"},
	}
	if !reflect.DeepEqual(req.Body, want) {
		t.Fatalf("body = %#v\nwant %#v", req.Body, want)
	}
	if got := req.Header.Get("Idempotency-Key"); got != "welcome/1" {
		t.Fatalf("Idempotency-Key = %q", got)
	}
	if ua := req.Header.Get("User-Agent"); !strings.HasPrefix(ua, "agdirlabs-email-cli/"+Version+" agdirlabs-email-go/") {
		t.Fatalf("User-Agent = %q", ua)
	}
	if auth := req.Header.Get("Authorization"); auth != "Bearer "+keyFull {
		t.Fatalf("Authorization = %q", auth)
	}
}

func TestSendOutputModes(t *testing.T) {
	e := newEnv(t).withKey(keyFull)
	args := []string{"send", "--from", "hi@acme.com", "--to", "ana@example.com", "--subject", "S", "--text", "T"}
	m := e.run(append(args, "--json")...).expect(t, 0).json(t)
	if m["id"] != "e-0001-0000-4000-8000-000000000000" {
		t.Fatalf("json = %v", m)
	}
	r := e.run(append(args, "-q")...).expect(t, 0)
	if r.stdout != "e-0002-0000-4000-8000-000000000000\n" {
		t.Fatalf("quiet output = %q", r.stdout)
	}
}

func TestSendGeneratesAnIdempotencyKey(t *testing.T) {
	e := newEnv(t).withKey(keyFull)
	e.run("send", "--from", "hi@acme.com", "--to", "a@example.com", "--subject", "S", "--text", "T").expect(t, 0)
	if e.api.lastRequest("POST", "/emails").Header.Get("Idempotency-Key") == "" {
		t.Fatal("no Idempotency-Key header: retries could send twice")
	}
}

func TestSendSchedule(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"in 1h", "2026-10-30T13:00:00Z"},
		{"in 2 days", "2026-11-01T12:00:00Z"},
		{"in 1 day 30 minutes", "2026-10-31T12:30:00Z"},
		{"in 90m", "2026-10-30T13:30:00Z"},
		{"2026-11-02T09:00:00+01:00", "2026-11-02T09:00:00+01:00"},
	} {
		t.Run(tc.in, func(t *testing.T) {
			e := newEnv(t).withKey(keyFull)
			r := e.run("send", "--from", "hi@acme.com", "--to", "a@example.com", "--subject", "S", "--text", "T", "--schedule", tc.in).expect(t, 0)
			r.contains(t, "Scheduled email e-0001")
			got, _ := e.api.lastRequest("POST", "/emails").Body["scheduled_at"].(string)
			gt, err1 := time.Parse(time.RFC3339, got)
			wt, err2 := time.Parse(time.RFC3339, tc.want)
			if err1 != nil || err2 != nil || !gt.Equal(wt) {
				t.Fatalf("scheduled_at = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSendBodyFromFilesAndStdin(t *testing.T) {
	e := newEnv(t).withKey(keyFull)
	html := writeFile(t, "body.html", "<h1>Report</h1>")
	e.stdin = "plain text from stdin\n"
	e.run("send", "--from", "hi@acme.com", "--to", "a@example.com", "--subject", "S", "--html-file", html, "--text-file", "-").expect(t, 0)
	body := e.api.lastRequest("POST", "/emails").Body
	if body["html"] != "<h1>Report</h1>" || body["text"] != "plain text from stdin\n" {
		t.Fatalf("body = %v", body)
	}

	// No body flag and piped stdin: stdin is the text part.
	e.stdin = "piped"
	e.run("send", "--from", "hi@acme.com", "--to", "a@example.com", "--subject", "S").expect(t, 0)
	if body := e.api.lastRequest("POST", "/emails").Body; body["text"] != "piped" || body["html"] != nil {
		t.Fatalf("body = %v", body)
	}
}

func TestSendUsageErrors(t *testing.T) {
	base := []string{"send", "--from", "hi@acme.com", "--to", "a@example.com", "--subject", "S"}
	for name, tc := range map[string]struct {
		args []string
		msg  string
	}{
		"missing from":   {[]string{"send", "--to", "a@example.com", "--subject", "S", "--text", "T"}, `required flag(s) "from" not set`},
		"no body":        {base, "no body"},
		"bad tag":        {append(base, "--text", "T", "--tag", "novalue"), "invalid --tag"},
		"bad header":     {append(base, "--text", "T", "--header", "NoColon"), "invalid --header"},
		"bad schedule":   {append(base, "--text", "T", "--schedule", "tomorrow-ish"), "invalid --schedule"},
		"both html":      {append(base, "--html", "x", "--html-file", "y"), "none of the others can be"},
		"missing file":   {append(base, "--html-file", "/does/not/exist"), "/does/not/exist"},
		"unknown flag":   {append(base, "--text", "T", "--colour"), "unknown flag: --colour"},
		"unexpected arg": {append(base, "--text", "T", "extra"), `unknown command "extra"`},
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t).withKey(keyFull)
			e.tty = true // don't read stdin
			e.run(tc.args...).expect(t, ExitUsage).errContains(t, tc.msg, "--help")
			if n := e.api.count("POST", "/emails"); n != 0 {
				t.Fatalf("%d requests sent", n)
			}
		})
	}
}

func TestSendAPIError(t *testing.T) {
	e := newEnv(t).withKey(keyFull)
	e.run("send", "--from", "hi@acme.com", "--to", "not-an-address", "--subject", "S", "--text", "T").
		expect(t, ExitError).errContains(t, "Error: validation_error: Invalid `to` field", "(HTTP 422)")

	r := e.run("send", "--json", "--from", "hi@acme.com", "--to", "nope", "--subject", "S", "--text", "T").expect(t, ExitError)
	if !strings.Contains(r.stderr, `"code": "validation_error"`) && !strings.Contains(r.stderr, `"code":"validation_error"`) {
		t.Fatalf("stderr = %s", r.stderr)
	}
	if !strings.Contains(r.stderr, `"status":422`) || r.stdout != "" {
		t.Fatalf("stdout=%q stderr=%s", r.stdout, r.stderr)
	}
}

func TestEmailsGetAndCancel(t *testing.T) {
	e := newEnv(t).withKey(keyFull)
	id := strings.TrimSpace(e.run("send", "-q", "--from", "hi@acme.com", "--to", "a@example.com", "--subject", "Hello",
		"--text", "T", "--schedule", "in 1h").expect(t, 0).stdout)

	e.run("emails", "get", id).expect(t, 0).contains(t, "Status:", "scheduled", "Subject:", "Hello", "a@example.com", "2026-10-30 13:00:00 UTC")
	m := e.run("emails", "get", id, "--json").expect(t, 0).json(t)
	if m["status"] != "scheduled" || m["id"] != id {
		t.Fatalf("json = %v", m)
	}

	e.run("emails", "cancel", id).expect(t, 0).contains(t, "Canceled email "+id)
	e.run("emails", "get", id, "--json").expect(t, 0)
	if e.run("emails", "get", id, "--json").json(t)["status"] != "canceled" {
		t.Fatal("not canceled")
	}
	e.run("emails", "cancel", id).expect(t, ExitError).errContains(t, "Only scheduled emails")
	e.run("emails", "get", "missing").expect(t, ExitError).errContains(t, "not_found", "(HTTP 404)")
}

func TestBatchSend(t *testing.T) {
	e := newEnv(t).withKey(keyFull)
	file := writeFile(t, "batch.json", `[
		{"from": "hi@acme.com", "to": "a@example.com", "subject": "One", "text": "1"},
		{"from": "hi@acme.com", "to": ["bad"], "subject": "Two", "text": "2"},
		{"from": "hi@acme.com", "to": ["c@example.com"], "subject": "Three", "html": "<p>3</p>", "scheduled_at": "2026-11-01T10:00:00Z"}
	]`)

	e.run("batch", "send", file).expect(t, ExitError).errContains(t, "validation_error", "[1]")

	r := e.run("batch", "send", file, "--permissive").expect(t, ExitError)
	r.contains(t, "0  ✓  e-0001", "1  ✗  Invalid `to` field", "2  ✓  e-0002", "Accepted 2 of 3 emails.")
	r.errContains(t, "1 of 3 emails were rejected")
	if got := e.api.lastRequest("POST", "/emails/batch").Header.Get("x-batch-validation"); got != "permissive" {
		t.Fatalf("x-batch-validation = %q", got)
	}
	body := e.api.lastRequest("POST", "/emails/batch").Raw
	if !strings.Contains(string(body), `"to":["a@example.com"]`) || !strings.Contains(string(body), `"scheduled_at":"2026-11-01T10:00:00Z"`) {
		t.Fatalf("batch body = %s", body)
	}

	m := e.run("batch", "send", file, "--permissive", "--json").expect(t, ExitError).json(t)
	results := m["results"].([]any)
	if len(results) != 3 || results[1].(map[string]any)["error"] == nil || results[2].(map[string]any)["id"] == nil {
		t.Fatalf("results = %v", results)
	}

	ok := writeFile(t, "ok.json", `[{"from": "hi@acme.com", "to": "a@example.com", "subject": "One", "text": "1"}]`)
	e.stdin = mustRead(t, ok)
	if out := e.run("batch", "send", "-", "-q").expect(t, 0).stdout; !strings.HasPrefix(out, "e-") {
		t.Fatalf("quiet = %q", out)
	}
}

func TestBatchSendFileErrors(t *testing.T) {
	e := newEnv(t).withKey(keyFull)
	e.run("batch", "send", writeFile(t, "x.json", `{"from": "x"}`)).expect(t, ExitUsage).errContains(t, "expected a JSON array")
	e.run("batch", "send", writeFile(t, "x.json", `[]`)).expect(t, ExitUsage).errContains(t, "1 to 100 emails")
	e.run("batch", "send", writeFile(t, "x.json", `[{"form": "typo"}]`)).expect(t, ExitUsage).errContains(t, `unknown field "form"`)
	big := "[" + strings.TrimSuffix(strings.Repeat(`{"from":"a@b.c","to":"d@e.f","subject":"s","text":"t"},`, 101), ",") + "]"
	e.run("batch", "send", writeFile(t, "big.json", big)).expect(t, ExitUsage).errContains(t, "got 101")
}

func TestParseSchedule(t *testing.T) {
	for _, bad := range []string{"", "1h", "in", "in -1h", "in 0m", "in 3 fortnights", "2026-11-02 09:00"} {
		if _, err := parseSchedule(bad, fixedNow); err == nil {
			t.Errorf("parseSchedule(%q) accepted", bad)
		}
	}
	got, err := parseSchedule("In 1W", fixedNow)
	if err != nil || !got.Equal(fixedNow.Add(7*24*time.Hour)) {
		t.Fatalf("got %v, %v", got, err)
	}
}
