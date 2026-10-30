# frozen_string_literal: true

module AgdirLabs
  module Email
    # API resources, reached through a {Client}.
    module Resources
      # Percent-encodes one path segment.
      # @api private
      def self.segment(value)
        value.to_s.b.gsub(/[^A-Za-z0-9\-._~]/n) { |c| format("%%%02X", c.ord) }
      end

      # Walks every page of a list endpoint lazily.
      # @api private
      def self.paginate(limit:, after:)
        Enumerator.new do |yielder|
          cursor = after
          loop do
            page = yield(limit, cursor)
            page.each { |item| yielder << item }
            break unless page.next_cursor

            cursor = page.next_cursor
          end
        end
      end

      # `client.emails`
      class Emails
        # Most emails one {#send_batch} call accepts.
        MAX_BATCH_SIZE = 100

        # @api private
        def initialize(transport)
          @transport = transport
        end

        # Accepts one email for delivery (or for `scheduled_at`).
        #
        # Note: this shadows `Object#send` on this object only; use `public_send`/`__send__` for dynamic dispatch.
        #
        # @param from [String] `"Name <addr@example.com>"` or a bare address
        # @param to [String, Array<String>]
        # @param subject [String]
        # @param html [String, nil] at least one of `html` and `text` is required
        # @param text [String, nil]
        # @param cc [String, Array<String>, nil]
        # @param bcc [String, Array<String>, nil]
        # @param reply_to [String, Array<String>, nil]
        # @param headers [Hash{String => String}, nil] custom headers, e.g. `{"X-Entity-Ref-ID" => "42"}`
        # @param tags [Array<Hash>, Hash, nil] `[{name: "kind", value: "welcome"}]` or `{kind: "welcome"}`;
        #   ASCII letters, digits, `_` and `-` only. Echoed back in webhook events.
        # @param scheduled_at [Time, String, nil] deliver later (a Time, or ISO 8601 with offset)
        # @param idempotency_key [String, nil] the same key with the same body returns the original result;
        #   generated automatically when retries are enabled
        # @return [SendResult]
        # @raise [Error]
        def send(idempotency_key: nil, **email)
          key = idempotency_key(idempotency_key)
          res = @transport.request("POST", "/emails", body: Wire.send_email_body(email),
                                                      headers: { "Idempotency-Key" => key }, retryable: !key.nil?)
          SendResult.new(id: Wire.str(res["id"]))
        end

        # Accepts up to 100 emails in one request.
        #
        # @param emails [Array<Hash>] each with the keyword arguments of {#send} (minus `idempotency_key`)
        # @param validation [Symbol, String] `:strict` (default): one invalid email rejects the whole batch;
        #   `:permissive`: valid emails are accepted and invalid ones are reported in {BatchResult#errors}
        # @param idempotency_key [String, nil]
        # @return [BatchResult]
        # @raise [Error]
        def send_batch(emails, validation: :strict, idempotency_key: nil)
          mode = validation.to_s
          raise ArgumentError, "validation must be :strict or :permissive" unless %w[strict permissive].include?(mode)

          key = idempotency_key(idempotency_key)
          res = @transport.request(
            "POST", "/emails/batch",
            body: emails.map { |e| Wire.send_email_body(e) },
            headers: { "Idempotency-Key" => key, "x-batch-validation" => mode == "permissive" ? mode : nil },
            retryable: !key.nil?
          )
          BatchResult.new(
            data: (res["data"] || []).map { |d| SendResult.new(id: Wire.str(d["id"])) }.freeze,
            errors: (res["errors"] || []).map { |e| BatchError.new(index: e["index"], message: Wire.str(e["message"])) }
                                         .freeze
          )
        end

        # Reads an email and its current delivery status. Requires a full-access key.
        # @param id [String]
        # @return [EmailDetails]
        # @raise [Error]
        def get(id)
          Wire.email(@transport.request("GET", "/emails/#{Resources.segment(id)}", retryable: true))
        end

        # Cancels a scheduled email that has not been sent yet.
        # @param id [String]
        # @return [Result]
        # @raise [Error]
        def cancel(id)
          res = @transport.request("POST", "/emails/#{Resources.segment(id)}/cancel", retryable: true)
          Result.new(id: Wire.str(res["id"]), deleted: false)
        end

        private

        def idempotency_key(given)
          return given.to_s unless given.nil? || given.to_s.empty?

          # Without a key a retried send could be delivered twice, so only generate one when we may retry.
          @transport.max_retries.positive? ? SecureRandom.uuid : nil
        end
      end
    end
  end
end
