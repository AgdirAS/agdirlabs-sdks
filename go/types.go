package agdiremail

import (
	"encoding/json"
	"time"
)

// Tag is a name/value pair attached to an email and echoed back in webhook events.
// Names and values may contain ASCII letters, digits, '_' and '-' only.
type Tag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// SendEmailRequest is one email to send. Addresses are "Name <addr@example.com>" or bare
// addresses. At least one of HTML and Text is required.
type SendEmailRequest struct {
	From    string            `json:"from"`
	To      []string          `json:"to"`
	Subject string            `json:"subject"`
	HTML    string            `json:"html,omitempty"`
	Text    string            `json:"text,omitempty"`
	Cc      []string          `json:"cc,omitempty"`
	Bcc     []string          `json:"bcc,omitempty"`
	ReplyTo []string          `json:"reply_to,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Tags    []Tag             `json:"tags,omitempty"`
	// ScheduledAt delivers the email later. The zero value sends it now.
	ScheduledAt time.Time `json:"scheduled_at,omitzero"`
}

// sendEmailBody is the wire form of SendEmailRequest (omits a zero ScheduledAt on Go 1.23).
type sendEmailBody struct {
	From        string            `json:"from"`
	To          []string          `json:"to"`
	Subject     string            `json:"subject"`
	HTML        string            `json:"html,omitempty"`
	Text        string            `json:"text,omitempty"`
	Cc          []string          `json:"cc,omitempty"`
	Bcc         []string          `json:"bcc,omitempty"`
	ReplyTo     []string          `json:"reply_to,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Tags        []Tag             `json:"tags,omitempty"`
	ScheduledAt string            `json:"scheduled_at,omitempty"`
}

func newSendEmailBody(r *SendEmailRequest) sendEmailBody {
	b := sendEmailBody{
		From: r.From, To: r.To, Subject: r.Subject, HTML: r.HTML, Text: r.Text,
		Cc: r.Cc, Bcc: r.Bcc, ReplyTo: r.ReplyTo, Headers: r.Headers, Tags: r.Tags,
	}
	if b.To == nil {
		b.To = []string{}
	}
	if !r.ScheduledAt.IsZero() {
		b.ScheduledAt = r.ScheduledAt.Format(time.RFC3339Nano)
	}
	return b
}

// SendResult identifies an accepted email.
type SendResult struct {
	ID string `json:"id"`
}

// BatchValidation selects how [EmailsService.SendBatch] handles invalid emails.
type BatchValidation string

const (
	// BatchStrict (the default) rejects the whole batch if any email is invalid.
	BatchStrict BatchValidation = "strict"
	// BatchPermissive accepts the valid emails and reports the others in SendBatchResult.Errors.
	BatchPermissive BatchValidation = "permissive"
)

// MaxBatchSize is the most emails one [EmailsService.SendBatch] call accepts.
const MaxBatchSize = 100

// BatchError reports an email of a permissive batch that was not accepted.
type BatchError struct {
	// Index is the position of the email in the request.
	Index   int    `json:"index"`
	Message string `json:"message"`
}

// SendBatchResult lists the accepted emails in request order (minus rejected ones).
type SendBatchResult struct {
	Data []SendResult `json:"data"`
	// Errors is only populated with BatchPermissive.
	Errors []BatchError `json:"errors"`
}

// EmailStatus is the most advanced delivery state an email has reached.
type EmailStatus string

// Email statuses.
const (
	EmailQueued          EmailStatus = "queued"
	EmailScheduled       EmailStatus = "scheduled"
	EmailSent            EmailStatus = "sent"
	EmailDeliveryDelayed EmailStatus = "delivery_delayed"
	EmailDelivered       EmailStatus = "delivered"
	EmailOpened          EmailStatus = "opened"
	EmailClicked         EmailStatus = "clicked"
	EmailBounced         EmailStatus = "bounced"
	EmailFailed          EmailStatus = "failed"
	EmailComplained      EmailStatus = "complained"
	EmailCanceled        EmailStatus = "canceled"
)

