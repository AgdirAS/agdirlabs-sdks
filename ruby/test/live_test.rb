# frozen_string_literal: true

require "test_helper"

# Opt-in integration test against a real Agdir Email API. Runs only when
# AGDIR_EMAIL_LIVE_BASE_URL and AGDIR_EMAIL_LIVE_API_KEY are set (and AGDIR_EMAIL_LIVE_FROM, the
# sender on a verified domain, for the email tests). Never point it at production.
class LiveTest < Minitest::Test
  E = AgdirLabs::Email
  BASE_URL = ENV.fetch("AGDIR_EMAIL_LIVE_BASE_URL", "")
  API_KEY = ENV.fetch("AGDIR_EMAIL_LIVE_API_KEY", "")
  FROM = ENV.fetch("AGDIR_EMAIL_LIVE_FROM", "")

  def setup
    super
    skip "set AGDIR_EMAIL_LIVE_BASE_URL and AGDIR_EMAIL_LIVE_API_KEY to run" if BASE_URL.empty? || API_KEY.empty?
    WebMock.allow_net_connect!
    Thread.current[:agdir_real_sleep] = true
    @client = E::Client.new(api_key: API_KEY, base_url: BASE_URL, max_retries: 5)
  end

  def teardown
    Thread.current[:agdir_real_sleep] = nil
    WebMock.disable_net_connect!
    super
  end

  def require_from
    skip "set AGDIR_EMAIL_LIVE_FROM to run" if FROM.empty?
  end

  def test_scheduled_send_get_cancel
    require_from
    sent = @client.emails.send(from: FROM, to: "delivered@example.com", subject: "Ruby SDK live test",
                               html: "<p>Scheduled</p>", text: "Scheduled", tags: { sdk: "ruby" },
                               headers: { "X-Live-Test" => "1" }, scheduled_at: Time.now + 3600)
    email = @client.emails.get(sent.id)
    assert_equal "scheduled", email.status
    assert_equal ["delivered@example.com"], email.to
    assert_kind_of Time, email.scheduled_at

    assert_equal sent.id, @client.emails.cancel(sent.id).id
    assert_equal "canceled", @client.emails.get(sent.id).status
  end

  def test_idempotent_replay_returns_the_same_id
    require_from
    key = "ruby-live-#{SecureRandom.uuid}"
    params = { from: FROM, to: "delivered@example.com", subject: "Idempotent", text: "Once",
               scheduled_at: Time.now + 3600 }
    first = @client.emails.send(idempotency_key: key, **params)
    second = @client.emails.send(idempotency_key: key, **params)
    assert_equal first.id, second.id
    @client.emails.cancel(first.id)
  end

  def test_permissive_batch_reports_the_bad_recipient
    require_from
    result = @client.emails.send_batch(
      [
        { from: FROM, to: "delivered@example.com", subject: "Batch 1", text: "1", scheduled_at: Time.now + 3600 },
        { from: FROM, to: "not-an-address", subject: "Batch 2", text: "2" }
      ],
      validation: :permissive
    )
    assert_equal 1, result.data.size
    assert_equal [1], result.errors.map(&:index)
    @client.emails.cancel(result.data.first.id)
  end

  def test_validation_error_on_bad_to
    require_from
    error = assert_raises(E::ValidationError) do
      @client.emails.send(from: FROM, to: "not-an-address", subject: "Bad", text: "x")
    end
    assert_equal "validation_error", error.code
    assert_equal 422, error.status
  end

  def test_invalid_api_key
    client = E::Client.new(api_key: "re_invalid", base_url: BASE_URL)
    error = assert_raises(E::AuthenticationError) { client.domains.list }
    assert_equal "invalid_api_key", error.code
    assert_equal 403, error.status
  end

  def test_domain_lifecycle
    name = "ruby-#{Time.now.to_i}-#{SecureRandom.hex(3)}.test"
    domain = @client.domains.create(name: name)
    begin
      assert_equal name, domain.name
      refute_empty domain.records
      assert_equal domain.id, @client.domains.get(domain.id).id
      assert_equal domain.id, @client.domains.verify(domain.id).id
      ids = @client.domains.list_all(limit: 1).map(&:id)
      assert_includes ids, domain.id
    ensure
      assert_predicate @client.domains.delete(domain.id), :deleted?
    end
    error = assert_raises(E::NotFoundError) { @client.domains.get(domain.id) }
    assert_equal "not_found", error.code
  end

  def test_webhook_lifecycle
    created = @client.webhooks.create(endpoint: "https://93.184.216.34/hook", events: %w[email.delivered email.bounced])
    begin
      assert created.signing_secret.start_with?("whsec_")
      hook = @client.webhooks.get(created.id)
      assert_equal "https://93.184.216.34/hook", hook.endpoint
      assert_equal created.signing_secret, hook.signing_secret

      @client.webhooks.update(created.id, status: :disabled)
      assert_equal "disabled", @client.webhooks.get(created.id).status

      rotated = @client.webhooks.rotate_secret(created.id)
      refute_equal created.signing_secret, rotated.signing_secret

      page = @client.webhooks.deliveries.list(created.id)
      assert_empty page.data
      refute_predicate page, :has_more?
    ensure
      assert_predicate @client.webhooks.delete(created.id), :deleted?
    end
  end
end
