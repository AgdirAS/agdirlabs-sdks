package agdiremail

import (
	"context"
	"errors"
	"net"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestNewClientConfig(t *testing.T) {
	t.Run("missing key", func(t *testing.T) {
		t.Setenv(EnvAPIKey, "")
		_, err := NewClient()
		if !errors.Is(err, ErrMissingAPIKey) {
			t.Fatalf("err = %v, want ErrMissingAPIKey", err)
		}
		for _, want := range []string{"AGDIR_EMAIL_API_KEY", "https://app.agdir.email"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not mention %s", err, want)
			}
		}
	})
	t.Run("env key and default base URL", func(t *testing.T) {
		t.Setenv(EnvAPIKey, "re_env")
		t.Setenv(EnvBaseURL, "")
		c, err := NewClient()
		if err != nil {
			t.Fatal(err)
		}
		if c.http.apiKey != "re_env" || c.BaseURL() != DefaultBaseURL {
			t.Fatalf("key %q base %q", c.http.apiKey, c.BaseURL())
		}
	})
	t.Run("option beats env", func(t *testing.T) {
		t.Setenv(EnvAPIKey, "re_env")
		t.Setenv(EnvBaseURL, "https://env.example.com")
		c, err := NewClient(WithAPIKey("re_opt"), WithBaseURL("https://opt.example.com/api/"))
		if err != nil {
			t.Fatal(err)
		}
		if c.http.apiKey != "re_opt" || c.BaseURL() != "https://opt.example.com/api" {
			t.Fatalf("key %q base %q", c.http.apiKey, c.BaseURL())
		}
	})
	t.Run("env base URL", func(t *testing.T) {
		t.Setenv(EnvBaseURL, "https://env.example.com")
		c, _ := NewClient(WithAPIKey("re_x"))
		if c.BaseURL() != "https://env.example.com" {
			t.Fatal(c.BaseURL())
		}
	})
	t.Run("invalid options", func(t *testing.T) {
		if _, err := NewClient(WithAPIKey("re_x"), WithMaxRetries(-1)); err == nil {
			t.Error("negative retries accepted")
		}
		if _, err := NewClient(WithAPIKey("re_x"), WithTimeout(0)); err == nil {
			t.Error("zero timeout accepted")
		}
	})
}

