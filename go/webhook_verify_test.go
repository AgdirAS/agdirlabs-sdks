package agdiremail

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

const samplePayload = `{"type":"email.bounced","created_at":"2026-10-30T09:50:00.000Z","data":{"created_at":"2026-10-30T09:44:50.892Z","email_id":"e1","message_id":"<m1@eu-north-1.amazonses.com>","from":"Acme <hi@acme.com>","to":["b@x.com"],"subject":"Hi","tags":{"kind":"welcome"},"bounce":{"message":"Mailbox does not exist","subType":"General","type":"Permanent"}}}`

func newSecret(t *testing.T) string {
	t.Helper()
	key := make([]byte, 24)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return "whsec_" + base64.StdEncoding.EncodeToString(key)
}

func sign(t *testing.T, secret, id string, ts int64, payload string) string {
	t.Helper()
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + strconv.FormatInt(ts, 10) + "." + payload))
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func signedHeaders(t *testing.T, secret string, ts int64, payload string) http.Header {
	h := http.Header{}
	h.Set("webhook-id", "msg_1")
	h.Set("webhook-timestamp", strconv.FormatInt(ts, 10))
	h.Set("webhook-signature", sign(t, secret, "msg_1", ts, payload))
	return h
}

func wantVerificationError(t *testing.T, err error, reason string) {
	t.Helper()
	var vErr *WebhookVerificationError
	if !errors.As(err, &vErr) || !strings.Contains(vErr.Reason, reason) {
		t.Fatalf("err = %v, want WebhookVerificationError containing %q", err, reason)
	}
}

func TestVerifyWebhook(t *testing.T) {
	secret := newSecret(t)
	now := time.Now().Unix()

	t.Run("valid", func(t *testing.T) {
		e, err := VerifyWebhook([]byte(samplePayload), signedHeaders(t, secret, now, samplePayload), secret)
		if err != nil {
			t.Fatal(err)
		}
		d := e.Data
		if e.Type != EventEmailBounced || d.EmailID != "e1" || d.MessageID != "<m1@eu-north-1.amazonses.com>" || d.To[0] != "b@x.com" ||
			d.Tags["kind"] != "welcome" || d.Bounce == nil || d.Bounce.SubType != "General" || d.Bounce.Type != "Permanent" ||
			d.Click != nil || d.Failed != nil || e.CreatedAt.IsZero() || d.CreatedAt.IsZero() {
			t.Fatalf("%+v", e)
		}
	})
	t.Run("click and failed events", func(t *testing.T) {
		p := `{"type":"email.clicked","created_at":"2026-10-30T09:50:00.000Z","data":{"created_at":"2026-10-30T09:44:50.892Z","email_id":"e1","from":"a@acme.com","to":["b@x.com"],"subject":"Hi","click":{"ipAddress":"1.2.3.4","link":"https://acme.com","timestamp":"2026-10-30T09:49:00.000Z","userAgent":"UA"},"failed":{"reason":"suppressed"}}}`
		e, err := VerifyWebhook([]byte(p), signedHeaders(t, secret, now, p), secret)
		if err != nil {
			t.Fatal(err)
		}
		if e.Data.Click == nil || e.Data.Click.IPAddress != "1.2.3.4" || e.Data.Click.Timestamp.IsZero() || e.Data.Failed.Reason != "suppressed" || e.Data.Tags == nil {
			t.Fatalf("%+v", e.Data)
		}
	})
	t.Run("secret without prefix", func(t *testing.T) {
		if _, err := VerifyWebhook([]byte(samplePayload), signedHeaders(t, secret, now, samplePayload), strings.TrimPrefix(secret, "whsec_")); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("tampered body", func(t *testing.T) {
		h := signedHeaders(t, secret, now, samplePayload)
		_, err := VerifyWebhook([]byte(strings.Replace(samplePayload, "e1", "e2", 1)), h, secret)
		wantVerificationError(t, err, "signature does not match")
	})
	t.Run("wrong secret", func(t *testing.T) {
		_, err := VerifyWebhook([]byte(samplePayload), signedHeaders(t, secret, now, samplePayload), newSecret(t))
		wantVerificationError(t, err, "signature does not match")
	})
	t.Run("missing headers", func(t *testing.T) {
		for _, name := range []string{"webhook-id", "webhook-timestamp", "webhook-signature"} {
			h := signedHeaders(t, secret, now, samplePayload)
			h.Del(name)
			_, err := VerifyWebhook([]byte(samplePayload), h, secret)
			wantVerificationError(t, err, "missing")
		}
	})
	t.Run("stale and future timestamps", func(t *testing.T) {
		for _, ts := range []int64{now - 301, now + 301} {
			_, err := VerifyWebhook([]byte(samplePayload), signedHeaders(t, secret, ts, samplePayload), secret)
			wantVerificationError(t, err, "tolerance")
		}
		h := signedHeaders(t, secret, now, samplePayload)
		h.Set("webhook-timestamp", "soon")
		_, err := VerifyWebhook([]byte(samplePayload), h, secret)
		wantVerificationError(t, err, "tolerance")
	})
	t.Run("custom tolerance and clock", func(t *testing.T) {
		old := now - 3600
		h := signedHeaders(t, secret, old, samplePayload)
		if _, err := VerifyWebhook([]byte(samplePayload), h, secret, WithTolerance(2*time.Hour)); err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyWebhook([]byte(samplePayload), h, secret, WithVerifyTime(time.Unix(old+10, 0))); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("multi-signature header", func(t *testing.T) {
		h := signedHeaders(t, secret, now, samplePayload)
		other := sign(t, newSecret(t), "msg_1", now, samplePayload)
		h.Set("webhook-signature", "v2,abc "+other+" "+h.Get("webhook-signature"))
		if _, err := VerifyWebhook([]byte(samplePayload), h, secret); err != nil {
			t.Fatal(err)
		}
		h.Set("webhook-signature", other)
		_, err := VerifyWebhook([]byte(samplePayload), h, secret)
		wantVerificationError(t, err, "signature does not match")
	})
	t.Run("invalid secret", func(t *testing.T) {
		_, err := VerifyWebhook([]byte(samplePayload), signedHeaders(t, secret, now, samplePayload), "whsec_!!!")
		wantVerificationError(t, err, "base64")
	})
	t.Run("invalid JSON", func(t *testing.T) {
		_, err := VerifyWebhook([]byte("not json"), signedHeaders(t, secret, now, "not json"), secret)
		wantVerificationError(t, err, "JSON")
	})
}

func TestWebhookHandler(t *testing.T) {
	secret := newSecret(t)
	var got *WebhookEvent
	var fail error
	h := WebhookHandler(secret, func(_ context.Context, e *WebhookEvent) error { got = e; return fail })

	do := func(method string, header http.Header, body string) int {
		r := httptest.NewRequest(method, "/hook", strings.NewReader(body))
		for k, v := range header {
			r.Header[k] = v
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	now := time.Now().Unix()
	if code := do("POST", signedHeaders(t, secret, now, samplePayload), samplePayload); code != 204 || got == nil || got.Data.EmailID != "e1" {
		t.Fatalf("valid: %d %+v", code, got)
	}
	if code := do("POST", signedHeaders(t, secret, now, samplePayload), samplePayload+" "); code != 400 {
		t.Fatalf("tampered: %d", code)
	}
	if code := do("GET", nil, ""); code != 405 {
		t.Fatalf("GET: %d", code)
	}
	fail = errors.New("db down")
	if code := do("POST", signedHeaders(t, secret, now, samplePayload), samplePayload); code != 500 {
		t.Fatalf("handler error: %d", code)
	}
}
