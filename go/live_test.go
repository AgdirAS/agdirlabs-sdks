package agdiremail_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	agdiremail "github.com/AgdirAS/agdirlabs-sdks/go"
)

// TestLive runs against a real Agdir Email API. It is skipped unless
// AGDIR_EMAIL_LIVE_BASE_URL and AGDIR_EMAIL_LIVE_API_KEY are set (and AGDIR_EMAIL_LIVE_FROM,
// an address on a verified domain, for the email steps). Never point it at production.
func TestLive(t *testing.T) {
	baseURL, apiKey := os.Getenv("AGDIR_EMAIL_LIVE_BASE_URL"), os.Getenv("AGDIR_EMAIL_LIVE_API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("set AGDIR_EMAIL_LIVE_BASE_URL and AGDIR_EMAIL_LIVE_API_KEY to run live tests")
	}
	from := os.Getenv("AGDIR_EMAIL_LIVE_FROM")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Generous retries: the default rate limit is 10 requests/second per team.
	client, err := agdiremail.NewClient(agdiremail.WithAPIKey(apiKey), agdiremail.WithBaseURL(baseURL), agdiremail.WithMaxRetries(6))
	if err != nil {
		t.Fatal(err)
	}
	wantCode := func(t *testing.T, err error, code agdiremail.ErrorCode) {
		t.Helper()
		var apiErr *agdiremail.Error
		if !errors.As(err, &apiErr) || apiErr.Code != code {
			t.Fatalf("err = %v, want %s", err, code)
		}
	}

	t.Run("emails", func(t *testing.T) {
		if from == "" {
			t.Skip("set AGDIR_EMAIL_LIVE_FROM")
		}
		email := &agdiremail.SendEmailRequest{
			From:        "Go SDK <" + from + ">",
			To:          []string{"delivered@example.com"},
			Subject:     "Go SDK live test",
			Text:        "Hello from the Go SDK",
			Tags:        []agdiremail.Tag{{Name: "sdk", Value: "go"}},
			ScheduledAt: time.Now().Add(time.Hour),
		}
		sent, err := client.Emails.Send(ctx, email)
		if err != nil {
			t.Fatal(err)
		}
		got, err := client.Emails.Get(ctx, sent.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != agdiremail.EmailScheduled || got.ScheduledAt == nil || got.Cc == nil || got.To[0] != "delivered@example.com" {
			t.Fatalf("%+v", got)
		}
		if _, err := client.Emails.Cancel(ctx, sent.ID); err != nil {
			t.Fatal(err)
		}
		if got, err = client.Emails.Get(ctx, sent.ID); err != nil || got.Status != agdiremail.EmailCanceled {
			t.Fatalf("after cancel: %+v %v", got, err)
		}

		// The same idempotency key and body return the original email.
		key := fmt.Sprintf("go-live-%d", time.Now().UnixNano())
		email.ScheduledAt = time.Time{}
		first, err := client.Emails.Send(ctx, email, agdiremail.WithIdempotencyKey(key))
		if err != nil {
			t.Fatal(err)
		}
		again, err := client.Emails.Send(ctx, email, agdiremail.WithIdempotencyKey(key))
		if err != nil || again.ID != first.ID {
			t.Fatalf("replay: %v %v (first %s)", again, err, first.ID)
		}

		batch, err := client.Emails.SendBatch(ctx, []agdiremail.SendEmailRequest{
			{From: from, To: []string{"delivered@example.com"}, Subject: "batch 1", Text: "one"},
			{From: from, To: []string{"not-an-address"}, Subject: "batch 2", Text: "two"},
		}, agdiremail.WithBatchValidation(agdiremail.BatchPermissive))
		if err != nil {
			t.Fatal(err)
		}
		if len(batch.Data) != 1 || len(batch.Errors) != 1 || batch.Errors[0].Index != 1 {
			t.Fatalf("%+v", batch)
		}

		_, err = client.Emails.Send(ctx, &agdiremail.SendEmailRequest{From: from, To: []string{"nope"}, Subject: "x", Text: "x"})
		wantCode(t, err, agdiremail.CodeValidationError)
	})

	t.Run("invalid key", func(t *testing.T) {
		bad, err := agdiremail.NewClient(agdiremail.WithAPIKey("re_invalid"), agdiremail.WithBaseURL(baseURL))
		if err != nil {
			t.Fatal(err)
		}
		_, err = bad.Domains.List(ctx, nil)
		wantCode(t, err, agdiremail.CodeInvalidAPIKey)
	})

	t.Run("domains", func(t *testing.T) {
		name := fmt.Sprintf("go-%d.test", time.Now().UnixNano())
		d, err := client.Domains.Create(ctx, &agdiremail.CreateDomainRequest{Name: name})
		if err != nil {
			t.Fatal(err)
		}
		deleted := false
		t.Cleanup(func() {
			if !deleted {
				_, _ = client.Domains.Delete(context.Background(), d.ID)
			}
		})
		if d.Name != name || len(d.Records) == 0 || d.Records[0].Purpose == "" {
			t.Fatalf("%+v", d)
		}
		got, err := client.Domains.Get(ctx, d.ID)
		if err != nil || got.ID != d.ID || len(got.Records) == 0 {
			t.Fatalf("%+v %v", got, err)
		}
		if _, err := client.Domains.Verify(ctx, d.ID); err != nil {
			t.Fatal(err)
		}
		found, pages := false, 0
		for item, err := range client.Domains.ListAll(ctx, &agdiremail.ListParams{Limit: 1}) {
			if err != nil {
				t.Fatal(err)
			}
			pages++
			found = found || item.ID == d.ID
		}
		if !found || pages < 2 {
			t.Fatalf("ListAll: found %v after %d items", found, pages)
		}
		if res, err := client.Domains.Delete(ctx, d.ID); err != nil || !res.Deleted {
			t.Fatal(res, err)
		}
		deleted = true
		_, err = client.Domains.Get(ctx, d.ID)
		wantCode(t, err, agdiremail.CodeNotFound)
	})

	t.Run("webhooks", func(t *testing.T) {
		created, err := client.Webhooks.Create(ctx, &agdiremail.CreateWebhookRequest{
			Endpoint: "https://93.184.216.34/hook",
			Events:   []agdiremail.WebhookEventType{agdiremail.EventEmailDelivered, agdiremail.EventEmailBounced},
		})
		if err != nil {
			t.Fatal(err)
		}
		deleted := false
		t.Cleanup(func() {
			if !deleted {
				_, _ = client.Webhooks.Delete(context.Background(), created.ID)
			}
		})
		w, err := client.Webhooks.Get(ctx, created.ID)
		if err != nil || w.SigningSecret != created.SigningSecret || w.Status != agdiremail.WebhookEnabled || len(w.Events) != 2 {
			t.Fatalf("%+v %v", w, err)
		}
		if _, err := client.Webhooks.Update(ctx, created.ID, &agdiremail.UpdateWebhookRequest{Status: agdiremail.WebhookDisabled}); err != nil {
			t.Fatal(err)
		}
		if w, err = client.Webhooks.Get(ctx, created.ID); err != nil || w.Status != agdiremail.WebhookDisabled {
			t.Fatalf("%+v %v", w, err)
		}
		rotated, err := client.Webhooks.RotateSecret(ctx, created.ID)
		if err != nil || rotated.SigningSecret == "" || rotated.SigningSecret == created.SigningSecret {
			t.Fatalf("%+v %v", rotated, err)
		}
		page, err := client.Webhooks.Deliveries.List(ctx, created.ID, nil)
		if err != nil || len(page.Data) != 0 || page.HasMore {
			t.Fatalf("%+v %v", page, err)
		}
		if res, err := client.Webhooks.Delete(ctx, created.ID); err != nil || !res.Deleted {
			t.Fatal(res, err)
		}
		deleted = true
	})
}