// Email is a sent (or scheduled) email as returned by [EmailsService.Get].
type Email struct {
	ID      string   `json:"id"`
	From    string   `json:"from"`
	To      []string `json:"to"`
	Cc      []string `json:"cc"`
	Bcc     []string `json:"bcc"`
	ReplyTo []string `json:"reply_to"`
	Subject string   `json:"subject"`
	// HTML and Text are empty when the email had no such part.
	HTML      string    `json:"html"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
	// ScheduledAt is nil unless the email was scheduled.
	ScheduledAt *time.Time `json:"scheduled_at"`
	// Status is the most advanced delivery state reached so far.
	Status EmailStatus `json:"status"`
}

type wireEmail struct {
	ID          string     `json:"id"`
	From        string     `json:"from"`
	To          []string   `json:"to"`
	Cc          []string   `json:"cc"`
	Bcc         []string   `json:"bcc"`
	ReplyTo     []string   `json:"reply_to"`
	Subject     string     `json:"subject"`
	HTML        *string    `json:"html"`
	Text        *string    `json:"text"`
	CreatedAt   time.Time  `json:"created_at"`
	ScheduledAt *time.Time `json:"scheduled_at"`
	LastEvent   string     `json:"last_event"`
}

func (w *wireEmail) toEmail() *Email {
	return &Email{
		ID: w.ID, From: w.From, To: list(w.To), Cc: list(w.Cc), Bcc: list(w.Bcc), ReplyTo: list(w.ReplyTo),
		Subject: w.Subject, HTML: deref(w.HTML), Text: deref(w.Text),
		CreatedAt: w.CreatedAt, ScheduledAt: w.ScheduledAt, Status: EmailStatus(w.LastEvent),
	}
}

// IDResult identifies the object an action was applied to.
type IDResult struct {
	ID string `json:"id"`
}

// DeleteResult confirms a deletion.
type DeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// DomainStatus is the verification state of a domain or one of its DNS records.
type DomainStatus string

// Domain statuses.
const (
	DomainNotStarted       DomainStatus = "not_started"
	DomainPending          DomainStatus = "pending"
	DomainVerified         DomainStatus = "verified"
	DomainFailed           DomainStatus = "failed"
	DomainTemporaryFailure DomainStatus = "temporary_failure"
)

// DNSRecord is a record to publish for a domain.
type DNSRecord struct {
	// Purpose is what the record is for: "DKIM", "SPF" or "DMARC".
	Purpose string `json:"purpose"`
	// Name is relative to the registrable domain, e.g. "agdir._domainkey" or "send".
	Name string `json:"name"`
	// Type is "TXT", "MX" or "CNAME".
	Type  string `json:"type"`
	Value string `json:"value"`
	TTL   string `json:"ttl"`
	// Priority is set for MX records only.
	Priority int          `json:"priority,omitempty"`
	Status   DomainStatus `json:"status"`
}

type wireRecord struct {
	Record   string       `json:"record"`
	Name     string       `json:"name"`
	Type     string       `json:"type"`
	Value    string       `json:"value"`
	TTL      string       `json:"ttl"`
	Priority int          `json:"priority"`
	Status   DomainStatus `json:"status"`
}

// Domain is a sending domain.
type Domain struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Status        DomainStatus `json:"status"`
	Region        string       `json:"region"`
	CreatedAt     time.Time    `json:"created_at"`
	OpenTracking  bool         `json:"open_tracking"`
	ClickTracking bool         `json:"click_tracking"`
	// Records is populated by Create and Get, not by List.
	Records []DNSRecord `json:"records,omitempty"`
}

type wireDomain struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Status        DomainStatus `json:"status"`
	Region        string       `json:"region"`
	CreatedAt     time.Time    `json:"created_at"`
	OpenTracking  bool         `json:"open_tracking"`
	ClickTracking bool         `json:"click_tracking"`
	Records       []wireRecord `json:"records"`
}

func (w wireDomain) toDomain() Domain {
	d := Domain{
		ID: w.ID, Name: w.Name, Status: w.Status, Region: w.Region, CreatedAt: w.CreatedAt,
		OpenTracking: w.OpenTracking, ClickTracking: w.ClickTracking,
	}
	if w.Records != nil {
		d.Records = make([]DNSRecord, len(w.Records))
		for i, r := range w.Records {
			d.Records[i] = DNSRecord{
				Purpose: r.Record, Name: r.Name, Type: r.Type, Value: r.Value,
				TTL: r.TTL, Priority: r.Priority, Status: r.Status,
			}
		}
	}
	return d
}

// CreateDomainRequest adds a sending domain.
type CreateDomainRequest struct {
	Name string `json:"name"`
	// Region is optional; the API currently uses eu-north-1.
	Region string `json:"region,omitempty"`
}

// WebhookEventType is an event a webhook can subscribe to.
type WebhookEventType string

// Webhook event types.
const (
	EventEmailSent            WebhookEventType = "email.sent"
	EventEmailScheduled       WebhookEventType = "email.scheduled"
	EventEmailDelivered       WebhookEventType = "email.delivered"
	EventEmailDeliveryDelayed WebhookEventType = "email.delivery_delayed"
	EventEmailComplained      WebhookEventType = "email.complained"
	EventEmailBounced         WebhookEventType = "email.bounced"
	EventEmailOpened          WebhookEventType = "email.opened"
	EventEmailClicked         WebhookEventType = "email.clicked"
	EventEmailFailed          WebhookEventType = "email.failed"
)

// WebhookEventTypes returns every event type a webhook can subscribe to.
func WebhookEventTypes() []WebhookEventType {
	return []WebhookEventType{
		EventEmailSent, EventEmailScheduled, EventEmailDelivered, EventEmailDeliveryDelayed,
		EventEmailComplained, EventEmailBounced, EventEmailOpened, EventEmailClicked, EventEmailFailed,
	}
}

// WebhookStatus tells whether a webhook endpoint receives events.
type WebhookStatus string

// Webhook statuses.
const (
	WebhookEnabled  WebhookStatus = "enabled"
	WebhookDisabled WebhookStatus = "disabled"
)

// Webhook is a registered endpoint.
type Webhook struct {
	ID        string             `json:"id"`
	Endpoint  string             `json:"endpoint"`
	Status    WebhookStatus      `json:"status"`
	Events    []WebhookEventType `json:"events"`
	CreatedAt time.Time          `json:"created_at"`
	// SigningSecret (whsec_…) is populated by Get only; pass it to VerifyWebhook.
	SigningSecret string `json:"signing_secret,omitempty"`
}

// CreateWebhookRequest registers an endpoint.
type CreateWebhookRequest struct {
	// Endpoint is a public https:// URL.
	Endpoint string             `json:"endpoint"`
	Events   []WebhookEventType `json:"events"`
}

// UpdateWebhookRequest changes a webhook. Zero-valued fields are left unchanged.
type UpdateWebhookRequest struct {
	Endpoint string             `json:"endpoint,omitempty"`
	Events   []WebhookEventType `json:"events,omitempty"`
	Status   WebhookStatus      `json:"status,omitempty"`
}

// WebhookSecret is returned when a webhook is created or its secret rotated.
type WebhookSecret struct {
	ID string `json:"id"`
	// SigningSecret (whsec_…) verifies incoming events; keep it secret.
	SigningSecret string `json:"signing_secret"`
}

// WebhookDeliveryStatus is the state of one message to a webhook endpoint.
type WebhookDeliveryStatus string

// Webhook delivery statuses.
const (
	DeliveryPending    WebhookDeliveryStatus = "pending"
	DeliveryAttempting WebhookDeliveryStatus = "attempting"
	DeliverySuccess    WebhookDeliveryStatus = "success"
	DeliveryFailed     WebhookDeliveryStatus = "failed"
)

// WebhookDelivery is one event message sent (or queued) to a webhook endpoint.
type WebhookDelivery struct {
	ID        string                `json:"id"`
	Type      WebhookEventType      `json:"type"`
	Status    WebhookDeliveryStatus `json:"status"`
	CreatedAt time.Time             `json:"created_at"`
	// NextAttemptAt is set by Get while a retry is scheduled.
	NextAttemptAt *time.Time `json:"next_attempt_at,omitempty"`
	// Payload is the exact JSON body sent to the endpoint; set by Get only.
	Payload json.RawMessage `json:"payload,omitempty"`
}

// WebhookAttempt is one HTTP request made for a delivery.
type WebhookAttempt struct {
	ID string `json:"id"`
	// HTTPStatus is 0 when no HTTP response was received.
	HTTPStatus int       `json:"http_status"`
	Response   string    `json:"response"`
	SentAt     time.Time `json:"sent_at"`
}

type wireAttempt struct {
	ID             string    `json:"id"`
	HTTPStatusCode int       `json:"http_status_code"`
	Response       string    `json:"response"`
	SentAt         time.Time `json:"sent_at"`
}

func list(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
