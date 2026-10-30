# frozen_string_literal: true

module AgdirLabs
  module Email
    # Mapping between the SDK's Ruby objects and the API's JSON.
    # @api private
    module Wire
      module_function

      def time(value) = Time.iso8601(value.to_s)
      def optional_time(value) = value.nil? || value == "" ? nil : time(value)
      def list(value) = (value.nil? ? [] : Array(value)).map { |v| str(v) }.freeze
      def str(value) = value.nil? ? nil : value.to_s.dup.freeze

      def deep_freeze(value)
        case value
        when Hash then value.each_value { |v| deep_freeze(v) }.freeze
        when Array then value.each { |v| deep_freeze(v) }.freeze
        else value.freeze
        end
      end

      # Builds a `POST /emails` body from keyword arguments (snake_case on both sides).
      def send_email_body(params)
        params = params.transform_keys(&:to_sym)
        unknown = params.keys - SEND_KEYS
        raise ArgumentError, "unknown email field(s): #{unknown.join(", ")}" unless unknown.empty?

        scheduled_at = params[:scheduled_at]
        scheduled_at = scheduled_at.iso8601 if scheduled_at.respond_to?(:iso8601) && !scheduled_at.is_a?(String)
        tags = params[:tags]
        tags = tags.map { |name, value| { name: name.to_s, value: value.to_s } } if tags.is_a?(Hash)
        {
          from: params[:from],
          to: params[:to],
          subject: params[:subject],
          html: params[:html],
          text: params[:text],
          cc: params[:cc],
          bcc: params[:bcc],
          reply_to: params[:reply_to],
          headers: params[:headers]&.to_h { |k, v| [k.to_s, v.to_s] },
          tags: tags&.map { |t| { name: (t[:name] || t["name"]).to_s, value: (t[:value] || t["value"]).to_s } },
          scheduled_at: scheduled_at
        }.compact
      end

      SEND_KEYS = %i[from to subject html text cc bcc reply_to headers tags scheduled_at].freeze

      def email(json)
        EmailDetails.new(
          id: str(json["id"]),
          from: str(json["from"]),
          to: list(json["to"]),
          cc: list(json["cc"]),
          bcc: list(json["bcc"]),
          reply_to: list(json["reply_to"]),
          subject: str(json["subject"]),
          html: str(json["html"]),
          text: str(json["text"]),
          created_at: time(json["created_at"]),
          scheduled_at: optional_time(json["scheduled_at"]),
          status: str(json["last_event"])
        )
      end

      def record(json)
        DnsRecord.new(
          purpose: str(json["record"]),
          name: str(json["name"]),
          type: str(json["type"]),
          value: str(json["value"]),
          ttl: str(json["ttl"]),
          priority: json["priority"],
          status: str(json["status"])
        )
      end

      def domain(json)
        Domain.new(
          id: str(json["id"]),
          name: str(json["name"]),
          status: str(json["status"]),
          region: str(json["region"]),
          created_at: time(json["created_at"]),
          open_tracking: json["open_tracking"] ? true : false,
          click_tracking: json["click_tracking"] ? true : false,
          records: json.key?("records") ? (json["records"] || []).map { |r| record(r) }.freeze : nil
        )
      end

      def webhook(json)
        WebhookEndpoint.new(
          id: str(json["id"]),
          endpoint: str(json["endpoint"]),
          status: str(json["status"]),
          events: list(json["events"]),
          created_at: time(json["created_at"]),
          signing_secret: str(json["signing_secret"])
        )
      end

      def delivery(json)
        WebhookDelivery.new(
          id: str(json["id"]),
          type: str(json["type"]),
          status: str(json["status"]),
          created_at: time(json["created_at"]),
          next_attempt_at: optional_time(json["next_attempt_at"]),
          payload: deep_freeze(json["payload"])
        )
      end

      def attempt(json)
        WebhookAttempt.new(
          id: str(json["id"]),
          http_status: json["http_status_code"].to_i,
          response: str(json["response"]),
          sent_at: time(json["sent_at"])
        )
      end

      def page(json, &)
        items = json["data"] || []
        has_more = json["has_more"] ? true : false
        last = items.last
        Page.new(
          data: items.map(&),
          has_more: has_more,
          next_cursor: has_more && last ? last["id"].to_s.freeze : nil
        )
      end

      def webhook_event(json)
        d = json["data"] || {}
        WebhookEvent.new(
          type: str(json["type"]),
          created_at: time(json["created_at"]),
          data: WebhookEventData.new(
            email_id: str(d["email_id"]),
            message_id: str(d["message_id"]),
            from: str(d["from"]),
            to: list(d["to"]),
            subject: str(d["subject"]),
            tags: deep_freeze((d["tags"] || {}).to_h { |k, v| [k.to_s, v.to_s] }),
            created_at: time(d["created_at"]),
            bounce: d["bounce"] && bounce(d["bounce"]),
            click: d["click"] && click(d["click"]),
            failed: d["failed"] && Failure.new(reason: str(d["failed"]["reason"]))
          )
        )
      end

      def bounce(json)
        Bounce.new(type: str(json["type"]), sub_type: str(json["subType"]), message: str(json["message"]))
      end

      def click(json)
        Click.new(link: str(json["link"]), ip_address: str(json["ipAddress"]), user_agent: str(json["userAgent"]),
                  timestamp: optional_time(json["timestamp"]))
      end
    end
  end
end
