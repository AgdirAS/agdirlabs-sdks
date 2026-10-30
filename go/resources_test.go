package agdiremail

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

var ctx = context.Background()

func TestEmailsSend(t *testing.T) {
	f, c := newFake(t, reply{Body: M{"id": "e1"}}, reply{Body: M{"id": "e2"}})
	at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.FixedZone("CET", 3600))
	res, err := c.Emails.Send(ctx, &SendEmailRequest{
		From:        "Acme <hi@acme.com>",
		To:          []string{"a@x.com", "b@x.com"},
		Subject:     "Hi",
		HTML:        "<p>Hi</p>",
		Cc:          []string{"c@x.com"},
		ReplyTo:     []string{"r@acme.com"},
		Headers:     map[string]string{"X-Ref": "1"},
		Tags:        []Tag{{Name: "kind", Value: "welcome"}},
		ScheduledAt: at,
	})
	if err != nil || res.ID != "e1" {
		t.Fatal(res, err)
	}
	want := M{
		"from": "Acme <hi@acme.com>", "to": []any{"a@x.com", "b@x.com"}, "subject": "Hi", "html": "<p>Hi</p>",
		"cc": []any{"c@x.com"}, "reply_to": []any{"r@acme.com"}, "headers": M{"X-Ref": "1"},
		"tags": []any{M{"name": "kind", "value": "welcome"}}, "scheduled_at": "2026-11-01T12:00:00+01:00",
	}
	r := f.req(0)
	if !reflect.DeepEqual(r.Body, want) {
		t.Errorf("body\n got %v\nwant %v", r.Body, want)
	}
	key := r.Header.Get("Idempotency-Key")
	if len(key) != 36 {
		t.Errorf("generated idempotency key %q", key)
	}

	// Minimal body omits optional fields; a caller key is sent as-is.
	if _, err := c.Emails.Send(ctx, &SendEmailRequest{From: "a@acme.com", To: []string{"b@x.com"}, Subject: "s", Text: "t"}, WithIdempotencyKey("welcome/42")); err != nil {
		t.Fatal(err)
	}
	r = f.req(1)
	if !reflect.DeepEqual(r.Body, M{"from": "a@acme.com", "to": []any{"b@x.com"}, "subject": "s", "text": "t"}) {
		t.Errorf("body %v", r.Body)
	}
	if r.Header.Get("Idempotency-Key") != "welcome/42" {
		t.Errorf("key %q", r.Header.Get("Idempotency-Key"))
	}
}

func TestEmailsSendRetriesWithSameKey(t *testing.T) {
	f, c := newFake(t, apiError(500, "internal_server_error", "x"), reply{Body: M{"id": "e1"}})
	if _, err := c.Emails.Send(ctx, &SendEmailRequest{From: "a@acme.com", To: []string{"b@x.com"}, Subject: "s", Text: "t"}); err != nil {
		t.Fatal(err)
	}
	k0, k1 := f.req(0).Header.Get("Idempotency-Key"), f.req(1).Header.Get("Idempotency-Key")
	if k0 == "" || k0 != k1 {
		t.Fatalf("keys %q %q", k0, k1)
	}
}

func TestEmailsSendWithoutRetriesHasNoKey(t *testing.T) {
	f, c := newFake(t, apiError(500, "internal_server_error", "x"))
	c.http.maxRetries = 0
	_, err := c.Emails.Send(ctx, &SendEmailRequest{From: "a@acme.com", To: []string{"b@x.com"}, Subject: "s", Text: "t"})
	if err == nil || f.req(0).Header.Get("Idempotency-Key") != "" {
		t.Fatalf("err %v key %q", err, f.req(0).Header.Get("Idempotency-Key"))
	}
}

