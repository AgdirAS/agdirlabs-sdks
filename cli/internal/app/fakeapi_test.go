package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// Test API keys understood by fakeAPI.
const (
	keyFull    = "re_full_access_key_1234"
	keySending = "re_sending_only_key_5678"
)

const dkimValue = "p=MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQC1testkeyAQAB"

// fakeAPI is an in-memory Agdir Email API for command tests.
type fakeAPI struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	requests []recordedRequest
	emails   map[string]map[string]any
	idem     map[string]string
	domains  []map[string]any
	webhooks []map[string]any
	seq      int
}

type recordedRequest struct {
	Method string
	Path   string
	Query  string
	Header http.Header
	Body   map[string]any
	Raw    []byte
}

func newFakeAPI(t *testing.T) *fakeAPI {
	f := &fakeAPI{t: t, emails: map[string]map[string]any{}, idem: map[string]string{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /emails", f.sendEmail)
	mux.HandleFunc("POST /emails/batch", f.sendBatch)
	mux.HandleFunc("GET /emails/{id}", f.getEmail)
	mux.HandleFunc("POST /emails/{id}/cancel", f.cancelEmail)
	mux.HandleFunc("POST /domains", f.createDomain)
	mux.HandleFunc("GET /domains", f.listDomains)
	mux.HandleFunc("GET /domains/{id}", f.getDomain)
	mux.HandleFunc("POST /domains/{id}/verify", f.verifyDomain)
	mux.HandleFunc("DELETE /domains/{id}", f.deleteDomain)
	mux.HandleFunc("POST /webhooks", f.createWebhook)
	mux.HandleFunc("GET /webhooks", f.listWebhooks)
	mux.HandleFunc("GET /webhooks/{id}", f.getWebhook)
	mux.HandleFunc("PATCH /webhooks/{id}", f.updateWebhook)
	mux.HandleFunc("DELETE /webhooks/{id}", f.deleteWebhook)
	mux.HandleFunc("POST /webhooks/{id}/signing-secret/rotate", f.rotateSecret)
	mux.HandleFunc("GET /webhooks/{id}/events", f.listDeliveries)
	mux.HandleFunc("GET /webhooks/{id}/events/{eid}", f.getDelivery)
	mux.HandleFunc("GET /webhooks/{id}/events/{eid}/attempts", f.listAttempts)
	mux.HandleFunc("POST /webhooks/{id}/events/{eid}/replay", f.replayDelivery)

	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		rec := recordedRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Header: r.Header.Clone(), Raw: raw}
		_ = json.Unmarshal(raw, &rec.Body)
		f.mu.Lock()
		f.requests = append(f.requests, rec)
		f.mu.Unlock()
		r.Body = io.NopCloser(strings.NewReader(string(raw)))

		w.Header().Set("ratelimit-limit", "10")
		auth := r.Header.Get("Authorization")
		switch {
		case auth == "":
			apiError(w, 401, "missing_api_key", "Missing API key in the authorization header")
			return
		case auth == "Bearer "+keySending:
			if !(r.Method == "POST" && (r.URL.Path == "/emails" || r.URL.Path == "/emails/batch")) {
				apiError(w, 401, "restricted_api_key", "This API key is restricted to only send emails")
				return
			}
		case auth != "Bearer "+keyFull:
			apiError(w, 403, "invalid_api_key", "API key is invalid")
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func apiError(w http.ResponseWriter, status int, name, msg string) {
	writeJSON(w, status, map[string]any{"statusCode": status, "name": name, "message": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (f *fakeAPI) id(prefix string) string {
	f.seq++
	return fmt.Sprintf("%s-%04d-0000-4000-8000-000000000000", prefix, f.seq)
}

// lastRequest returns the last request to method+path.
func (f *fakeAPI) lastRequest(method, path string) *recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.requests) - 1; i >= 0; i-- {
		if f.requests[i].Method == method && f.requests[i].Path == path {
			r := f.requests[i]
			return &r
		}
	}
	f.t.Fatalf("no %s %s request; got %v", method, path, f.requests)
	return nil
}

func (f *fakeAPI) count(method, path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.requests {
		if r.Method == method && r.Path == path {
			n++
		}
	}
	return n
}

func decode(r *http.Request) map[string]any {
	var m map[string]any
	_ = json.NewDecoder(r.Body).Decode(&m)
	return m
}

func validateEmail(m map[string]any) string {
	if m["from"] == nil || m["from"] == "" {
		return "The `from` field is required."
	}
	to, _ := m["to"].([]any)
	if len(to) == 0 {
		return "The `to` field is required."
	}
	for _, x := range to {
		if s, _ := x.(string); !strings.Contains(s, "@") {
			return fmt.Sprintf("Invalid `to` field: %q is not a valid email address.", s)
		}
	}
	if m["html"] == nil && m["text"] == nil {
		return "Either `html` or `text` is required."
	}
	return ""
}

func (f *fakeAPI) storeEmail(m map[string]any) string {
	id := f.id("e")
	status := "queued"
	if m["scheduled_at"] != nil {
		status = "scheduled"
	}
	f.emails[id] = map[string]any{
		"object": "email", "id": id, "from": m["from"], "to": m["to"], "cc": m["cc"], "bcc": m["bcc"],
		"reply_to": m["reply_to"], "subject": m["subject"], "html": m["html"], "text": m["text"],
		"created_at": "2026-10-30T10:00:00.000Z", "scheduled_at": m["scheduled_at"], "last_event": status,
	}
	return id
}

func (f *fakeAPI) sendEmail(w http.ResponseWriter, r *http.Request) {
	m := decode(r)
	if key := r.Header.Get("Idempotency-Key"); key != "" {
		if id, ok := f.idem[key]; ok {
			writeJSON(w, 200, map[string]any{"id": id})
			return
		}
	}
	if msg := validateEmail(m); msg != "" {
		apiError(w, 422, "validation_error", msg)
		return
	}
	id := f.storeEmail(m)
	if key := r.Header.Get("Idempotency-Key"); key != "" {
		f.idem[key] = id
	}
	writeJSON(w, 200, map[string]any{"id": id})
}

func (f *fakeAPI) sendBatch(w http.ResponseWriter, r *http.Request) {
	var items []map[string]any
	_ = json.NewDecoder(r.Body).Decode(&items)
	permissive := r.Header.Get("x-batch-validation") == "permissive"
	data := []map[string]any{}
	errs := []map[string]any{}
	if !permissive {
		for i, m := range items {
			if msg := validateEmail(m); msg != "" {
				apiError(w, 422, "validation_error", fmt.Sprintf("[%d] %s", i, msg))
				return
			}
		}
	}
	for i, m := range items {
		if msg := validateEmail(m); msg != "" {
			if !permissive {
				apiError(w, 422, "validation_error", fmt.Sprintf("[%d] %s", i, msg))
				return
			}
			errs = append(errs, map[string]any{"index": i, "message": msg})
			continue
		}
		data = append(data, map[string]any{"id": f.storeEmail(m)})
	}
	out := map[string]any{"data": data}
	if permissive {
		out["errors"] = errs
	}
	writeJSON(w, 200, out)
}

func (f *fakeAPI) getEmail(w http.ResponseWriter, r *http.Request) {
	e, ok := f.emails[r.PathValue("id")]
	if !ok {
		apiError(w, 404, "not_found", "Email not found")
		return
	}
	writeJSON(w, 200, e)
}

func (f *fakeAPI) cancelEmail(w http.ResponseWriter, r *http.Request) {
	e, ok := f.emails[r.PathValue("id")]
	if !ok {
		apiError(w, 404, "not_found", "Email not found")
		return
	}
	if e["last_event"] != "scheduled" {
		apiError(w, 422, "validation_error", "Only scheduled emails can be canceled")
		return
	}
	e["last_event"] = "canceled"
	writeJSON(w, 200, map[string]any{"object": "email", "id": e["id"]})
}

// domainRecords mirrors the API: names relative to the registrable domain.
func domainRecords(name string) []map[string]any {
	sub := ""
	if parts := strings.Split(name, "."); len(parts) > 2 {
		sub = "." + strings.Join(parts[:len(parts)-2], ".")
	}
	return []map[string]any{
		{"record": "DKIM", "name": "agdir._domainkey" + sub, "type": "TXT", "ttl": "Auto", "status": "not_started", "value": dkimValue},
		{"record": "SPF", "name": "send" + sub, "type": "MX", "ttl": "Auto", "status": "not_started", "value": "feedback-smtp.eu-north-1.amazonses.com", "priority": 10},
		{"record": "SPF", "name": "send" + sub, "type": "TXT", "ttl": "Auto", "status": "not_started", "value": "v=spf1 include:amazonses.com ~all"},
		{"record": "DMARC", "name": "_dmarc" + sub, "type": "TXT", "ttl": "Auto", "status": "not_started", "value": "v=DMARC1; p=none;"},
	}
}

func (f *fakeAPI) addDomain(name string) map[string]any {
	d := map[string]any{
		"object": "domain", "id": f.id("d"), "name": name, "status": "not_started",
		"created_at": "2026-10-30T10:00:00.000Z", "region": "eu-north-1",
		"capabilities":  map[string]any{"sending": "enabled", "receiving": "disabled"},
		"open_tracking": false, "click_tracking": false, "records": domainRecords(name),
	}
	f.domains = append(f.domains, d)
	return d
}

func (f *fakeAPI) createDomain(w http.ResponseWriter, r *http.Request) {
	m := decode(r)
	name, _ := m["name"].(string)
	if !strings.Contains(name, ".") {
		apiError(w, 422, "validation_error", "Invalid domain name")
		return
	}
	writeJSON(w, 200, f.addDomain(name))
}

func (f *fakeAPI) findDomain(id string) (int, map[string]any) {
	for i, d := range f.domains {
		if d["id"] == id {
			return i, d
		}
	}
	return -1, nil
}

// page applies limit/after to items (ordered as stored).
func page(r *http.Request, items []map[string]any, strip ...string) map[string]any {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 20
	}
	start := 0
	if after := r.URL.Query().Get("after"); after != "" {
		for i, it := range items {
			if it["id"] == after {
				start = i + 1
			}
		}
	}
	end := min(start+limit, len(items))
	data := []map[string]any{}
	for _, it := range items[start:end] {
		c := map[string]any{}
		for k, v := range it {
			c[k] = v
		}
		for _, s := range strip {
			delete(c, s)
		}
		data = append(data, c)
	}
	return map[string]any{"object": "list", "has_more": end < len(items), "data": data}
}

func (f *fakeAPI) listDomains(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, page(r, f.domains, "records"))
}

func (f *fakeAPI) getDomain(w http.ResponseWriter, r *http.Request) {
	if _, d := f.findDomain(r.PathValue("id")); d != nil {
		writeJSON(w, 200, d)
		return
	}
	apiError(w, 404, "not_found", "Domain not found")
}

func (f *fakeAPI) verifyDomain(w http.ResponseWriter, r *http.Request) {
	if _, d := f.findDomain(r.PathValue("id")); d != nil {
		d["status"] = "pending"
		writeJSON(w, 200, map[string]any{"object": "domain", "id": d["id"]})
		return
	}
	apiError(w, 404, "not_found", "Domain not found")
}

func (f *fakeAPI) deleteDomain(w http.ResponseWriter, r *http.Request) {
	if i, d := f.findDomain(r.PathValue("id")); d != nil {
		f.domains = append(f.domains[:i], f.domains[i+1:]...)
		writeJSON(w, 200, map[string]any{"object": "domain", "id": d["id"], "deleted": true})
		return
	}
	apiError(w, 404, "not_found", "Domain not found")
}

func (f *fakeAPI) createWebhook(w http.ResponseWriter, r *http.Request) {
	m := decode(r)
	endpoint, _ := m["endpoint"].(string)
	if !strings.HasPrefix(endpoint, "https://") {
		apiError(w, 422, "validation_error", "endpoint must be an https URL")
		return
	}
	id := f.id("w")
	secret := "whsec_" + "c2VjcmV0LWZvci0" + strconv.Itoa(f.seq)
	f.webhooks = append(f.webhooks, map[string]any{
		"object": "webhook", "id": id, "endpoint": endpoint, "events": m["events"], "status": "enabled",
		"created_at": "2026-10-30T10:00:00.000Z", "signing_secret": secret,
		"deliveries": []map[string]any{{
			"object": "webhook_event", "id": "evt-1", "type": "email.delivered", "status": "success",
			"created_at": "2026-10-30T10:01:00.000Z", "next_attempt_at": nil,
			"payload": map[string]any{"type": "email.delivered", "data": map[string]any{"email_id": "e-1"}},
		}},
	})
	writeJSON(w, 200, map[string]any{"object": "webhook", "id": id, "signing_secret": secret})
}

func (f *fakeAPI) findWebhook(id string) (int, map[string]any) {
	for i, h := range f.webhooks {
		if h["id"] == id {
			return i, h
		}
	}
	return -1, nil
}

func (f *fakeAPI) listWebhooks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, page(r, f.webhooks, "signing_secret", "deliveries"))
}

