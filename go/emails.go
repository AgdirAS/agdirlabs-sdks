package agdiremail

import (
	"context"
	"errors"
)

// RequestOption customizes a single send call.
type RequestOption func(*requestOptions)

type requestOptions struct {
	idempotencyKey string
	validation     BatchValidation
}

// WithIdempotencyKey makes a send safe to repeat: the same key with the same body returns the
// original result instead of sending again. Without it, the client generates a random key per
// call when retries are enabled, so its own retries never send twice.
func WithIdempotencyKey(key string) RequestOption {
	return func(o *requestOptions) { o.idempotencyKey = key }
}

// WithBatchValidation sets how [EmailsService.SendBatch] treats invalid emails.
// It has no effect on [EmailsService.Send].
func WithBatchValidation(v BatchValidation) RequestOption {
	return func(o *requestOptions) { o.validation = v }
}

// EmailsService sends emails and reads their delivery status.
type EmailsService struct {
	http *httpClient
}

func (s *EmailsService) options(opts []RequestOption) requestOptions {
	var o requestOptions
	for _, opt := range opts {
		opt(&o)
	}
	// Without a key a retried send could be delivered twice, so only generate one when we may retry.
	if o.idempotencyKey == "" && s.http.maxRetries > 0 {
		o.idempotencyKey = newIdempotencyKey()
	}
	return o
}

// Send accepts one email for delivery (or for its ScheduledAt time).
func (s *EmailsService) Send(ctx context.Context, email *SendEmailRequest, opts ...RequestOption) (*SendResult, error) {
	if email == nil {
		return nil, errors.New("agdiremail: Send: email is nil")
	}
	o := s.options(opts)
	var out SendResult
	err := s.http.do(ctx, request{
		method:    "POST",
		path:      "/emails",
		body:      newSendEmailBody(email),
		header:    map[string]string{"Idempotency-Key": o.idempotencyKey},
		retryable: o.idempotencyKey != "",
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// SendBatch accepts up to [MaxBatchSize] emails in one request. With the default strict
// validation one invalid email rejects the whole batch; see [WithBatchValidation].
func (s *EmailsService) SendBatch(ctx context.Context, emails []SendEmailRequest, opts ...RequestOption) (*SendBatchResult, error) {
	o := s.options(opts)
	body := make([]sendEmailBody, len(emails))
	for i := range emails {
		body[i] = newSendEmailBody(&emails[i])
	}
	header := map[string]string{"Idempotency-Key": o.idempotencyKey}
	if o.validation == BatchPermissive {
		header["X-Batch-Validation"] = string(BatchPermissive)
	}
	var out SendBatchResult
	err := s.http.do(ctx, request{
		method:    "POST",
		path:      "/emails/batch",
		body:      body,
		header:    header,
		retryable: o.idempotencyKey != "",
	}, &out)
	if err != nil {
		return nil, err
	}
	if out.Data == nil {
		out.Data = []SendResult{}
	}
	if out.Errors == nil {
		out.Errors = []BatchError{}
	}
	return &out, nil
}

// Get reads an email and its current delivery status. Requires a full-access API key.
func (s *EmailsService) Get(ctx context.Context, id string) (*Email, error) {
	var w wireEmail
	if err := s.http.do(ctx, request{method: "GET", path: "/emails/" + seg(id), retryable: true}, &w); err != nil {
		return nil, err
	}
	return w.toEmail(), nil
}

// Cancel cancels a scheduled email that has not been sent yet.
func (s *EmailsService) Cancel(ctx context.Context, id string) (*IDResult, error) {
	var out IDResult
	if err := s.http.do(ctx, request{method: "POST", path: "/emails/" + seg(id) + "/cancel", retryable: true}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
