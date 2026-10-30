# frozen_string_literal: true

module AgdirLabs
  module Email
    # Immutable response objects. All are frozen, keyword-initialized Structs
    # (`Data` needs Ruby 3.2; this gem supports 3.1), so they support `==`, `to_h`
    # and pattern matching (`in {status: "delivered"}`).
    module Model
      # Defines a frozen keyword Struct.
      # @api private
      def self.define(*fields, &block)
        Struct.new(*fields, keyword_init: true) do
          define_method(:initialize) do |**attributes|
            super(**attributes)
            freeze
          end
          class_eval(&block) if block
        end
      end
    end

    # Result of {Resources::Emails#send}.
    # @!attribute [r] id
    #   @return [String]
    SendResult = Model.define(:id)

    # Result of {Resources::Emails#send_batch}.
    # @!attribute [r] data
    #   @return [Array<SendResult>] accepted emails, in input order
    # @!attribute [r] errors
    #   @return [Array<BatchError>] only with `validation: :permissive`: emails that were not accepted
    BatchResult = Model.define(:data, :errors)

    # An email rejected by a permissive batch.
    # @!attribute [r] index
    #   @return [Integer] position in the input array
    # @!attribute [r] message
    #   @return [String]
    BatchError = Model.define(:index, :message)

    # Result of operations that only echo the object's id (cancel, verify, update, replay, delete).
    # @!attribute [r] id
    #   @return [String]
    # @!attribute [r] deleted
    #   @return [Boolean] true for delete operations
    Result = Model.define(:id, :deleted) do
      def deleted? = deleted == true
    end

    # An email and its delivery status, from {Resources::Emails#get}.
    # @!attribute [r] status
    #   @return [String] most advanced state so far: queued, scheduled, sent, delivery_delayed,
    #     delivered, opened, clicked, bounced, failed, complained or canceled
    # @!attribute [r] created_at
    #   @return [Time]
    # @!attribute [r] scheduled_at
    #   @return [Time, nil]
    EmailDetails = Model.define(:id, :from, :to, :cc, :bcc, :reply_to, :subject, :html, :text,
                                :created_at, :scheduled_at, :status)

    # A DNS record to publish for a domain.
    # @!attribute [r] purpose
    #   @return [String] what the record is for: DKIM, SPF or DMARC
    # @!attribute [r] name
    #   @return [String] relative to the registrable domain, e.g. `agdir._domainkey` or `send`
    # @!attribute [r] type
    #   @return [String] TXT, MX or CNAME
    # @!attribute [r] priority
    #   @return [Integer, nil] MX records only
    DnsRecord = Model.define(:purpose, :name, :type, :value, :ttl, :priority, :status)

    # A sending domain.
    # @!attribute [r] status
    #   @return [String] not_started, pending, verified, failed or temporary_failure
    # @!attribute [r] records
    #   @return [Array<DnsRecord>, nil] DNS records; nil on items of {Resources::Domains#list}
    Domain = Model.define(:id, :name, :status, :region, :created_at, :open_tracking, :click_tracking, :records) do
      def verified? = status == "verified"
    end

    # A webhook endpoint.
    # @!attribute [r] status
    #   @return [String] enabled or disabled
    # @!attribute [r] events
    #   @return [Array<String>] subscribed event types, e.g. `email.delivered`
    # @!attribute [r] signing_secret
    #   @return [String, nil] `whsec_…`; only from {Resources::Webhooks#get}
    WebhookEndpoint = Model.define(:id, :endpoint, :status, :events, :created_at, :signing_secret) do
      def enabled? = status == "enabled"
    end

    # A webhook id with its (new) signing secret, from webhook create and rotate.
    WebhookSecret = Model.define(:id, :signing_secret)

    # A message sent (or queued) to a webhook endpoint.
    # @!attribute [r] status
    #   @return [String] pending, attempting, success or failed
    # @!attribute [r] next_attempt_at
    #   @return [Time, nil] only from {Resources::WebhookDeliveries#get}
    # @!attribute [r] payload
    #   @return [Hash, nil] the exact JSON body sent; only from {Resources::WebhookDeliveries#get}
    WebhookDelivery = Model.define(:id, :type, :status, :created_at, :next_attempt_at, :payload)

    # One HTTP attempt of a webhook delivery.
    # @!attribute [r] http_status
    #   @return [Integer] 0 when no HTTP response was received
    WebhookAttempt = Model.define(:id, :http_status, :response, :sent_at)

    # An event received on your webhook endpoint, returned by {Webhook.verify}.
    # @!attribute [r] type
    #   @return [String] e.g. `email.delivered`
    # @!attribute [r] data
    #   @return [WebhookEventData]
    WebhookEvent = Model.define(:type, :created_at, :data)

    # The email an event is about.
    # @!attribute [r] tags
    #   @return [Hash{String => String}]
    # @!attribute [r] bounce
    #   @return [Bounce, nil] `email.bounced` only
    # @!attribute [r] click
    #   @return [Click, nil] `email.clicked` only
    # @!attribute [r] failed
    #   @return [Failure, nil] `email.failed` only
    WebhookEventData = Model.define(:email_id, :message_id, :from, :to, :subject, :tags, :created_at,
                                    :bounce, :click, :failed)

    # Bounce details of an `email.bounced` event.
    Bounce = Model.define(:type, :sub_type, :message)
    # Click details of an `email.clicked` event.
    Click = Model.define(:link, :ip_address, :user_agent, :timestamp)
    # Failure details of an `email.failed` event.
    Failure = Model.define(:reason)

    # One page of a list endpoint. Enumerable over {#data}.
    # @!attribute [r] data
    #   @return [Array]
    # @!attribute [r] has_more
    #   @return [Boolean]
    # @!attribute [r] next_cursor
    #   @return [String, nil] pass as `after:` to fetch the next page; nil on the last page
    class Page
      include Enumerable

      attr_reader :data, :has_more, :next_cursor

      def initialize(data:, has_more:, next_cursor:)
        @data = data.freeze
        @has_more = has_more
        @next_cursor = next_cursor
        freeze
      end

      def has_more? = has_more
      def each(&) = data.each(&)
      def size = data.size
      alias length size
      def empty? = data.empty?
      def to_h = { data: data, has_more: has_more, next_cursor: next_cursor }

      def ==(other)
        other.is_a?(Page) && other.to_h == to_h
      end
      alias eql? ==
      def hash = to_h.hash

      def deconstruct_keys(_keys) = to_h

      def inspect = "#<#{self.class.name} size=#{size} has_more=#{has_more} next_cursor=#{next_cursor.inspect}>"
    end
  end
end