func (f *fakeAPI) withWebhook(w http.ResponseWriter, r *http.Request, fn func(i int, h map[string]any)) {
	i, h := f.findWebhook(r.PathValue("id"))
	if h == nil {
		apiError(w, 404, "not_found", "Webhook not found")
		return
	}
	fn(i, h)
}

func (f *fakeAPI) getWebhook(w http.ResponseWriter, r *http.Request) {
	f.withWebhook(w, r, func(_ int, h map[string]any) {
		c := map[string]any{}
		for k, v := range h {
			if k != "deliveries" {
				c[k] = v
			}
		}
		writeJSON(w, 200, c)
	})
}

func (f *fakeAPI) updateWebhook(w http.ResponseWriter, r *http.Request) {
	f.withWebhook(w, r, func(_ int, h map[string]any) {
		for k, v := range decode(r) {
			h[k] = v
		}
		writeJSON(w, 200, map[string]any{"object": "webhook", "id": h["id"]})
	})
}

func (f *fakeAPI) deleteWebhook(w http.ResponseWriter, r *http.Request) {
	f.withWebhook(w, r, func(i int, h map[string]any) {
		f.webhooks = append(f.webhooks[:i], f.webhooks[i+1:]...)
		writeJSON(w, 200, map[string]any{"object": "webhook", "id": h["id"], "deleted": true})
	})
}

