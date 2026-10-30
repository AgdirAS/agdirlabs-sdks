# frozen_string_literal: true

module AgdirLabs
  module Email
    # A raw HTTP response, as returned by an HTTP client.
    # @!attribute [r] status
    #   @return [Integer]
    # @!attribute [r] headers
    #   @return [Hash{String => String}] lower-case header names
    # @!attribute [r] body
    #   @return [String]
    HttpResponse = Model.define(:status, :headers, :body)

    # The default HTTP client: Net::HTTP, one connection per request, honouring the
    # standard `https_proxy` / `http_proxy` / `no_proxy` environment variables.
    #
    # A custom client is any object with the same `call` signature (see {Client#initialize}).
    class NetHttpClient
      # @param method [String] GET, POST, PATCH or DELETE
      # @param url [URI::HTTP]
      # @param headers [Hash{String => String}]
      # @param body [String, nil]
      # @param timeout [Numeric] seconds, applied to connect, write and read
      # @return [HttpResponse]
      def call(method:, url:, headers:, body:, timeout:)
        http = Net::HTTP.new(url.host, url.port)
        http.use_ssl = url.scheme == "https"
        http.open_timeout = timeout
        http.read_timeout = timeout
        http.write_timeout = timeout
        request = Net::HTTPGenericRequest.new(method, !body.nil?, true, url.request_uri, headers)
        request.body = body if body
        response = http.start { |conn| conn.request(request) }
        HttpResponse.new(
          status: response.code.to_i,
          headers: response.each_header.to_h { |k, v| [k.downcase, v] },
          body: response.body.to_s
        )
      end
    end

    # Sends API requests, retrying rate limits, server errors and network failures when safe.
    # @api private
    class Transport
      RETRY_STATUSES = [408, 500, 502, 503, 504].freeze
      TIMEOUT_ERRORS = [Net::OpenTimeout, Net::ReadTimeout, Net::WriteTimeout, ::Timeout::Error].freeze
      NETWORK_ERRORS = [SocketError, SystemCallError, IOError, EOFError, OpenSSL::SSL::SSLError,
                        Net::HTTPBadResponse, Net::ProtocolError].freeze

      attr_reader :base_url, :max_retries, :timeout

      def initialize(api_key:, base_url:, max_retries:, timeout:, http_client:)
        @api_key = api_key
        @base_url = base_url
        @max_retries = max_retries
        @timeout = timeout
        @http_client = http_client
      end

      # @param retryable [Boolean] safe to repeat: GETs, and POSTs that carry an idempotency key
      # @return [Hash] the parsed JSON response
      def request(method, path, retryable:, query: nil, body: nil, headers: {})
        url = build_url(path, query)
        request_headers = {
          "Authorization" => "Bearer #{@api_key}",
          "User-Agent" => USER_AGENT,
          "Accept" => "application/json"
        }
        request_headers["Content-Type"] = "application/json" unless body.nil?
        headers.each { |k, v| request_headers[k] = v.to_s unless v.nil? }
        payload = body.nil? ? nil : JSON.generate(body)

        attempt = 0
        loop do
          can_retry = attempt < @max_retries
          begin
            response = @http_client.call(method: method, url: url, headers: request_headers, body: payload,
                                         timeout: @timeout)
          rescue *TIMEOUT_ERRORS, *NETWORK_ERRORS => e
            # A timed-out POST may have been accepted: only retry it when it is idempotent.
            if can_retry && retryable
              pause(backoff(attempt))
              attempt += 1
              next
            end
            raise network_error(e)
          end

          return parse(response.body) if response.status.between?(200, 299)

          error = to_error(response)
          # Rate limits are refused before any work is done, so they are always safe to retry.
          retry_it = can_retry && (error.code == "rate_limit_exceeded" ||
                                   (retryable && RETRY_STATUSES.include?(response.status)))
          raise error unless retry_it

          pause(error.retry_after || backoff(attempt))
          attempt += 1
        end
      end

      private

      def build_url(path, query)
        url = URI.parse(@base_url.sub(%r{/+\z}, "") + path)
        params = (query || {}).compact
        url.query = URI.encode_www_form(params) unless params.empty?
        url
      end

      def parse(body)
        return {} if body.nil? || body.strip.empty?

        JSON.parse(body)
      end

      def network_error(cause)
        if TIMEOUT_ERRORS.any? { |k| cause.is_a?(k) }
          TimeoutError.new("Request timed out after #{@timeout} s", code: "timeout")
        else
          ConnectionError.new("Could not reach the Agdir Email API (#{cause.class}: #{cause.message})",
                              code: "network_error")
        end
      end

      def to_error(response)
        json = begin
          parsed = JSON.parse(response.body.to_s)
          parsed.is_a?(Hash) ? parsed : {}
        rescue JSON::ParserError
          {} # Not JSON (e.g. a proxy error page) — fall back to the status line.
        end
        message = json["message"].is_a?(String) ? json["message"] : "Request failed with HTTP #{response.status}"
        code = json["name"].is_a?(String) ? json["name"] : default_code(response.status)
        Error.build(message, code: code, status: response.status,
                             retry_after: retry_after(response.headers["retry-after"]))
      end

      def default_code(status) = status >= 500 ? "internal_server_error" : "unknown_error"

      def retry_after(raw)
        return nil if raw.nil? || raw.strip.empty?

        seconds = Float(raw, exception: false)
        return [seconds, 0.0].max if seconds

        [Time.httpdate(raw) - Time.now, 0.0].max
      rescue ArgumentError
        nil
      end

      # Exponential backoff with jitter: ~0.5 s, 1 s, 2 s … capped at 8 s.
      def backoff(attempt)
        base = [8.0, 0.5 * (2**attempt)].min
        (base / 2) + (rand * (base / 2))
      end

      def pause(seconds) = sleep(seconds)
    end
  end
end