func TestRequestHeaders(t *testing.T) {
	f, c := newFake(t, reply{Body: M{"id": "e1"}})
	if _, err := c.Emails.Send(context.Background(), &SendEmailRequest{From: "a@x.com", To: []string{"b@y.com"}, Subject: "s", Text: "t"}); err != nil {
		t.Fatal(err)
	}
	r := f.req(0)
	checks := map[string]string{
		"Authorization": "Bearer re_test",
		"User-Agent":    "agdirlabs-email-go/" + Version,
		"Accept":        "application/json",
		"Content-Type":  "application/json",
	}
	for k, want := range checks {
		if got := r.Header.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if r.Path != "/emails" || r.Method != "POST" {
		t.Errorf("%s %s", r.Method, r.Path)
	}
}

func TestVersionConstant(t *testing.T) {
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(Version) || UserAgent != "agdirlabs-email-go/"+Version {
		t.Fatalf("Version %q UserAgent %q", Version, UserAgent)
	}
}

func TestAPIErrors(t *testing.T) {
	t.Run("JSON body", func(t *testing.T) {
		_, c := newFake(t, apiError(422, "validation_error", "Invalid `to` field."))
		_, err := c.Emails.Get(context.Background(), "x")
		var apiErr *Error
		if !errors.As(err, &apiErr) {
			t.Fatalf("err = %T %v", err, err)
		}
		if apiErr.Code != CodeValidationError || apiErr.Status != 422 || apiErr.Message != "Invalid `to` field." {
			t.Fatalf("%+v", apiErr)
		}
		if !strings.Contains(err.Error(), "validation_error") {
			t.Errorf("Error() = %q", err)
		}
	})
	t.Run("non-JSON body", func(t *testing.T) {
		_, c := newFake(t, reply{Status: 418, Raw: "<html>teapot</html>"})
		_, err := c.Domains.Get(context.Background(), "x")
		var apiErr *Error
		errors.As(err, &apiErr)
		if apiErr == nil || apiErr.Message != "Request failed with HTTP 418" || apiErr.Code != CodeUnknownError || apiErr.Status != 418 {
			t.Fatalf("%+v", apiErr)
		}
	})
	t.Run("non-JSON 5xx", func(t *testing.T) {
		f, c := newFake(t, reply{Status: 502, Raw: "bad gateway"})
		_, err := c.Domains.Create(context.Background(), &CreateDomainRequest{Name: "a.com"})
		var apiErr *Error
		errors.As(err, &apiErr)
		if apiErr == nil || apiErr.Code != CodeInternalServerError || apiErr.Message != "Request failed with HTTP 502" {
			t.Fatalf("%+v", apiErr)
		}
		if f.count() != 1 {
			t.Errorf("non-idempotent POST retried: %d requests", f.count())
		}
	})
}

func TestRetries(t *testing.T) {
	ctx := context.Background()

	t.Run("rate limit always retried, honouring retry-after", func(t *testing.T) {
		rl := apiError(429, "rate_limit_exceeded", "Too many requests")
		rl.Header = map[string]string{"Retry-After": "2"}
		f, c := newFake(t, rl, reply{Body: M{"object": "domain", "id": "d1", "name": "a.com", "status": "not_started", "created_at": "2026-10-30T09:00:00.000Z", "records": []any{}}})
		d, err := c.Domains.Create(ctx, &CreateDomainRequest{Name: "a.com"})
		if err != nil || d.ID != "d1" {
			t.Fatal(d, err)
		}
		if f.count() != 2 || len(f.sleeps) != 1 || f.sleeps[0] != 2*time.Second {
			t.Fatalf("requests %d sleeps %v", f.count(), f.sleeps)
		}
	})
	t.Run("rate limit gives up after maxRetries", func(t *testing.T) {
		rl := apiError(429, "rate_limit_exceeded", "slow down")
		rl.Header = map[string]string{"Retry-After": "1"}
		f, c := newFake(t, rl, rl, rl)
		_, err := c.Domains.Get(ctx, "d1")
		var apiErr *Error
		if !errors.As(err, &apiErr) || apiErr.Code != CodeRateLimitExceeded || apiErr.RetryAfter != time.Second {
			t.Fatalf("%v", err)
		}
		if f.count() != 3 {
			t.Fatalf("requests %d", f.count())
		}
	})
	t.Run("quota errors are not retried", func(t *testing.T) {
		for _, code := range []string{"daily_quota_exceeded", "monthly_quota_exceeded"} {
			f, c := newFake(t, apiError(429, code, "quota"))
			_, err := c.Emails.Send(ctx, &SendEmailRequest{From: "a@x.com", To: []string{"b@y.com"}, Subject: "s", Text: "t"})
			var apiErr *Error
			if !errors.As(err, &apiErr) || string(apiErr.Code) != code || f.count() != 1 {
				t.Fatalf("%s: %v after %d requests", code, err, f.count())
			}
		}
	})
	t.Run("server errors retried for GET with backoff", func(t *testing.T) {
		f, c := newFake(t, apiError(503, "internal_server_error", "x"), apiError(500, "internal_server_error", "x"),
			reply{Body: M{"object": "list", "has_more": false, "data": []any{}}})
		if _, err := c.Domains.List(ctx, nil); err != nil {
			t.Fatal(err)
		}
		if f.count() != 3 || len(f.sleeps) != 2 {
			t.Fatalf("requests %d sleeps %v", f.count(), f.sleeps)
		}
		if f.sleeps[0] < 250*time.Millisecond || f.sleeps[0] > 500*time.Millisecond || f.sleeps[1] < 500*time.Millisecond || f.sleeps[1] > time.Second {
			t.Errorf("backoff %v", f.sleeps)
		}
	})
	t.Run("4xx not retried", func(t *testing.T) {
		f, c := newFake(t, apiError(404, "not_found", "Email not found"))
		_, err := c.Emails.Get(ctx, "nope")
		var apiErr *Error
		if !errors.As(err, &apiErr) || apiErr.Code != CodeNotFound || f.count() != 1 {
			t.Fatal(err, f.count())
		}
	})
	t.Run("maxRetries 0 disables retries", func(t *testing.T) {
		f, c := newFake(t, apiError(429, "rate_limit_exceeded", "x"))
		c.http.maxRetries = 0
		if _, err := c.Domains.Get(ctx, "d"); err == nil || f.count() != 1 {
			t.Fatal(err, f.count())
		}
	})
	t.Run("non-idempotent operations not retried on 500", func(t *testing.T) {
		f, c := newFake(t, apiError(500, "internal_server_error", "x"), apiError(500, "internal_server_error", "x"),
			apiError(500, "internal_server_error", "x"), apiError(500, "internal_server_error", "x"))
		_, _ = c.Webhooks.Create(ctx, &CreateWebhookRequest{Endpoint: "https://x", Events: []WebhookEventType{EventEmailSent}})
		_, _ = c.Webhooks.Delete(ctx, "w")
		_, _ = c.Webhooks.RotateSecret(ctx, "w")
		_, _ = c.Webhooks.Deliveries.Replay(ctx, "w", "e")
		if f.count() != 4 {
			t.Fatalf("requests %d", f.count())
		}
	})
	t.Run("idempotent operations retried on 500", func(t *testing.T) {
		e500 := apiError(500, "internal_server_error", "x")
		f, c := newFake(t, e500, reply{Body: M{"id": "w"}}, e500, reply{Body: M{"id": "d"}}, e500, reply{Body: M{"id": "e"}})
		if _, err := c.Webhooks.Update(ctx, "w", &UpdateWebhookRequest{Status: WebhookDisabled}); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Domains.Verify(ctx, "d"); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Emails.Cancel(ctx, "e"); err != nil {
			t.Fatal(err)
		}
		if f.count() != 6 {
			t.Fatalf("requests %d", f.count())
		}
	})
}

func TestNetworkErrorsAndTimeouts(t *testing.T) {
	ctx := context.Background()

	// A listener that accepts and immediately closes connections.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	c, _ := NewClient(WithAPIKey("re_x"), WithBaseURL("http://"+ln.Addr().String()))
	var sleeps int
	c.http.sleep = func(context.Context, time.Duration) error { sleeps++; return nil }

	_, err = c.Domains.Get(ctx, "d")
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Code != CodeNetworkError || apiErr.Status != 0 || apiErr.Err == nil {
		t.Fatalf("%v", err)
	}
	if sleeps != 2 {
		t.Errorf("GET network error retried %d times, want 2", sleeps)
	}

	sleeps = 0
	_, err = c.Domains.Create(ctx, &CreateDomainRequest{Name: "a.com"})
	if !errors.As(err, &apiErr) || apiErr.Code != CodeNetworkError || sleeps != 0 {
		t.Fatalf("%v, sleeps %d", err, sleeps)
	}

	t.Run("timeout", func(t *testing.T) {
		f, c := newFake(t, reply{Delay: time.Second}, reply{Delay: time.Second})
		c.http.timeout = 50 * time.Millisecond
		c.http.maxRetries = 1
		_, err := c.Domains.Get(ctx, "d")
		var apiErr *Error
		if !errors.As(err, &apiErr) || apiErr.Code != CodeTimeout || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("%v", err)
		}
		if f.count() != 2 {
			t.Errorf("requests %d", f.count())
		}
	})

	t.Run("caller cancellation returns the context error", func(t *testing.T) {
		_, c := newFake(t, reply{Delay: time.Second})
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_, err := c.Domains.Get(ctx, "d")
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("%v", err)
		}
		var apiErr *Error
		if errors.As(err, &apiErr) {
			t.Fatalf("got *Error %v, want the context error", apiErr)
		}
	})
}

func TestParseRetryAfter(t *testing.T) {
	if parseRetryAfter("") != 0 || parseRetryAfter("1.5") != 1500*time.Millisecond || parseRetryAfter("junk") != 0 {
		t.Fatal("seconds")
	}
	d := parseRetryAfter(time.Now().Add(10 * time.Second).UTC().Format(http.TimeFormat))
	if d < 8*time.Second || d > 10*time.Second {
		t.Fatalf("date: %v", d)
	}
}

func TestBackoffBounds(t *testing.T) {
	for attempt := 0; attempt < 10; attempt++ {
		for i := 0; i < 50; i++ {
			d := backoff(attempt)
			if d > 8*time.Second || d < 250*time.Millisecond {
				t.Fatalf("attempt %d: %v", attempt, d)
			}
		}
	}
}

func TestIdempotencyKey(t *testing.T) {
	uuid := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if k := newIdempotencyKey(); !uuid.MatchString(k) {
		t.Fatalf("not a UUID v4: %s", k)
	}
}
