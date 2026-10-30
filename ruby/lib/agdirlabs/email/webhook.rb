# frozen_string_literal: true

module AgdirLabs
  module Email
    # Verification of signed webhook requests (Standard Webhooks: `webhook-id`,
    # `webhook-timestamp`, `webhook-signature`).
    module Webhook
      # Alias of {AgdirLabs::Email::WebhookVerificationError}.
      VerificationError = WebhookVerificationError

      # Default maximum age (and clock skew) of a message, in seconds.
      DEFAULT_TOLERANCE = 300

      module_function

      # Checks that a webhook request came from Agdir Email and returns the parsed event.
      #
      #   event = AgdirLabs::Email::Webhook.verify(
      #     payload: request.raw_post, headers: request.headers, secret: ENV["AGDIR_EMAIL_WEBHOOK_SECRET"]
      #   )
      #
      # @param payload [String] the raw request body, exactly as received (not re-serialized JSON)
      # @param headers [Hash, #[]] request headers: a Hash (any key case, Rack `HTTP_*` keys work too),
      #   or any object answering `headers["webhook-id"]` (e.g. Rails `request.headers`)
      # @param secret [String] the webhook's `whsec_…` signing secret
      # @param tolerance [Integer] reject messages older or newer than this many seconds
      # @param now [Time] for tests
      # @return [WebhookEvent]
      # @raise [WebhookVerificationError]
      def verify(payload:, headers:, secret:, tolerance: DEFAULT_TOLERANCE, now: Time.now)
        id = header(headers, "webhook-id")
        timestamp = header(headers, "webhook-timestamp")
        signatures = header(headers, "webhook-signature")
        raise WebhookVerificationError, "Missing webhook signature headers" unless id && timestamp && signatures

        sent_at = Integer(timestamp, 10, exception: false)
        if sent_at.nil? || (now.to_i - sent_at).abs > tolerance
          raise WebhookVerificationError, "Webhook timestamp is outside the allowed tolerance"
        end

        expected = sign(secret: secret, id: id, timestamp: timestamp, payload: payload)
        # The header may hold several space-separated `v1,<sig>` entries (e.g. during secret rotation).
        valid = signatures.split.any? do |entry|
          entry.start_with?("v1,") && secure_compare(entry.delete_prefix("v1,"), expected)
        end
        raise WebhookVerificationError, "Webhook signature does not match" unless valid

        begin
          json = JSON.parse(payload)
          raise WebhookVerificationError, "Webhook payload is not a JSON object" unless json.is_a?(Hash)

          Wire.webhook_event(json)
        rescue JSON::ParserError
          raise WebhookVerificationError, "Webhook payload is not valid JSON"
        end
      end

      # Computes the base64 `v1` signature of a message. Useful to sign test requests.
      # @return [String]
      # @raise [WebhookVerificationError] when the secret is not valid base64
      def sign(secret:, id:, timestamp:, payload:)
        encoded = secret.to_s.delete_prefix("whsec_")
        key = begin
          encoded.unpack1("m0")
        rescue ArgumentError
          raise WebhookVerificationError, "Signing secret is not valid base64"
        end
        [OpenSSL::HMAC.digest("SHA256", key, "#{id}.#{timestamp}.#{payload}")].pack("m0")
      end

      # @api private
      def header(headers, name)
        value = if headers.is_a?(Hash)
                  rack = "HTTP_#{name.upcase.tr("-", "_")}"
                  match = headers.find { |k, _| k.to_s.casecmp?(name) || k.to_s == rack }
                  match&.last
                else
                  headers[name]
                end
        value = value.first if value.is_a?(Array)
        value.nil? || value.to_s.empty? ? nil : value.to_s
      end

      # @api private
      def secure_compare(left, right)
        return false unless left.bytesize == right.bytesize

        OpenSSL.fixed_length_secure_compare(left, right)
      end
    end
  end
end
