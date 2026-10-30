# frozen_string_literal: true

module AgdirLabs
  module Email
    module Resources
      # `client.webhooks.deliveries`: messages sent (or queued) to one webhook endpoint, with their HTTP attempts.
      class WebhookDeliveries
        # @api private
        def initialize(transport)
          @transport = transport
        end

        # @param webhook_id [String]
        # @param limit [Integer, nil] 1–100
        # @param after [String, nil]
        # @return [Page<WebhookDelivery>]
        # @raise [Error]
        def list(webhook_id, limit: nil, after: nil)
          res = @transport.request("GET", "/webhooks/#{Resources.segment(webhook_id)}/events",
                                   query: { limit: limit, after: after }, retryable: true)
          Wire.page(res) { |e| Wire.delivery(e) }
        end

        # @return [Enumerator<WebhookDelivery>]
        def list_all(webhook_id, limit: nil, after: nil)
          Resources.paginate(limit: limit, after: after) { |l, a| list(webhook_id, limit: l, after: a) }
        end

        # One delivery with its `payload` and `next_attempt_at`.
        # @return [WebhookDelivery]
        # @raise [Error]
        def get(webhook_id, delivery_id)
          Wire.delivery(@transport.request("GET", path(webhook_id, delivery_id), retryable: true))
        end

        # HTTP attempts for one delivery, newest first.
        # @return [Page<WebhookAttempt>]
        # @raise [Error]
        def attempts(webhook_id, delivery_id, limit: nil, after: nil)
          res = @transport.request("GET", "#{path(webhook_id, delivery_id)}/attempts",
                                   query: { limit: limit, after: after }, retryable: true)
          Wire.page(res) { |a| Wire.attempt(a) }
        end

        # Sends the identical message again, once, right now (also to disabled endpoints).
        # @return [Result]
        # @raise [Error]
        def replay(webhook_id, delivery_id)
          res = @transport.request("POST", "#{path(webhook_id, delivery_id)}/replay", retryable: false)
          Result.new(id: Wire.str(res["id"]), deleted: false)
        end

        private

        def path(webhook_id, delivery_id)
          "/webhooks/#{Resources.segment(webhook_id)}/events/#{Resources.segment(delivery_id)}"
        end
      end

      # `client.webhooks`
      class Webhooks
        # Event types a webhook can subscribe to.
        EVENT_TYPES = %w[
          email.sent email.scheduled email.delivered email.delivery_delayed email.complained
          email.bounced email.opened email.clicked email.failed
        ].freeze

        # @return [WebhookDeliveries]
        attr_reader :deliveries

        # @api private
        def initialize(transport)
          @transport = transport
          @deliveries = WebhookDeliveries.new(transport)
        end

        # Registers an endpoint. Keep the returned `signing_secret` to verify incoming events.
        # @param endpoint [String] public `https://` URL
        # @param events [Array<String>] see {EVENT_TYPES}
        # @return [WebhookSecret]
        # @raise [Error]
        def create(endpoint:, events:)
          res = @transport.request("POST", "/webhooks", body: { endpoint: endpoint, events: Array(events) },
                                                        retryable: false)
          WebhookSecret.new(id: Wire.str(res["id"]), signing_secret: Wire.str(res["signing_secret"]))
        end

        # @return [WebhookEndpoint] including `signing_secret`
        # @raise [Error]
        def get(id)
          Wire.webhook(@transport.request("GET", "/webhooks/#{Resources.segment(id)}", retryable: true))
        end

        # @return [Page<WebhookEndpoint>]
        # @raise [Error]
        def list(limit: nil, after: nil)
          res = @transport.request("GET", "/webhooks", query: { limit: limit, after: after }, retryable: true)
          Wire.page(res) { |w| Wire.webhook(w) }
        end

        # @return [Enumerator<WebhookEndpoint>]
        def list_all(limit: nil, after: nil)
          Resources.paginate(limit: limit, after: after) { |l, a| list(limit: l, after: a) }
        end

        # @param endpoint [String, nil]
        # @param events [Array<String>, nil]
        # @param status [String, Symbol, nil] `enabled` or `disabled`
        # @return [Result]
        # @raise [Error]
        def update(id, endpoint: nil, events: nil, status: nil)
          body = { endpoint: endpoint, events: events && Array(events), status: status&.to_s }.compact
          res = @transport.request("PATCH", "/webhooks/#{Resources.segment(id)}", body: body, retryable: true)
          Result.new(id: Wire.str(res["id"]), deleted: false)
        end

        # @return [Result] with `deleted: true`
        # @raise [Error]
        def delete(id)
          res = @transport.request("DELETE", "/webhooks/#{Resources.segment(id)}", retryable: false)
          Result.new(id: Wire.str(res["id"]), deleted: true)
        end

        # Issues a new signing secret; the old one stops working immediately.
        # @return [WebhookSecret]
        # @raise [Error]
        def rotate_secret(id)
          res = @transport.request("POST", "/webhooks/#{Resources.segment(id)}/signing-secret/rotate",
                                   retryable: false)
          WebhookSecret.new(id: Wire.str(res["id"]), signing_secret: Wire.str(res["signing_secret"]))
        end
      end
    end
  end
end
