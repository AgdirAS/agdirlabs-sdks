package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestWebhooksLifecycle(t *testing.T) {
	e := newEnv(t).withKey(keyFull)
	r := e.run("webhooks", "create", "--endpoint", "https://acme.com/hook", "--event", "delivered,email.bounced", "--event", "bounced").expect(t, 0)
	r.contains(t, "Created webhook w-0001", "Signing secret: whsec_")
	if got := e.api.lastRequest("POST", "/webhooks").Body["events"]; !reflect.DeepEqual(got, []any{"email.delivered", "email.bounced"}) {
		t.Fatalf("events = %v", got)
	}
	id := "w-0001-0000-4000-8000-000000000000"

	m := e.run("webhooks", "create", "--endpoint", "https://acme.com/all", "--event", "all", "--json").expect(t, 0).json(t)
	if !strings.HasPrefix(m["signing_secret"].(string), "whsec_") {
		t.Fatalf("json = %v", m)
	}
	if n := len(e.api.lastRequest("POST", "/webhooks").Body["events"].([]any)); n != 9 {
		t.Fatalf("all → %d events", n)
	}

	e.run("webhooks", "list").expect(t, 0).contains(t, id, "https://acme.com/hook", "enabled", "delivered,bounced", "https://acme.com/all", "all")
	e.run("webhooks", "get", id).expect(t, 0).contains(t, "Endpoint:", "https://acme.com/hook", "Signing secret:", "whsec_")

	e.run("webhooks", "update", id, "--status", "disabled").expect(t, 0).contains(t, "Updated webhook "+id)
	if body := e.api.lastRequest("PATCH", "/webhooks/"+id).Body; !reflect.DeepEqual(body, map[string]any{"status": "disabled"}) {
		t.Fatalf("patch body = %v", body)
	}
	e.run("webhooks", "update", id).expect(t, ExitUsage).errContains(t, "nothing to update")
	e.run("webhooks", "update", id, "--status", "paused").expect(t, ExitUsage)

	e.run("webhooks", "rotate-secret", id).expect(t, 0).contains(t, "New signing secret: whsec_cm90YXRlZA==")
	if out := e.run("webhooks", "rotate-secret", id, "-q").expect(t, 0).stdout; out != "whsec_cm90YXRlZA==\n" {
		t.Fatalf("quiet = %q", out)
	}

	e.run("webhooks", "deliveries", "list", id).expect(t, 0).contains(t, "evt-1", "email.delivered", "success")
	e.run("webhooks", "deliveries", "get", id, "evt-1").expect(t, 0).contains(t, "Payload:", `"email_id": "e-1"`)
	m = e.run("webhooks", "deliveries", "get", id, "evt-1", "--json").expect(t, 0).json(t)
	if m["payload"].(map[string]any)["type"] != "email.delivered" {
		t.Fatalf("json = %v", m)
	}
	e.run("webhooks", "deliveries", "attempts", id, "evt-1").expect(t, 0).contains(t, "att-1", "500", "upstream error", "att-2", "200")
	m = e.run("webhooks", "deliveries", "attempts", id, "evt-1", "--json").expect(t, 0).json(t)
	if a := m["data"].([]any)[0].(map[string]any); a["http_status"] != float64(500) {
		t.Fatalf("attempt = %v", a)
	}
	e.run("webhooks", "deliveries", "replay", id, "evt-1").expect(t, 0).contains(t, "Replaying delivery evt-1")
	e.run("webhooks", "deliveries", "get", id, "nope").expect(t, ExitError).errContains(t, "not_found")

	e.run("webhooks", "delete", id, "--yes").expect(t, 0).contains(t, "Deleted webhook "+id)
	e.run("webhooks", "get", id).expect(t, ExitError).errContains(t, "not_found")
}

func TestWebhooksCreateValidation(t *testing.T) {
	e := newEnv(t).withKey(keyFull)
	e.run("webhooks", "create", "--endpoint", "https://acme.com/hook").expect(t, ExitUsage).errContains(t, `"event" not set`)
	e.run("webhooks", "create", "--endpoint", "https://acme.com/hook", "--event", "email.read").expect(t, ExitUsage).errContains(t, `unknown event "email.read"`)
	e.run("webhooks", "create", "--endpoint", "http://acme.com/hook", "--event", "sent").expect(t, ExitError).errContains(t, "validation_error")
	if e.api.count("POST", "/webhooks") != 1 {
		t.Fatal("invalid events were sent to the API")
	}
}

