package agdiremail

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DefaultWebhookTolerance is how far a webhook timestamp may be from now before
// [VerifyWebhook] rejects the message.
const DefaultWebhookTolerance = 5 * time.Minute

// WebhookEvent is an event received on your webhook endpoint.
type WebhookEvent struct {
	Type      WebhookEventType `json:"type"`
	CreatedAt time.Time        `json:"created_at"`
	Data      WebhookEventData `json:"data"`
}

// WebhookEventData describes the email an event is about.
type WebhookEventData struct {
	EmailID string `json:"email_id"`
	// MessageID is the SMTP Message-ID, once the email has been sent.
	MessageID string            `json:"message_id,omitempty"`
	From      string            `json:"from"`
	To        []string          `json:"to"`
	Subject   string            `json:"subject"`
	Tags      map[string]string `json:"tags"`
	CreatedAt time.Time         `json:"created_at"`
	// Bounce is set for email.bounced.
	Bounce *WebhookBounce `json:"bounce,omitempty"`
	// Click is set for email.clicked.
	Click *WebhookClick `json:"click,omitempty"`
	// Failed is set for email.failed.
	Failed *WebhookFailure `json:"failed,omitempty"`
}

// WebhookBounce details a bounce.
type WebhookBounce struct {
	Type    string `json:"type"`
	SubType string `json:"subType"`
	Message string `json:"message"`
}

// WebhookClick details a link click.
type WebhookClick struct {
	Link      string    `json:"link"`
	IPAddress string    `json:"ipAddress"`
	UserAgent string    `json:"userAgent"`
	Timestamp time.Time `json:"timestamp"`
}

// WebhookFailure explains why an email could not be sent.
type WebhookFailure struct {
	Reason string `json:"reason"`
}

// VerifyOption configures [VerifyWebhook].
type VerifyOption func(*verifyOptions)

type verifyOptions struct {
	tolerance time.Duration
	now       func() time.Time
}

// WithTolerance sets how old (or how far in the future) a message may be. Default 5 minutes.
func WithTolerance(d time.Duration) VerifyOption {
	return func(o *verifyOptions) { o.tolerance = d }
}

// WithVerifyTime sets the current time used for the timestamp check (for tests).
func WithVerifyTime(t time.Time) VerifyOption {
	return func(o *verifyOptions) { o.now = func() time.Time { return t } }
}

// VerifyWebhook checks that a webhook request came from Agdir Email and returns the parsed
// event. Messages are signed per the Standard Webhooks spec (webhook-id, webhook-timestamp and
// webhook-signature headers). payload must be the raw request body, exactly as received.
// It returns a *[WebhookVerificationError] if anything doesn't match.
func VerifyWebhook(payload []byte, headers http.Header, secret string, opts ...VerifyOption) (*WebhookEvent, error) {
	o := verifyOptions{tolerance: DefaultWebhookTolerance, now: time.Now}
	for _, opt := range opts {
		opt(&o)
	}
	fail := func(reason string) (*WebhookEvent, error) {
		return nil, &WebhookVerificationError{Reason: reason}
	}

	id := headers.Get("webhook-id")
	timestamp := headers.Get("webhook-timestamp")
	signatures := headers.Get("webhook-signature")
	if id == "" || timestamp == "" || signatures == "" {
		return fail("missing webhook signature headers")
	}

	sentAt, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fail("webhook timestamp is outside the allowed tolerance")
	}
	diff := o.now().Unix() - sentAt
	if diff < 0 {
		diff = -diff
	}
	if diff > int64(o.tolerance/time.Second) {
		return fail("webhook timestamp is outside the allowed tolerance")
	}

	key, err := decodeSecret(secret)
	if err != nil {
		return fail("signing secret is not valid base64")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + timestamp + "."))
	mac.Write(payload)
	expected := []byte(base64.StdEncoding.EncodeToString(mac.Sum(nil)))

	// The header may hold several space-separated "v1,<sig>" entries (e.g. during secret rotation).
	valid := false
	for _, entry := range strings.Fields(signatures) {
		if sig, ok := strings.CutPrefix(entry, "v1,"); ok && hmac.Equal([]byte(sig), expected) {
			valid = true
		}
	}
	if !valid {
		return fail("webhook signature does not match")
	}

	var event WebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return fail("webhook payload is not valid JSON")
	}
	if event.Data.To == nil {
		event.Data.To = []string{}
	}
	if event.Data.Tags == nil {
		event.Data.Tags = map[string]string{}
	}
	return &event, nil
}

func decodeSecret(secret string) ([]byte, error) {
	s := strings.TrimPrefix(secret, "whsec_")
	key, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		key, err = base64.RawStdEncoding.DecodeString(s)
	}
	if err == nil && len(key) == 0 {
		err = errors.New("empty key")
	}
	return key, err
}

// maxWebhookBody caps the request body read by WebhookHandler.
const maxWebhookBody = 1 << 20

// WebhookHandler returns an http.Handler that verifies each request with [VerifyWebhook]
// and passes the event to handle. It responds 405 to non-POST requests, 400 when verification
// fails, 500 when handle returns an error (so the event is retried), and 204 otherwise.
//
//	http.Handle("POST /webhooks/email", agdiremail.WebhookHandler(secret,
//		func(ctx context.Context, e *agdiremail.WebhookEvent) error {
//			log.Println(e.Type, e.Data.EmailID)
//			return nil
//		}))
func WebhookHandler(secret string, handle func(ctx context.Context, event *WebhookEvent) error, opts ...VerifyOption) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
		if err != nil {
			http.Error(w, "could not read body", http.StatusBadRequest)
			return
		}
		event, err := VerifyWebhook(payload, r.Header, secret, opts...)
		if err != nil {
			http.Error(w, "invalid webhook signature", http.StatusBadRequest)
			return
		}
		if err := handle(r.Context(), event); err != nil {
			http.Error(w, "webhook handler failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
