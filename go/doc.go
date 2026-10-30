// Package agdiremail is the official Go client for Agdir Email (https://agdir.email),
// programmatic SMTP for developers, hosted in the EU.
//
// The import path ends in /go, so import it with a name:
//
//	import agdiremail "github.com/AgdirAS/agdirlabs-sdks/go"
//
// Create a client and send an email:
//
//	client, err := agdiremail.NewClient(agdiremail.WithAPIKey("re_…"))
//	if err != nil {
//		log.Fatal(err)
//	}
//	sent, err := client.Emails.Send(ctx, &agdiremail.SendEmailRequest{
//		From:    "Acme <hello@acme.com>",
//		To:      []string{"user@example.com"},
//		Subject: "Welcome to Acme",
//		HTML:    "<p>Thanks for signing up!</p>",
//	})
//
// NewClient with no options reads the API key from AGDIR_EMAIL_API_KEY and the base URL
// from AGDIR_EMAIL_BASE_URL (default https://api.agdir.email).
//
// Every method takes a [context.Context] for cancellation. Failed calls return an *[Error]
// carrying the API error code; use [errors.As] to inspect it. Rate limits, server errors and
// network failures are retried automatically when it is safe to do so (see [WithMaxRetries]).
//
// Incoming webhook requests are verified with [VerifyWebhook] or [WebhookHandler].
package agdiremail