func TestEmailsSendBatch(t *testing.T) {
	f, c := newFake(t,
		reply{Body: M{"data": []any{M{"id": "e1"}, M{"id": "e2"}}}},
		reply{Body: M{"data": []any{M{"id": "e1"}}, "errors": []any{M{"index": 1, "message": "Invalid `to` field."}}}},
	)
	emails := []SendEmailRequest{
		{From: "a@acme.com", To: []string{"b@x.com"}, Subject: "1", Text: "t"},
		{From: "a@acme.com", To: []string{"c@x.com"}, Subject: "2", Text: "t"},
	}
	res, err := c.Emails.SendBatch(ctx, emails)
	if err != nil || len(res.Data) != 2 || res.Data[1].ID != "e2" || res.Errors == nil || len(res.Errors) != 0 {
		t.Fatal(res, err)
	}
	r := f.req(0)
	if r.Path != "/emails/batch" || len(r.Body.([]any)) != 2 || r.Header.Get("X-Batch-Validation") != "" || r.Header.Get("Idempotency-Key") == "" {
		t.Errorf("%+v", r)
	}

	res, err = c.Emails.SendBatch(ctx, emails, WithBatchValidation(BatchPermissive))
	if err != nil || len(res.Data) != 1 || !reflect.DeepEqual(res.Errors, []BatchError{{Index: 1, Message: "Invalid `to` field."}}) {
		t.Fatal(res, err)
	}
	if f.req(1).Header.Get("X-Batch-Validation") != "permissive" {
		t.Error("permissive header missing")
	}
}

func TestEmailsGet(t *testing.T) {
	f, c := newFake(t,
		reply{Body: M{
			"object": "email", "id": "e/1", "from": "a@acme.com", "to": []any{"b@x.com"}, "cc": nil, "bcc": nil, "reply_to": nil,
			"subject": "s", "html": nil, "text": "t", "created_at": "2026-10-30T09:44:50.892Z", "scheduled_at": nil, "last_event": "delivered",
		}},
		reply{Body: M{
			"object": "email", "id": "e2", "from": "a@acme.com", "to": []any{"b@x.com"}, "cc": []any{"c@x.com"}, "bcc": []any{"d@x.com"},
			"reply_to": []any{"r@acme.com"}, "subject": "s", "html": "<p>x</p>", "text": nil,
			"created_at": "2026-10-30T09:44:50.892Z", "scheduled_at": "2026-10-30T10:44:50.000Z", "last_event": "scheduled",
		}},
	)
	e, err := c.Emails.Get(ctx, "e/1")
	if err != nil {
		t.Fatal(err)
	}
	if f.req(0).Path != "/emails/e/1" && f.req(0).Path != "/emails/e%2F1" {
		t.Errorf("path %q", f.req(0).Path)
	}
	if e.Status != EmailDelivered || e.Cc == nil || len(e.Cc) != 0 || e.Bcc == nil || e.ReplyTo == nil || e.HTML != "" || e.Text != "t" || e.ScheduledAt != nil {
		t.Fatalf("%+v", e)
	}
	if !e.CreatedAt.Equal(time.Date(2026, 10, 30, 9, 44, 50, 892e6, time.UTC)) {
		t.Errorf("created %v", e.CreatedAt)
	}
	e, _ = c.Emails.Get(ctx, "e2")
	if e.Status != EmailScheduled || e.ScheduledAt == nil || e.HTML != "<p>x</p>" || e.ReplyTo[0] != "r@acme.com" || e.Bcc[0] != "d@x.com" {
		t.Fatalf("%+v", e)
	}
	// SDK-level JSON uses status, not last_event.
	b, _ := json.Marshal(e)
	var m M
	_ = json.Unmarshal(b, &m)
	if m["status"] != "scheduled" || m["last_event"] != nil {
		t.Errorf("json %s", b)
	}
}

func TestEmailsCancel(t *testing.T) {
	f, c := newFake(t, reply{Body: M{"object": "email", "id": "e1"}})
	res, err := c.Emails.Cancel(ctx, "e1")
	if err != nil || res.ID != "e1" || f.req(0).Path != "/emails/e1/cancel" || f.req(0).Method != "POST" {
		t.Fatal(res, err)
	}
}

