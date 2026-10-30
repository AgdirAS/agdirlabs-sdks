# frozen_string_literal: true

require "test_helper"

class WebhookTest < Minitest::Test
  def self.b64(bytes) = [bytes].pack("m0")
  def b64(bytes) = self.class.b64(bytes)

  E = AgdirLabs::Email
  SECRET = "whsec_#{b64("0123456789abcdef0123456789abcdef")}".freeze
  NOW = Time.at(1_793_000_000)

  PAYLOAD = JSON.generate({
                            type: "email.bounced",
                            created_at: "2026-10-30T10:00:00.000Z",
                            data: {
                              created_at: "2026-10-30T09:59:00.000Z", email_id: "em_1", message_id: "<m@x>",
                              from: "Acme <hi@acme.com>", to: ["a@x.com"], subject: "Hi", tags: { kind: "welcome" },
                              bounce: { message: "Mailbox full", subType: "MailboxFull", type: "Transient" }
                            }
                          })

  # Signs independently of the SDK, straight from the Standard Webhooks definition.
  def signature(payload, id: "msg_1", timestamp: NOW.to_i, secret: SECRET)
    key = secret.delete_prefix("whsec_").unpack1("m0")
    "v1,#{b64(OpenSSL::HMAC.digest("SHA256", key, "#{id}.#{timestamp}.#{payload}"))}"
  end

  def headers(payload = PAYLOAD, **options)
    { "webhook-id" => "msg_1", "webhook-timestamp" => (options[:timestamp] || NOW.to_i).to_s,
      "webhook-signature" => signature(payload, **options) }
  end

  def verify(payload: PAYLOAD, headers: self.headers, secret: SECRET, **options)
    E::Webhook.verify(payload: payload, headers: headers, secret: secret, now: NOW, **options)
  end

  def test_valid_signature_returns_the_event
    event = verify
    assert_equal "email.bounced", event.type
    assert_equal Time.utc(2026, 10, 30, 10), event.created_at
    assert_equal "em_1", event.data.email_id
    assert_equal "<m@x>", event.data.message_id
    assert_equal ["a@x.com"], event.data.to
    assert_equal({ "kind" => "welcome" }, event.data.tags)
    assert_equal E::Bounce.new(type: "Transient", sub_type: "MailboxFull", message: "Mailbox full"), event.data.bounce
    assert_nil event.data.click
    assert_nil event.data.failed
    assert_predicate event, :frozen?
  end

  def test_click_and_failed_details
    payload = JSON.generate({ type: "email.clicked", created_at: "2026-10-30T10:00:00Z",
                              data: { created_at: "2026-10-30T10:00:00Z", email_id: "em_1", from: "a@b.c", to: "x@y.z",
                                      subject: "s", click: { link: "https://x", ipAddress: "1.2.3.4", userAgent: "UA",
                                                             timestamp: "2026-10-30T10:01:00Z" },
                                      failed: { reason: "nope" } } })
    event = verify(payload: payload, headers: headers(payload))
    assert_equal "1.2.3.4", event.data.click.ip_address
    assert_equal Time.utc(2026, 10, 30, 10, 1), event.data.click.timestamp
    assert_equal "nope", event.data.failed.reason
    assert_equal ["x@y.z"], event.data.to
    assert_equal({}, event.data.tags)
    assert_nil event.data.message_id
  end

  def test_tampered_body
    error = assert_raises(E::WebhookVerificationError) { verify(payload: PAYLOAD.sub("Mailbox", "Mailb0x")) }
    assert_equal "Webhook signature does not match", error.message
  end

  def test_wrong_secret
    other = "whsec_#{b64("another secret of 32 bytes......")}"
    assert_raises(E::Webhook::VerificationError) { verify(secret: other) }
  end

  def test_secret_without_prefix_works
    assert_equal "email.bounced", verify(secret: SECRET.delete_prefix("whsec_")).type
  end

  def test_invalid_base64_secret
    error = assert_raises(E::WebhookVerificationError) { verify(secret: "whsec_not*base64!") }
    assert_includes error.message, "base64"
  end

  def test_missing_headers
    %w[webhook-id webhook-timestamp webhook-signature].each do |name|
      error = assert_raises(E::WebhookVerificationError) { verify(headers: headers.except(name)) }
      assert_equal "Missing webhook signature headers", error.message
    end
  end

  def test_stale_and_future_timestamps
    old = NOW.to_i - 301
    assert_raises(E::WebhookVerificationError) { verify(headers: headers(timestamp: old)) }
    future = NOW.to_i + 301
    assert_raises(E::WebhookVerificationError) { verify(headers: headers(timestamp: future)) }
    assert verify(headers: headers(timestamp: old), tolerance: 600)
    assert_raises(E::WebhookVerificationError) { verify(headers: headers.merge("webhook-timestamp" => "abc")) }
  end

  def test_multiple_signatures
    other = "v1,#{b64("x" * 32)}"
    h = headers
    h["webhook-signature"] = "v1a,zzz #{other} #{h["webhook-signature"]}"
    assert verify(headers: h)

    h["webhook-signature"] = "#{other} v2,#{h["webhook-signature"].split.last.delete_prefix("v1,")}"
    assert_raises(E::WebhookVerificationError) { verify(headers: h) }
  end

  def test_header_shapes
    h = headers
    rack = h.to_h { |k, v| ["HTTP_#{k.upcase.tr("-", "_")}", v] }
    assert verify(headers: rack)
    assert verify(headers: h.transform_keys { |k| k.split("-").map(&:capitalize).join("-") })
    assert verify(headers: h.transform_keys(&:to_sym))
    assert verify(headers: h.transform_values { |v| [v] })

    # Objects with a [] reader, like Rails' ActionDispatch::Http::Headers.
    reader = Class.new do
      def initialize(hash) = @hash = hash
      def [](name) = @hash[name]
    end
    assert verify(headers: reader.new(h))
  end

  def test_signed_non_json_payload
    error = assert_raises(E::WebhookVerificationError) { verify(payload: "not json", headers: headers("not json")) }
    assert_equal "Webhook payload is not valid JSON", error.message
  end

  def test_sign_matches_an_independent_implementation
    assert_equal signature(PAYLOAD).delete_prefix("v1,"),
                 E::Webhook.sign(secret: SECRET, id: "msg_1", timestamp: NOW.to_i, payload: PAYLOAD)
  end
end