// signed returns Standard Webhooks headers for payload.
func signed(secret, id string, ts int64, payload string) http.Header {
	key, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	mac := hmac.New(sha256.New, key)
	fmt.Fprintf(mac, "%s.%d.%s", id, ts, payload)
	h := http.Header{}
	h.Set("webhook-id", id)
	h.Set("webhook-timestamp", strconv.FormatInt(ts, 10))
	h.Set("webhook-signature", "v1,"+base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	return h
}

func headerLines(h http.Header) string {
	var b strings.Builder
	b.WriteString("POST /webhooks/email HTTP/1.1\nContent-Type: application/json\n")
	for _, k := range []string{"webhook-id", "webhook-timestamp", "webhook-signature"} {
		fmt.Fprintf(&b, "%s: %s\n", k, h.Get(k))
	}
	return b.String()
}

func TestWebhooksVerify(t *testing.T) {
	const secret = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"
	payload := `{"type":"email.delivered","created_at":"2026-10-30T11:59:00.000Z","data":{"email_id":"e-42","from":"hi@acme.com","to":["ana@example.com"],"subject":"Hi","created_at":"2026-10-30T11:58:00.000Z","tags":{}}}`
	h := signed(secret, "msg_1", fixedNow.Unix()-30, payload)
	body := writeFile(t, "body.json", payload)
	headers := writeFile(t, "headers.txt", headerLines(h))

	e := newEnv(t)
	e.run("webhooks", "verify", "--secret", secret, "--headers-file", headers, body).expect(t, 0).
		contains(t, "✓ Signature valid", "email.delivered", "e-42", "ana@example.com")

	// JSON headers file, secret from the environment, payload on stdin.
	jsonHeaders := writeFile(t, "h.json", fmt.Sprintf(`{"webhook-id":"msg_1","webhook-timestamp":%d,"webhook-signature":%q}`,
		fixedNow.Unix()-30, h.Get("webhook-signature")))
	e.vars[EnvWebhookSecret] = secret
	e.stdin = payload
	m := e.run("webhooks", "verify", "--headers-file", jsonHeaders, "-", "--json").expect(t, 0).json(t)
	if m["valid"] != true || m["event"].(map[string]any)["data"].(map[string]any)["email_id"] != "e-42" {
		t.Fatalf("json = %v", m)
	}

	// --header flags, several signatures (rotation): one valid entry is enough.
	e.run("webhooks", "verify", body, "--header", "webhook-id: msg_1",
		"--header", "webhook-timestamp: "+h.Get("webhook-timestamp"),
		"--header", "webhook-signature: v1,AAAA "+h.Get("webhook-signature")).expect(t, 0)

	tampered := writeFile(t, "tampered.json", strings.Replace(payload, "e-42", "e-43", 1))
	e.run("webhooks", "verify", "--headers-file", headers, tampered).expect(t, ExitError).
		contains(t, "✗ Invalid: webhook signature does not match", "Expected webhook-signature: v1,")

	e.run("webhooks", "verify", "--secret", "whsec_d3Jvbmc=", "--headers-file", headers, body).expect(t, ExitError).
		contains(t, "signature does not match")

	old := signed(secret, "msg_1", fixedNow.Unix()-3600, payload)
	oldHeaders := writeFile(t, "old.txt", headerLines(old))
	e.run("webhooks", "verify", "--headers-file", oldHeaders, body).expect(t, ExitError).contains(t, "tolerance", "--ignore-timestamp")
	e.run("webhooks", "verify", "--headers-file", oldHeaders, body, "--ignore-timestamp").expect(t, 0)
	e.run("webhooks", "verify", "--headers-file", oldHeaders, body, "--tolerance", "2h").expect(t, 0)

	m = e.run("webhooks", "verify", "--header", "webhook-id: x", body, "--json").expect(t, ExitError).json(t)
	if m["valid"] != false || m["error"] != "missing webhook signature headers" {
		t.Fatalf("json = %v", m)
	}

	delete(e.vars, EnvWebhookSecret)
	e.run("webhooks", "verify", "--headers-file", headers, body).expect(t, ExitUsage).errContains(t, "signing secret")
	e.run("webhooks", "verify", "--secret", secret, "--headers-file", writeFile(t, "bad.txt", "no colon here"), body).expect(t, ExitUsage)
}

func TestParseHeaders(t *testing.T) {
	h := http.Header{}
	err := parseHeaders([]byte("# captured\nPOST /hook HTTP/1.1\n\nWebhook-Id: msg_1\nwebhook-signature: v1,abc v1,def\n"), h)
	if err != nil || h.Get("webhook-id") != "msg_1" || h.Get("webhook-signature") != "v1,abc v1,def" {
		t.Fatalf("h = %v, err = %v", h, err)
	}
	if err := parseHeaders([]byte(`{"a": true}`), http.Header{}); err == nil {
		t.Fatal("accepted a non-string JSON header")
	}
	if err := parseHeaders([]byte("\n\n"), http.Header{}); err == nil {
		t.Fatal("accepted no headers")
	}
}

func TestParseEvents(t *testing.T) {
	got, err := parseEvents([]string{"Delivered", "email.delivered", " opened "})
	if err != nil || len(got) != 2 || got[0] != "email.delivered" || got[1] != "email.opened" {
		t.Fatalf("got %v, %v", got, err)
	}
	if _, err := parseEvents([]string{"email.unknown"}); err == nil {
		t.Fatal("accepted an unknown event")
	}
}
