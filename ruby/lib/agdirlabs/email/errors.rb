# frozen_string_literal: true

module AgdirLabs
  module Email
    # Raised by every client method when a request fails.
    #
    # Rescue this class for everything, or one of its subclasses for a family of errors.
    # {#code} is the API's machine-readable error name (e.g. `"validation_error"`), or
    # `"network_error"` / `"timeout"` when no response was received.
    class Error < StandardError
      # @return [String] machine-readable code, e.g. `"validation_error"` or `"rate_limit_exceeded"`
      attr_reader :code
      # @return [Integer] HTTP status, or 0 when no response was received
      attr_reader :status
      # @return [Float, nil] seconds the API asked us to wait (from `retry-after`), if any
      attr_reader :retry_after

      # @param message [String]
      # @param code [String]
      # @param status [Integer]
      # @param retry_after [Float, nil]
      def initialize(message = nil, code: "unknown_error", status: 0, retry_after: nil)
        super(message)
        @code = code
        @status = status
        @retry_after = retry_after
      end

      # Builds the most specific subclass for an API error code / HTTP status.
      # @return [Error]
      def self.build(message, code:, status:, retry_after: nil)
        klass = CODES.fetch(code) do
          if status >= 500 then ServerError
          elsif status == 409 then IdempotencyError
          else Error
          end
        end
        klass.new(message, code: code, status: status, retry_after: retry_after)
      end
    end

    # `missing_api_key`, `invalid_api_key`, `restricted_api_key`.
    class AuthenticationError < Error; end
    # `validation_error` (422; 403 for an unverified domain, paused team or domain limit).
    class ValidationError < Error; end
    # `not_found`.
    class NotFoundError < Error; end
    # `invalid_idempotent_request`, `concurrent_idempotent_requests`.
    class IdempotencyError < Error; end
    # `rate_limit_exceeded`: retried automatically; raised once retries are exhausted.
    class RateLimitError < Error; end
    # `daily_quota_exceeded`, `monthly_quota_exceeded`: never retried.
    class QuotaExceededError < Error; end
    # 5xx responses.
    class ServerError < Error; end
    # The API could not be reached (`network_error`).
    class ConnectionError < Error; end
    # An attempt took longer than the configured timeout (`timeout`).
    class TimeoutError < ConnectionError; end

    class Error
      CODES = {
        "missing_api_key" => AuthenticationError,
        "invalid_api_key" => AuthenticationError,
        "restricted_api_key" => AuthenticationError,
        "validation_error" => ValidationError,
        "not_found" => NotFoundError,
        "invalid_idempotent_request" => IdempotencyError,
        "concurrent_idempotent_requests" => IdempotencyError,
        "rate_limit_exceeded" => RateLimitError,
        "daily_quota_exceeded" => QuotaExceededError,
        "monthly_quota_exceeded" => QuotaExceededError,
        "internal_server_error" => ServerError,
        "network_error" => ConnectionError,
        "timeout" => TimeoutError
      }.freeze
    end

    # Raised when the client is misconfigured, e.g. no API key is available.
    class ConfigurationError < ArgumentError; end

    # Raised by {Webhook.verify} when a payload's signature or timestamp is invalid.
    class WebhookVerificationError < StandardError; end

    # Raised by {DeliveryMethod} for a message the API cannot send (e.g. one with attachments).
    class UnsupportedMessageError < ArgumentError; end
  end
end
