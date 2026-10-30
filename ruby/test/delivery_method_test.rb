# frozen_string_literal: true

require "test_helper"
require "mail"

class DeliveryMethodTest < Minitest::Test
  E = AgdirLabs::Email

  def setup
    super
    @bodies = []
    @keys = []
    stub_request(:post, "#{BASE}/emails").to_return do |req|
      @bodies << JSON.parse(req.body)
      @keys << req.headers["Idempotency-Key"]
      json({ id: "em_#{@bodies.size}" })
    end
  end

  def deliver(mail, **settings)
    mail.delivery_method(E::DeliveryMethod, { api_key: "re_test" }.merge(settings))
    mail.deliver
    @bodies.last
  end

  def test_multipart_message_with_all_address_fields
    mail = Mail.new do
      from "Acme Støtte <hi@acme.com>"
      to ["Ana <ana@example.com>", "ben@example.com"]
      cc "cc@example.com"
      bcc "hidden@example.com"
      reply_to "support@acme.com"
      subject "Velkommen — welcome"
      text_part { body "Hei Ana" }
      html_part do
        content_type "text/html; charset=UTF-8"
        body "<p>Hei Ana ✓</p>"
      end
    end

    body = deliver(mail)

    assert_equal "\"Acme Støtte\" <hi@acme.com>", body["from"] # quoted display name (RFC 5322)
    assert_equal ["Ana <ana@example.com>", "ben@example.com"], body["to"]
    assert_equal ["cc@example.com"], body["cc"]
    assert_equal ["hidden@example.com"], body["bcc"]
    assert_equal ["support@acme.com"], body["reply_to"]
    assert_equal "Velkommen — welcome", body["subject"]
    assert_equal "<p>Hei Ana ✓</p>", body["html"]
    assert_equal "Hei Ana", body["text"]
    refute body.key?("headers")
    refute body.key?("tags")
    assert_equal "em_1", mail["X-Agdir-Email-Id"].value
  end

  def test_single_part_html_and_text
    html = Mail.new(from: "a@acme.com", to: "b@x.com", subject: "H", content_type: "text/html; charset=UTF-8",
                    body: "<b>Hi</b>")
    body = deliver(html)
    assert_equal "<b>Hi</b>", body["html"]
    refute body.key?("text")

    text = Mail.new(from: "a@acme.com", to: "b@x.com", subject: "T", body: "Plain")
    body = deliver(text)
    assert_equal "Plain", body["text"]
    refute body.key?("html")
    refute body.key?("cc")
  end

  def test_custom_headers_tags_schedule_and_idempotency_key
    mail = Mail.new(from: "a@acme.com", to: "b@x.com", subject: "S", body: "B")
    mail["X-Entity-Ref-ID"] = "inv-1042"
    mail["X-Agdir-Tags"] = "kind=invoice, plan = pro"
    mail["X-Agdir-Scheduled-At"] = "2026-11-01T09:00:00+01:00"
    mail["X-Agdir-Idempotency-Key"] = "invoice/1042"
    mail["List-Id"] = "not forwarded"

    body = deliver(mail)

    assert_equal({ "X-Entity-Ref-ID" => "inv-1042" }, body["headers"])
    assert_equal [{ "name" => "kind", "value" => "invoice" }, { "name" => "plan", "value" => "pro" }], body["tags"]
    assert_equal "2026-11-01T09:00:00+01:00", body["scheduled_at"]
    assert_equal "invoice/1042", @keys.last
  end

  def test_attachments_are_rejected_with_a_clear_error
    mail = Mail.new(from: "a@acme.com", to: "b@x.com", subject: "S", body: "B")
    mail.add_file(filename: "report.csv", content: "a,b\n")
    mail.delivery_method(E::DeliveryMethod, api_key: "re_test")
    error = assert_raises(E::UnsupportedMessageError) { mail.deliver }
    assert_includes error.message, "Attachments are not supported"
    assert_empty @bodies
  end

  def test_api_errors_propagate
    stub_request(:post, "#{BASE}/emails")
      .to_return(api_error(403, "validation_error", "The acme.com domain is not verified."))
    mail = Mail.new(from: "a@acme.com", to: "b@x.com", subject: "S", body: "B")
    error = assert_raises(E::ValidationError) { deliver(mail) }
    assert_equal 403, error.status
  end

  def test_settings_build_the_client
    method = E::DeliveryMethod.new("api_key" => "re_s", "base_url" => "http://localhost:8080/api",
                                   "max_retries" => 0)
    assert_equal "http://localhost:8080/api", method.client.base_url
    assert_equal 0, method.client.max_retries

    custom = E::Client.new(api_key: "re_c")
    assert_same custom, E::DeliveryMethod.new(client: custom).client
  end

  def test_settings_fall_back_to_env
    with_env("AGDIR_EMAIL_API_KEY" => "re_env") do
      assert_equal "https://api.agdir.email", E::DeliveryMethod.new(nil).client.base_url
    end
  end

  def test_returns_the_send_result_with_return_response
    mail = Mail.new(from: "a@acme.com", to: "b@x.com", subject: "S", body: "B")
    mail.delivery_method(E::DeliveryMethod, api_key: "re_test", return_response: true)
    assert_equal E::SendResult.new(id: "em_1"), mail.deliver!
  end
end
