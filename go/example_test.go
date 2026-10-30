package agdiremail_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	agdiremail "github.com/AgdirAS/agdirlabs-sdks/go"
)

func ExampleNewClient() {
	// Reads AGDIR_EMAIL_API_KEY (and AGDIR_EMAIL_BASE_URL) when no options are given.
	client, err := agdiremail.NewClient(
		agdiremail.WithAPIKey("re_123"),
		agdiremail.WithMaxRetries(3),
		agdiremail.WithTimeout(10*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(client.BaseURL())
}

func ExampleEmailsService_Send() {
	client, err := agdiremail.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	sent, err := client.Emails.Send(context.Background(), &agdiremail.SendEmailRequest{
		From:    "Acme <hello@acme.com>",
		To:      []string{"user@example.com"},
		Subject: "Welcome to Acme",
		HTML:    "<p>Thanks for signing up!</p>",
		Tags:    []agdiremail.Tag{{Name: "kind", Value: "welcome"}},
	}, agdiremail.WithIdempotencyKey("welcome/user-42"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(sent.ID)
}

func ExampleEmailsService_SendBatch() {
	client, err := agdiremail.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	res, err := client.Emails.SendBatch(context.Background(), []agdiremail.SendEmailRequest{
		{From: "hello@acme.com", To: []string{"ana@example.com"}, Subject: "Hi Ana", Text: "…"},
		{From: "hello@acme.com", To: []string{"ben@example.com"}, Subject: "Hi Ben", Text: "…"},
	}, agdiremail.WithBatchValidation(agdiremail.BatchPermissive))
	if err != nil {
		log.Fatal(err)
	}
	for _, e := range res.Errors {
		fmt.Printf("email %d rejected: %s\n", e.Index, e.Message)
	}
}

func ExampleDomainsService_ListAll() {
	client, err := agdiremail.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	for d, err := range client.Domains.ListAll(context.Background(), nil) {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(d.Name, d.Status)
	}
}

func ExampleError() {
	client, err := agdiremail.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	_, err = client.Emails.Get(context.Background(), "4ef9a417-02e9-4d39-ad75-9611e0fcc33c")
	var apiErr *agdiremail.Error
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case agdiremail.CodeNotFound:
			fmt.Println("no such email")
		case agdiremail.CodeRateLimitExceeded:
			fmt.Println("retry after", apiErr.RetryAfter)
		default:
			fmt.Println(apiErr.Status, apiErr.Message)
		}
	}
}

func ExampleVerifyWebhook() {
	http.HandleFunc("POST /webhooks/email", func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body) // the raw bytes: the signature covers them exactly
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		event, err := agdiremail.VerifyWebhook(payload, r.Header, "whsec_…")
		if err != nil {
			http.Error(w, "invalid signature", http.StatusBadRequest)
			return
		}
		if event.Type == agdiremail.EventEmailBounced {
			log.Printf("bounced: %v (%s)", event.Data.To, event.Data.Bounce.Type)
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func ExampleWebhookHandler() {
	http.Handle("POST /webhooks/email", agdiremail.WebhookHandler("whsec_…",
		func(ctx context.Context, event *agdiremail.WebhookEvent) error {
			log.Println(event.Type, event.Data.EmailID)
			return nil // a non-nil error responds 500, so the event is retried
		}))
}
