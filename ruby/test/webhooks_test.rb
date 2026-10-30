# frozen_string_literal: true

require "test_helper"

class WebhooksTest < Minitest::Test
  E = AgdirLabs::Email

  def webhook_json(id = "wh_1", secret: nil)
    json = { id: id, endpoint: "https://acme.com/hook", created_at: "2026-10-30T10:00:00Z", status: "enabled",
             events: ["email.delivered"] }
    secret ? json.merge(object: "webhook", signing_secret: secret) : json
  end

  def test_create
    stub = stub_request(:post, "#{BASE}/webhooks")
           .with(body: { endpoint: "https://acme.com/hook", events: ["email.delivered", "email.bounced"] })
           .to_return(json({ object: "webhook", id: "wh_1", signing_secret: "whsec_abc" }))
    result = client.webhooks.create(endpoint: "https://acme.com/hook", events: %w[email.delivered email.bounced])
    assert_requested stub
    assert_equal E::WebhookSecret.new(id: "wh_1", signing_secret: "whsec_abc"), result
  end

  def test_get_and_list
    stub_request(:get, "#{BASE}/webhooks/wh_1").to_return(json(webhook_json(secret: "whsec_abc")))
    stub_request(:get, "#{BASE}/webhooks?after=wh_0&limit=5")
      .to_return(json({ object: "list", has_more: false, data: [webhook_json] }))

    hook = client.webhooks.get("wh_1")
    assert_equal "whsec_abc", hook.signing_secret
    assert_predicate hook, :enabled?
    assert_equal ["email.delivered"], hook.events
    assert_equal Time.utc(2026, 10, 30, 10), hook.created_at

    page = client.webhooks.list(limit: 5, after: "wh_0")
    assert_nil page.first.signing_secret
    refute_predicate page, :has_more?
  end

  def test_list_all
    stub_request(:get, "#{BASE}/webhooks").to_return(json({ object: "list", has_more: true, data: [webhook_json] }))
    stub_request(:get, "#{BASE}/webhooks?after=wh_1")
      .to_return(json({ object: "list", has_more: false, data: [webhook_json("wh_2")] }))
    assert_equal %w[wh_1 wh_2], client.webhooks.list_all.map(&:id)
  end

  def test_update_delete_rotate
    update = stub_request(:patch, "#{BASE}/webhooks/wh_1").with(body: { status: "disabled" })
                                                          .to_return(json({ object: "webhook", id: "wh_1" }))
    stub_request(:delete, "#{BASE}/webhooks/wh_1").to_return(json({ object: "webhook", id: "wh_1", deleted: true }))
    stub_request(:post, "#{BASE}/webhooks/wh_1/signing-secret/rotate")
      .to_return(json({ object: "webhook", id: "wh_1", signing_secret: "whsec_new" }))

    assert_equal "wh_1", client.webhooks.update("wh_1", status: :disabled).id
    assert_requested update
    assert_predicate client.webhooks.delete("wh_1"), :deleted?
    assert_equal "whsec_new", client.webhooks.rotate_secret("wh_1").signing_secret
  end

  def test_update_is_retried
    stub_request(:patch, "#{BASE}/webhooks/wh_1")
      .to_return(api_error(502, "internal_server_error")).then.to_return(json({ object: "webhook", id: "wh_1" }))
    client.webhooks.update("wh_1", events: "email.sent")
    assert_requested(:patch, "#{BASE}/webhooks/wh_1", times: 2, body: { events: ["email.sent"] })
  end

  def test_deliveries
    base = "#{BASE}/webhooks/wh_1/events"
    created = "2026-10-30T10:00:00Z"
    summary = { id: "ev_1", type: "email.delivered", created_at: created, status: "success" }
    detail = summary.merge(status: "pending", next_attempt_at: "2026-10-30T10:05:00Z",
                           payload: { type: "email.delivered", data: { email_id: "em_1" } })
    attempt = { id: "at_1", http_status_code: 500, response: "oops", sent_at: "2026-10-30T10:00:01Z" }
    stub_request(:get, "#{base}?limit=10").to_return(json({ object: "list", has_more: false, data: [summary] }))
    stub_request(:get, "#{base}/ev_1").to_return(json(detail))
    stub_request(:get, "#{base}/ev_1/attempts").to_return(json({ object: "list", has_more: false, data: [attempt] }))
    stub_request(:post, "#{base}/ev_1/replay").to_return(json({ object: "webhook_event", id: "ev_2" }))

    deliveries = client.webhooks.deliveries
    assert_equal ["success"], deliveries.list("wh_1", limit: 10).map(&:status)
    assert_equal ["ev_1"], deliveries.list_all("wh_1", limit: 10).map(&:id)

    got = deliveries.get("wh_1", "ev_1")
    assert_equal Time.utc(2026, 10, 30, 10, 5), got.next_attempt_at
    assert_equal "em_1", got.payload.dig("data", "email_id")
    assert_predicate got.payload["data"], :frozen?

    first = deliveries.attempts("wh_1", "ev_1").first
    assert_equal 500, first.http_status
    assert_equal "oops", first.response
    assert_equal Time.utc(2026, 10, 30, 10, 0, 1), first.sent_at

    assert_equal "ev_2", deliveries.replay("wh_1", "ev_1").id
  end

  def test_event_types
    assert_includes E::Resources::Webhooks::EVENT_TYPES, "email.delivery_delayed"
    assert_equal 9, E::Resources::Webhooks::EVENT_TYPES.size
  end
end
