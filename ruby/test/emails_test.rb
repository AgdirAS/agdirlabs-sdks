# frozen_string_literal: true

require "test_helper"

class EmailsTest < Minitest::Test
  E = AgdirLabs::Email
  UUID = /\A\h{8}-\h{4}-4\h{3}-[89ab]\h{3}-\h{12}\z/

  def test_send_maps_fields_to_the_wire_and_generates_an_idempotency_key
    body = nil
    key = nil
    stub_request(:post, "#{BASE}/emails").to_return do |req|
      body = JSON.parse(req.body)
      key = req.headers["Idempotency-Key"]
      json({ id: "em_1" })
    end

    result = client.emails.send(
      from: "Acme <hi@acme.com>", to: ["a@x.com", "b@x.com"], cc: "c@x.com", bcc: ["d@x.com"],
      reply_to: "support@acme.com", subject: "Hi", html: "<p>Hi</p>", text: "Hi",
      headers: { "X-Entity-Ref-ID": 42 }, tags: [{ name: "kind", value: "welcome" }],
      scheduled_at: Time.utc(2026, 11, 1, 9, 0, 0)
    )

    assert_equal E::SendResult.new(id: "em_1"), result
    assert_predicate result, :frozen?
    assert_equal(
      { "from" => "Acme <hi@acme.com>", "to" => ["a@x.com", "b@x.com"], "cc" => "c@x.com", "bcc" => ["d@x.com"],
        "reply_to" => "support@acme.com", "subject" => "Hi", "html" => "<p>Hi</p>", "text" => "Hi",
        "headers" => { "X-Entity-Ref-ID" => "42" }, "tags" => [{ "name" => "kind", "value" => "welcome" }],
        "scheduled_at" => "2026-11-01T09:00:00Z" }, body
    )
    assert_match UUID, key
  end

  def test_send_omits_unset_fields_and_accepts_hash_tags
    expected = { from: "a@acme.com", to: "b@x.com", subject: "S", text: "T",
                 tags: [{ name: "plan", value: "pro" }], scheduled_at: "in 1 hour" }
    stub = stub_request(:post, "#{BASE}/emails").with(body: expected).to_return(json({ id: "em_2" }))
    client.emails.send(from: "a@acme.com", to: "b@x.com", subject: "S", text: "T", tags: { plan: :pro },
                       scheduled_at: "in 1 hour")
    assert_requested stub
  end

  def test_send_rejects_unknown_fields
    error = assert_raises(ArgumentError) do
      client.emails.send(from: "a", to: "b", subject: "s", text: "t", replyTo: "x")
    end
    assert_includes error.message, "replyTo"
  end

  def test_send_uses_a_given_idempotency_key_and_retries_with_it
    stub_request(:post, "#{BASE}/emails")
      .with(headers: { "Idempotency-Key" => "welcome/42" })
      .to_return(api_error(500, "internal_server_error")).then.to_return(json({ id: "em_3" }))
    result = client.emails.send(from: "a@acme.com", to: "b@x.com", subject: "S", text: "T",
                                idempotency_key: "welcome/42")
    assert_equal "em_3", result.id
    assert_requested :post, "#{BASE}/emails", times: 2
  end

  def test_send_without_retries_has_no_key_and_is_not_retried
    stub_request(:post, "#{BASE}/emails").to_return(api_error(503, "internal_server_error"))
    assert_raises(E::ServerError) do
      client(max_retries: 0).emails.send(from: "a@acme.com", to: "b@x.com", subject: "S", text: "T")
    end
    assert_requested(:post, "#{BASE}/emails", times: 1) { |req| !req.headers.key?("Idempotency-Key") }
  end

  def test_send_batch_strict
    stub = stub_request(:post, "#{BASE}/emails/batch")
           .with { |req| JSON.parse(req.body).size == 2 && !req.headers.key?("X-Batch-Validation") }
           .to_return(json({ data: [{ id: "1" }, { id: "2" }] }))
    result = client.emails.send_batch(
      [
        { from: "a@acme.com", to: "b@x.com", subject: "1", text: "1" },
        { from: "a@acme.com", to: "c@x.com", subject: "2", text: "2", reply_to: "r@x.com" }
      ]
    )
    assert_requested stub
    assert_equal %w[1 2], result.data.map(&:id)
    assert_empty result.errors
  end

  def test_send_batch_permissive
    stub_request(:post, "#{BASE}/emails/batch")
      .with(headers: { "X-Batch-Validation" => "permissive" })
      .to_return(json({ data: [{ id: "1" }], errors: [{ index: 1, message: "Invalid `to` field." }] }))
    result = client.emails.send_batch([{ from: "a", to: "b", subject: "s", text: "t" }] * 2, validation: :permissive)
    assert_equal [E::BatchError.new(index: 1, message: "Invalid `to` field.")], result.errors
    assert_raises(ArgumentError) { client.emails.send_batch([], validation: :lenient) }
  end

  def test_get_parses_the_email
    body = { object: "email", id: "em_1", from: "Acme <hi@acme.com>", to: ["a@x.com"], cc: nil,
             bcc: ["b@x.com"], reply_to: nil, subject: "Hi", html: nil, text: "Hi",
             created_at: "2026-10-30T10:00:00.123Z", scheduled_at: "2026-10-30T11:00:00+00:00",
             last_event: "scheduled" }
    stub_request(:get, "#{BASE}/emails/em_1").to_return(json(body))
    email = client.emails.get("em_1")
    assert_equal "scheduled", email.status
    assert_equal [], email.cc
    assert_equal ["b@x.com"], email.bcc
    assert_equal [], email.reply_to
    assert_nil email.html
    assert_equal Time.utc(2026, 10, 30, 10, 0, 0.123r), email.created_at
    assert_equal Time.utc(2026, 10, 30, 11), email.scheduled_at
    assert_predicate email, :frozen?
    assert_predicate email.to, :frozen?
    assert_raises(FrozenError) { email.to << "x" }
    assert_raises(NoMethodError, FrozenError) { email.status = "sent" }
  end

  def test_cancel
    stub_request(:post, "#{BASE}/emails/em_1/cancel").to_return(json({ object: "email", id: "em_1" }))
    assert_equal "em_1", client.emails.cancel("em_1").id
  end
end
