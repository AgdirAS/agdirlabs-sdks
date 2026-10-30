package agdiremail

import (
	"context"
	"errors"
	"iter"
)

// WebhooksService manages webhook endpoints. Requires a full-access API key.
type WebhooksService struct {
	// Deliveries lists the messages sent to an endpoint and their HTTP attempts.
	Deliveries *WebhookDeliveriesService

	http *httpClient
}

// Create registers an endpoint. Keep the returned SigningSecret to verify incoming events.
func (s *WebhooksService) Create(ctx context.Context, params *CreateWebhookRequest) (*WebhookSecret, error) {
	if params == nil {
		return nil, errors.New("agdiremail: Webhooks.Create: params is nil")
	}
	var out WebhookSecret
	if err := s.http.do(ctx, request{method: "POST", path: "/webhooks", body: params}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get reads a webhook, including its signing secret.
func (s *WebhooksService) Get(ctx context.Context, id string) (*Webhook, error) {
	var out Webhook
	if err := s.http.do(ctx, request{method: "GET", path: "/webhooks/" + seg(id), retryable: true}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns one page of webhooks (without signing secrets).
func (s *WebhooksService) List(ctx context.Context, params *ListParams) (*Page[Webhook], error) {
	var w wirePage[Webhook]
	if err := s.http.do(ctx, request{method: "GET", path: "/webhooks", query: listQuery(params), retryable: true}, &w); err != nil {
		return nil, err
	}
	return toPage(w, same[Webhook], func(h Webhook) string { return h.ID }), nil
}

// ListAll iterates over every webhook, fetching further pages as needed.
// Iteration stops after the first error.
func (s *WebhooksService) ListAll(ctx context.Context, params *ListParams) iter.Seq2[Webhook, error] {
	return paginate(ctx, params, s.List)
}

// Update changes a webhook's endpoint, events or status.
func (s *WebhooksService) Update(ctx context.Context, id string, params *UpdateWebhookRequest) (*IDResult, error) {
	if params == nil {
		params = &UpdateWebhookRequest{}
	}
	var out IDResult
	if err := s.http.do(ctx, request{method: "PATCH", path: "/webhooks/" + seg(id), body: params, retryable: true}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete removes a webhook.
func (s *WebhooksService) Delete(ctx context.Context, id string) (*DeleteResult, error) {
	var out DeleteResult
	if err := s.http.do(ctx, request{method: "DELETE", path: "/webhooks/" + seg(id)}, &out); err != nil {
		return nil, err
	}
	out.Deleted = true
	return &out, nil
}

// RotateSecret issues a new signing secret; the old one stops working immediately.
func (s *WebhooksService) RotateSecret(ctx context.Context, id string) (*WebhookSecret, error) {
	var out WebhookSecret
	if err := s.http.do(ctx, request{method: "POST", path: "/webhooks/" + seg(id) + "/signing-secret/rotate"}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// WebhookDeliveriesService reads the messages sent (or queued) to a webhook endpoint,
// with their HTTP attempts, and replays them.
type WebhookDeliveriesService struct {
	http *httpClient
}

func deliveriesPath(webhookID string) string { return "/webhooks/" + seg(webhookID) + "/events" }

// List returns one page of a webhook's deliveries, newest first.
func (s *WebhookDeliveriesService) List(ctx context.Context, webhookID string, params *ListParams) (*Page[WebhookDelivery], error) {
	var w wirePage[WebhookDelivery]
	if err := s.http.do(ctx, request{method: "GET", path: deliveriesPath(webhookID), query: listQuery(params), retryable: true}, &w); err != nil {
		return nil, err
	}
	return toPage(w, same[WebhookDelivery], func(d WebhookDelivery) string { return d.ID }), nil
}

// ListAll iterates over every delivery of a webhook, fetching further pages as needed.
// Iteration stops after the first error.
func (s *WebhookDeliveriesService) ListAll(ctx context.Context, webhookID string, params *ListParams) iter.Seq2[WebhookDelivery, error] {
	return paginate(ctx, params, func(ctx context.Context, p *ListParams) (*Page[WebhookDelivery], error) {
		return s.List(ctx, webhookID, p)
	})
}

// Get reads one delivery, including the exact payload sent and the next retry time.
func (s *WebhookDeliveriesService) Get(ctx context.Context, webhookID, deliveryID string) (*WebhookDelivery, error) {
	var out WebhookDelivery
	if err := s.http.do(ctx, request{method: "GET", path: deliveriesPath(webhookID) + "/" + seg(deliveryID), retryable: true}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Attempts returns one page of the HTTP attempts made for a delivery, newest first.
func (s *WebhookDeliveriesService) Attempts(ctx context.Context, webhookID, deliveryID string, params *ListParams) (*Page[WebhookAttempt], error) {
	var w wirePage[wireAttempt]
	path := deliveriesPath(webhookID) + "/" + seg(deliveryID) + "/attempts"
	if err := s.http.do(ctx, request{method: "GET", path: path, query: listQuery(params), retryable: true}, &w); err != nil {
		return nil, err
	}
	conv := func(a wireAttempt) WebhookAttempt {
		return WebhookAttempt{ID: a.ID, HTTPStatus: a.HTTPStatusCode, Response: a.Response, SentAt: a.SentAt}
	}
	return toPage(w, conv, func(a WebhookAttempt) string { return a.ID }), nil
}

// Replay sends the identical message again, once, right now (also to disabled endpoints).
func (s *WebhookDeliveriesService) Replay(ctx context.Context, webhookID, deliveryID string) (*IDResult, error) {
	var out IDResult
	if err := s.http.do(ctx, request{method: "POST", path: deliveriesPath(webhookID) + "/" + seg(deliveryID) + "/replay"}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
