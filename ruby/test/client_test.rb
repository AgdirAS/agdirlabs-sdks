# frozen_string_literal: true

require "test_helper"

class ClientTest < Minitest::Test
  E = AgdirLabs::Email

  def test_version_and_user_agent
    assert_match(/\A\d+\.\d+\.\d+\z/, E::VERSION)
    assert_equal "agdirlabs-email-ruby/#{E::VERSION}", E::USER_AGENT
  end

  def test_requires_an_api_key
    with_env("AGDIR_EMAIL_API_KEY" => nil) do
      error = assert_raises(E::ConfigurationError) { E::Client.new }
      assert_includes error.message, "AGDIR_EMAIL_API_KEY"
      assert_includes error.message, "https://app.agdir.email"
    end
  end

  def test_empty_env_values_count_as_unset
    with_env("AGDIR_EMAIL_API_KEY" => "", "AGDIR_EMAIL_BASE_URL" => "") do
      assert_raises(E::ConfigurationError) { E::Client.new }
      assert_equal "https://api.agdir.email", E::Client.new(api_key: "re_x").base_url
    end
  end

  def test_reads_key_and_base_url_from_env
    with_env("AGDIR_EMAIL_API_KEY" => "re_env", "AGDIR_EMAIL_BASE_URL" => "http://localhost:8080/api/") do
      stub = stub_request(:get, "http://localhost:8080/api/emails/abc")
             .with(headers: { "Authorization" => "Bearer re_env" })
             .to_return(json(email_json))
      E::Client.new.emails.get("abc")
      assert_requested stub
    end
  end

  def test_explicit_options_win_over_env
    with_env("AGDIR_EMAIL_BASE_URL" => "http://env.example") do
      assert_equal "https://custom.example", client(base_url: "https://custom.example").base_url
    end
  end

  def test_validates_options
    assert_raises(E::ConfigurationError) { client(max_retries: -1) }
    assert_raises(E::ConfigurationError) { client(timeout: 0) }
  end

  def test_inspect_hides_the_key
    refute_includes client.inspect, "re_test"
  end

  def test_sends_standard_headers
    stub = stub_request(:get, "#{BASE}/emails/abc").with(
      headers: { "Authorization" => "Bearer re_test", "User-Agent" => E::USER_AGENT, "Accept" => "application/json" }
    ).to_return(json(email_json))
    client.emails.get("abc")
    assert_requested stub
  end

  def test_encodes_path_segments
    stub = stub_request(:get, "#{BASE}/emails/a%2Fb%20c").to_return(json(email_json))
    client.emails.get("a/b c")
    assert_requested stub
  end

  def test_api_errors_are_typed
    {
      [401, "missing_api_key"] => E::AuthenticationError,
      [403, "invalid_api_key"] => E::AuthenticationError,
      [401, "restricted_api_key"] => E::AuthenticationError,
      [422, "validation_error"] => E::ValidationError,
      [404, "not_found"] => E::NotFoundError,
      [409, "invalid_idempotent_request"] => E::IdempotencyError,
      [409, "concurrent_idempotent_requests"] => E::IdempotencyError,
      [429, "daily_quota_exceeded"] => E::QuotaExceededError,
      [429, "monthly_quota_exceeded"] => E::QuotaExceededError,
      [418, "something_new"] => E::Error
    }.each do |(status, name), klass|
      stub_request(:get, "#{BASE}/emails/x").to_return(api_error(status, name, "msg #{name}"))
      error = assert_raises(E::Error) { client(max_retries: 0).emails.get("x") }
      assert_instance_of klass, error
      assert_equal name, error.code
      assert_equal status, error.status
      assert_equal "msg #{name}", error.message
    end
  end

  def test_non_json_error_body
    stub_request(:get, "#{BASE}/emails/x").to_return(status: 502, body: "<html>Bad gateway</html>")
    error = assert_raises(E::ServerError) { client(max_retries: 0).emails.get("x") }
    assert_equal "Request failed with HTTP 502", error.message
    assert_equal "internal_server_error", error.code
    assert_equal 502, error.status
  end

  def test_retries_rate_limits_honouring_retry_after
    stub_request(:post, "#{BASE}/domains")
      .to_return(api_error(429, "rate_limit_exceeded", headers: { "retry-after" => "2" }))
      .then.to_return(json(domain_json))
    domain = client.domains.create(name: "acme.com")
    assert_equal "d_1", domain.id
    assert_equal [2.0], TestSupport.pauses
  end

  def test_gives_up_on_rate_limit_after_max_retries
    stub_request(:get, "#{BASE}/emails/x")
      .to_return(api_error(429, "rate_limit_exceeded", headers: { "retry-after" => "1" }))
    error = assert_raises(E::RateLimitError) { client(max_retries: 2).emails.get("x") }
    assert_in_delta 1.0, error.retry_after
    assert_requested :get, "#{BASE}/emails/x", times: 3
  end

  def test_never_retries_quota_errors
    stub_request(:get, "#{BASE}/emails/x").to_return(api_error(429, "daily_quota_exceeded"))
    assert_raises(E::QuotaExceededError) { client.emails.get("x") }
    assert_requested :get, "#{BASE}/emails/x", times: 1
  end

  def test_retries_server_errors_for_retryable_requests_with_backoff
    stub_request(:get, "#{BASE}/emails/x")
      .to_return(api_error(503, "internal_server_error")).times(2)
      .then.to_return(json(email_json))
    assert_equal "abc", client.emails.get("x").id
    assert_equal 2, TestSupport.pauses.size
    assert(TestSupport.pauses[0].between?(0.25, 0.5))
    assert(TestSupport.pauses[1].between?(0.5, 1.0))
  end

  def test_does_not_retry_server_errors_for_non_idempotent_requests
    stub_request(:post, "#{BASE}/domains").to_return(api_error(500, "internal_server_error"))
    assert_raises(E::ServerError) { client.domains.create(name: "acme.com") }
    assert_requested :post, "#{BASE}/domains", times: 1
  end

  def test_network_errors
    stub_request(:get, "#{BASE}/emails/x").to_raise(Errno::ECONNREFUSED).then.to_return(json(email_json))
    assert_equal "abc", client.emails.get("x").id

    stub_request(:post, "#{BASE}/domains").to_raise(Errno::ECONNRESET)
    error = assert_raises(E::ConnectionError) { client.domains.create(name: "acme.com") }
    assert_equal "network_error", error.code
    assert_equal 0, error.status
  end

  def test_timeouts
    stub_request(:get, "#{BASE}/emails/x").to_timeout
    error = assert_raises(E::TimeoutError) { client(max_retries: 1, timeout: 5).emails.get("x") }
    assert_equal "timeout", error.code
    assert_kind_of E::ConnectionError, error
    assert_includes error.message, "5"
    assert_requested :get, "#{BASE}/emails/x", times: 2
  end

  def test_backoff_is_capped
    transport = client.emails.instance_variable_get(:@transport)
    10.times { |n| assert_operator transport.send(:backoff, n), :<=, 8.0 }
  end

  def test_retry_after_http_date
    transport = client.emails.instance_variable_get(:@transport)
    assert_in_delta 30, transport.send(:retry_after, (Time.now + 30).httpdate), 1.5
    assert_nil transport.send(:retry_after, "soon")
  end

  def test_custom_http_client
    calls = []
    http = lambda do |method:, url:, headers:, body:, timeout:|
      calls << [method, url.to_s, headers["Authorization"], body, timeout]
      E::HttpResponse.new(status: 200, headers: {}, body: JSON.generate(email_json))
    end
    client(http_client: http, timeout: 7).emails.get("abc")
    assert_equal [["GET", "#{BASE}/emails/abc", "Bearer re_test", nil, 7]], calls
  end

  private

  def email_json
    { object: "email", id: "abc", from: "a@b.c", to: ["x@y.z"], cc: nil, bcc: nil, reply_to: nil, subject: "s",
      html: "<p>x</p>", text: nil, created_at: "2026-10-30T10:00:00.000Z", scheduled_at: nil, last_event: "queued" }
  end

  def domain_json
    { object: "domain", id: "d_1", name: "acme.com", status: "not_started", created_at: "2026-10-30T10:00:00Z",
      region: "eu-north-1", open_tracking: false, click_tracking: false, records: [] }
  end
end