var domainJSON = M{
	"object": "domain", "id": "d1", "name": "acme.com", "status": "pending", "created_at": "2026-10-30T09:44:50.892Z",
	"region": "eu-north-1", "capabilities": M{"sending": "enabled", "receiving": "disabled"}, "open_tracking": false, "click_tracking": true,
	"records": []any{
		M{"record": "DKIM", "name": "agdir._domainkey", "type": "TXT", "ttl": "Auto", "status": "pending", "value": "p=MIGf"},
		M{"record": "SPF", "name": "send", "type": "MX", "ttl": "Auto", "status": "pending", "value": "feedback-smtp.eu-north-1.amazonses.com", "priority": 10},
	},
}

func TestDomains(t *testing.T) {
	f, c := newFake(t,
		reply{Body: domainJSON},
		reply{Body: domainJSON},
		reply{Body: M{"object": "domain", "id": "d1"}},
		reply{Body: M{"object": "domain", "id": "d1", "deleted": true}},
	)
	d, err := c.Domains.Create(ctx, &CreateDomainRequest{Name: "acme.com"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.req(0).Body, M{"name": "acme.com"}) {
		t.Errorf("body %v", f.req(0).Body)
	}
	want := []DNSRecord{
		{Purpose: "DKIM", Name: "agdir._domainkey", Type: "TXT", Value: "p=MIGf", TTL: "Auto", Status: DomainPending},
		{Purpose: "SPF", Name: "send", Type: "MX", Value: "feedback-smtp.eu-north-1.amazonses.com", TTL: "Auto", Priority: 10, Status: DomainPending},
	}
	if d.ID != "d1" || d.Status != DomainPending || d.Region != "eu-north-1" || !d.ClickTracking || d.OpenTracking || !reflect.DeepEqual(d.Records, want) {
		t.Fatalf("%+v", d)
	}
	if _, err := c.Domains.Get(ctx, "d1"); err != nil || f.req(1).Method != "GET" || f.req(1).Path != "/domains/d1" {
		t.Fatal(err)
	}
	if v, err := c.Domains.Verify(ctx, "d1"); err != nil || v.ID != "d1" || f.req(2).Path != "/domains/d1/verify" {
		t.Fatal(v, err)
	}
	del, err := c.Domains.Delete(ctx, "d1")
	if err != nil || !del.Deleted || del.ID != "d1" || f.req(3).Method != "DELETE" {
		t.Fatal(del, err)
	}
}

func listOf(hasMore bool, ids ...string) reply {
	data := []any{}
	for _, id := range ids {
		data = append(data, M{"id": id, "name": id + ".com", "status": "verified", "created_at": "2026-10-30T09:44:50.892Z", "region": "eu-north-1"})
	}
	return reply{Body: M{"object": "list", "has_more": hasMore, "data": data}}
}

func TestPagination(t *testing.T) {
	t.Run("page", func(t *testing.T) {
		f, c := newFake(t, listOf(true, "a", "b"), listOf(false, "c"))
		p, err := c.Domains.List(ctx, &ListParams{Limit: 2})
		if err != nil || !p.HasMore || p.NextCursor != "b" || len(p.Data) != 2 || p.Data[0].Records != nil {
			t.Fatal(p, err)
		}
		if q := f.req(0).Query; q["limit"][0] != "2" || q["after"] != nil {
			t.Errorf("query %v", q)
		}
		p, _ = c.Domains.List(ctx, &ListParams{Limit: 2, After: p.NextCursor})
		if p.HasMore || p.NextCursor != "" || f.req(1).Query["after"][0] != "b" {
			t.Fatal(p)
		}
	})
	t.Run("nil params send no query", func(t *testing.T) {
		f, c := newFake(t, listOf(false))
		if _, err := c.Domains.List(ctx, nil); err != nil || len(f.req(0).Query) != 0 {
			t.Fatal(err, f.req(0).Query)
		}
	})
	t.Run("ListAll walks every page", func(t *testing.T) {
		f, c := newFake(t, listOf(true, "a"), listOf(true, "b"), listOf(false, "c"))
		var names []string
		for d, err := range c.Domains.ListAll(ctx, &ListParams{Limit: 1}) {
			if err != nil {
				t.Fatal(err)
			}
			names = append(names, d.Name)
		}
		if !reflect.DeepEqual(names, []string{"a.com", "b.com", "c.com"}) {
			t.Fatal(names)
		}
		if f.req(2).Query["after"][0] != "b" || f.req(2).Query["limit"][0] != "1" {
			t.Error(f.req(2).Query)
		}
	})
	t.Run("ListAll stops early on break", func(t *testing.T) {
		f, c := newFake(t, listOf(true, "a", "b"), listOf(false, "c"))
		for range c.Domains.ListAll(ctx, nil) {
			break
		}
		if f.count() != 1 {
			t.Fatalf("requests %d", f.count())
		}
	})
	t.Run("ListAll yields errors", func(t *testing.T) {
		_, c := newFake(t, listOf(true, "a"), apiError(403, "invalid_api_key", "API key is invalid"))
		var got []string
		var gotErr error
		for d, err := range c.Domains.ListAll(ctx, nil) {
			if err != nil {
				gotErr = err
				break
			}
			got = append(got, d.ID)
		}
		var apiErr *Error
		if len(got) != 1 || !errors.As(gotErr, &apiErr) || apiErr.Code != CodeInvalidAPIKey {
			t.Fatal(got, gotErr)
		}
	})
}

func TestWebhooks(t *testing.T) {
	hook := M{"id": "w1", "endpoint": "https://acme.com/hook", "created_at": "2026-10-30T09:44:50.892Z", "status": "enabled", "events": []any{"email.sent"}}
	withSecret := M{"signing_secret": "whsec_abc"}
	for k, v := range hook {
		withSecret[k] = v
	}
	f, c := newFake(t,
		reply{Body: M{"object": "webhook", "id": "w1", "signing_secret": "whsec_abc"}},
		reply{Body: withSecret},
		reply{Body: M{"object": "list", "has_more": false, "data": []any{hook}}},
		reply{Body: M{"object": "webhook", "id": "w1"}},
		reply{Body: M{"object": "webhook", "id": "w1", "signing_secret": "whsec_new"}},
		reply{Body: M{"object": "webhook", "id": "w1", "deleted": true}},
	)
	created, err := c.Webhooks.Create(ctx, &CreateWebhookRequest{Endpoint: "https://acme.com/hook", Events: []WebhookEventType{EventEmailSent, EventEmailBounced}})
	if err != nil || created.ID != "w1" || created.SigningSecret != "whsec_abc" {
		t.Fatal(created, err)
	}
	if !reflect.DeepEqual(f.req(0).Body, M{"endpoint": "https://acme.com/hook", "events": []any{"email.sent", "email.bounced"}}) {
		t.Errorf("body %v", f.req(0).Body)
	}
	w, err := c.Webhooks.Get(ctx, "w1")
	if err != nil || w.SigningSecret != "whsec_abc" || w.Status != WebhookEnabled || w.Events[0] != EventEmailSent || w.CreatedAt.IsZero() {
		t.Fatal(w, err)
	}
	p, err := c.Webhooks.List(ctx, nil)
	if err != nil || len(p.Data) != 1 || p.Data[0].Endpoint != "https://acme.com/hook" || p.Data[0].SigningSecret != "" {
		t.Fatal(p, err)
	}
	if _, err := c.Webhooks.Update(ctx, "w1", &UpdateWebhookRequest{Status: WebhookDisabled}); err != nil {
		t.Fatal(err)
	}
	if r := f.req(3); r.Method != "PATCH" || !reflect.DeepEqual(r.Body, M{"status": "disabled"}) {
		t.Errorf("%s %v", r.Method, r.Body)
	}
	rot, err := c.Webhooks.RotateSecret(ctx, "w1")
	if err != nil || rot.SigningSecret != "whsec_new" || f.req(4).Path != "/webhooks/w1/signing-secret/rotate" {
		t.Fatal(rot, err)
	}
	del, err := c.Webhooks.Delete(ctx, "w1")
	if err != nil || !del.Deleted || f.req(5).Method != "DELETE" {
		t.Fatal(del, err)
	}
}

func TestWebhookDeliveries(t *testing.T) {
	ev := M{"id": "ev1", "type": "email.delivered", "created_at": "2026-10-30T09:44:50.892Z", "status": "failed"}
	detail := M{"next_attempt_at": "2026-10-30T10:00:00.000Z", "payload": M{"type": "email.delivered", "data": M{"email_id": "e1"}}}
	for k, v := range ev {
		detail[k] = v
	}
	f, c := newFake(t,
		reply{Body: M{"object": "list", "has_more": true, "data": []any{ev}}},
		reply{Body: detail},
		reply{Body: M{"object": "list", "has_more": false, "data": []any{M{"id": "a1", "http_status_code": 503, "response": "busy", "sent_at": "2026-10-30T09:45:00.000Z"}}}},
		reply{Body: M{"object": "webhook_event", "id": "ev1"}},
	)
	p, err := c.Webhooks.Deliveries.List(ctx, "w1", &ListParams{Limit: 1})
	if err != nil || p.NextCursor != "ev1" || p.Data[0].Status != DeliveryFailed || p.Data[0].Type != EventEmailDelivered || f.req(0).Path != "/webhooks/w1/events" {
		t.Fatal(p, err)
	}
	d, err := c.Webhooks.Deliveries.Get(ctx, "w1", "ev1")
	if err != nil || d.NextAttemptAt == nil || f.req(1).Path != "/webhooks/w1/events/ev1" {
		t.Fatal(d, err)
	}
	var payload M
	if err := json.Unmarshal(d.Payload, &payload); err != nil || payload["type"] != "email.delivered" {
		t.Fatal(string(d.Payload), err)
	}
	a, err := c.Webhooks.Deliveries.Attempts(ctx, "w1", "ev1", nil)
	if err != nil || a.Data[0].HTTPStatus != 503 || a.Data[0].Response != "busy" || a.Data[0].SentAt.IsZero() || f.req(2).Path != "/webhooks/w1/events/ev1/attempts" {
		t.Fatal(a, err)
	}
	r, err := c.Webhooks.Deliveries.Replay(ctx, "w1", "ev1")
	if err != nil || r.ID != "ev1" || f.req(3).Method != "POST" || f.req(3).Path != "/webhooks/w1/events/ev1/replay" {
		t.Fatal(r, err)
	}
}

func TestDeliveriesListAll(t *testing.T) {
	ev := func(id string) M {
		return M{"id": id, "type": "email.sent", "created_at": "2026-10-30T09:44:50.892Z", "status": "success"}
	}
	f, c := newFake(t,
		reply{Body: M{"object": "list", "has_more": true, "data": []any{ev("1")}}},
		reply{Body: M{"object": "list", "has_more": false, "data": []any{ev("2")}}},
	)
	var ids []string
	for d, err := range c.Webhooks.Deliveries.ListAll(ctx, "w1", nil) {
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, d.ID)
	}
	if !reflect.DeepEqual(ids, []string{"1", "2"}) || f.req(1).Query["after"][0] != "1" {
		t.Fatal(ids)
	}
}

func TestNilParams(t *testing.T) {
	_, c := newFake(t)
	if _, err := c.Emails.Send(ctx, nil); err == nil {
		t.Error("Send(nil)")
	}
	if _, err := c.Domains.Create(ctx, nil); err == nil {
		t.Error("Domains.Create(nil)")
	}
	if _, err := c.Webhooks.Create(ctx, nil); err == nil {
		t.Error("Webhooks.Create(nil)")
	}
}