func (f *fakeAPI) rotateSecret(w http.ResponseWriter, r *http.Request) {
	f.withWebhook(w, r, func(_ int, h map[string]any) {
		h["signing_secret"] = "whsec_cm90YXRlZA=="
		writeJSON(w, 200, map[string]any{"object": "webhook", "id": h["id"], "signing_secret": h["signing_secret"]})
	})
}

func (f *fakeAPI) listDeliveries(w http.ResponseWriter, r *http.Request) {
	f.withWebhook(w, r, func(_ int, h map[string]any) {
		writeJSON(w, 200, page(r, h["deliveries"].([]map[string]any), "payload", "next_attempt_at"))
	})
}

func (f *fakeAPI) withDelivery(w http.ResponseWriter, r *http.Request, fn func(d map[string]any)) {
	f.withWebhook(w, r, func(_ int, h map[string]any) {
		for _, d := range h["deliveries"].([]map[string]any) {
			if d["id"] == r.PathValue("eid") {
				fn(d)
				return
			}
		}
		apiError(w, 404, "not_found", "Webhook event not found")
	})
}

func (f *fakeAPI) getDelivery(w http.ResponseWriter, r *http.Request) {
	f.withDelivery(w, r, func(d map[string]any) { writeJSON(w, 200, d) })
}

func (f *fakeAPI) listAttempts(w http.ResponseWriter, r *http.Request) {
	f.withDelivery(w, r, func(map[string]any) {
		writeJSON(w, 200, page(r, []map[string]any{
			{"object": "webhook_delivery", "id": "att-1", "http_status_code": 500, "response": "upstream error", "sent_at": "2026-10-30T10:01:00.000Z"},
			{"object": "webhook_delivery", "id": "att-2", "http_status_code": 200, "response": "ok", "sent_at": "2026-10-30T10:01:05.000Z"},
		}))
	})
}

func (f *fakeAPI) replayDelivery(w http.ResponseWriter, r *http.Request) {
	f.withDelivery(w, r, func(d map[string]any) {
		writeJSON(w, 200, map[string]any{"object": "webhook_event", "id": d["id"]})
	})
}

// domainNames lists the stored domain names, sorted.
func (f *fakeAPI) domainNames() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, d := range f.domains {
		out = append(out, d["name"].(string))
	}
	sort.Strings(out)
	return out
}
