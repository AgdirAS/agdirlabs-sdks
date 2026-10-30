# frozen_string_literal: true

module AgdirLabs
  module Email
    # A delivery method for the `mail` gem and Rails ActionMailer that sends through the Agdir Email API.
    #
    # Rails (registered by the Railtie):
    #
    #   config.action_mailer.delivery_method = :agdir_email
    #   config.action_mailer.agdir_email_settings = { api_key: ENV["AGDIR_EMAIL_API_KEY"] }
    #
    # Plain `mail` gem:
    #
    #   Mail.defaults { delivery_method AgdirLabs::Email::DeliveryMethod, api_key: "re_…" }
    #
    # Mapped: From, To, Cc, Bcc, Reply-To, Subject, the HTML and text parts, and every `X-…` header.
    # Special headers (removed before sending):
    #
    # - `X-Agdir-Tags: kind=welcome, plan=pro` → `tags`
    # - `X-Agdir-Scheduled-At: 2026-11-01T09:00:00+01:00` → `scheduled_at`
    # - `X-Agdir-Idempotency-Key: welcome/42` → idempotency key
    #
    # After sending, the email id is stored in the message's `X-Agdir-Email-Id` header and returned
    # (Mail returns it from `deliver!` when `return_response: true` is set).
    class DeliveryMethod
      TAGS_HEADER = "X-Agdir-Tags"
      SCHEDULED_AT_HEADER = "X-Agdir-Scheduled-At"
      IDEMPOTENCY_KEY_HEADER = "X-Agdir-Idempotency-Key"
      EMAIL_ID_HEADER = "X-Agdir-Email-Id"
      CONTROL_HEADERS = [TAGS_HEADER, SCHEDULED_AT_HEADER, IDEMPOTENCY_KEY_HEADER, EMAIL_ID_HEADER]
                        .map(&:downcase).freeze
      CLIENT_OPTIONS = %i[api_key base_url max_retries timeout http_client].freeze

      # @return [Hash] the settings this delivery method was created with
      attr_accessor :settings

      # @param settings [Hash] `api_key:`, `base_url:`, `max_retries:`, `timeout:`, `http_client:`
      #   (see {Client#initialize}) or a ready `client:`. Missing values fall back to the environment.
      def initialize(settings = {})
        @settings = (settings || {}).to_h.transform_keys(&:to_sym)
      end

      # Sends a Mail::Message.
      # @param mail [Mail::Message]
      # @return [SendResult]
      # @raise [UnsupportedMessageError] for messages with attachments
      # @raise [Error] when the API rejects the email
      def deliver!(mail)
        params, key = self.class.to_params(mail)
        result = client.emails.send(idempotency_key: key, **params)
        mail[EMAIL_ID_HEADER] = result.id
        result
      end

      # The {Client} used to send (built lazily from {#settings}).
      # @return [Client]
      def client
        @client ||= settings[:client] || Client.new(**settings.slice(*CLIENT_OPTIONS))
      end

      class << self
        # Maps a Mail::Message to keyword arguments for {Resources::Emails#send}.
        # @return [Array(Hash, String)] the email params and the idempotency key (or nil)
        def to_params(mail)
          if mail.has_attachments?
            raise UnsupportedMessageError,
                  "Attachments are not supported by the Agdir Email API yet; remove them or link to the files instead"
          end

          params = {
            from: addresses(mail, :from).first,
            to: addresses(mail, :to),
            cc: addresses(mail, :cc),
            bcc: addresses(mail, :bcc),
            reply_to: addresses(mail, :reply_to),
            subject: mail.subject.to_s,
            html: body(mail, "text/html"),
            text: body(mail, "text/plain"),
            headers: custom_headers(mail),
            tags: tags(mail),
            scheduled_at: header(mail, SCHEDULED_AT_HEADER)
          }
          optional = %i[cc bcc reply_to headers tags]
          params.reject! { |k, v| v.nil? || (optional.include?(k) && v.empty?) }
          [params, header(mail, IDEMPOTENCY_KEY_HEADER)]
        end

        private

        def addresses(mail, field)
          value = mail[field]
          return [] if value.nil?

          formatted = value.respond_to?(:formatted) ? value.formatted : Array(value.to_s)
          formatted.map { |a| utf8(a) }
        end

        def body(mail, mime_type)
          part = if mail.multipart?
                   mime_type == "text/html" ? mail.html_part : mail.text_part
                 elsif (mail.mime_type || "text/plain") == mime_type
                   mail
                 end
          return nil if part.nil?

          text = utf8(part.decoded)
          text.empty? ? nil : text
        end

        def custom_headers(mail)
          mail.header.fields.each_with_object({}) do |field, out|
            name = field.name.to_s
            next unless name.downcase.start_with?("x-") && !CONTROL_HEADERS.include?(name.downcase)

            out[name] = out.key?(name) ? "#{out[name]}, #{utf8(field.value)}" : utf8(field.value)
          end
        end

        def tags(mail)
          values = mail.header.fields.select { |f| f.name.to_s.casecmp?(TAGS_HEADER) }.map { |f| f.value.to_s }
          values.flat_map { |v| v.split(",") }.filter_map do |pair|
            name, value = pair.split("=", 2).map(&:strip)
            next if name.nil? || name.empty?

            { name: name, value: value.to_s }
          end
        end

        def header(mail, name)
          field = mail.header.fields.find { |f| f.name.to_s.casecmp?(name) }
          value = field&.value.to_s.strip
          value.nil? || value.empty? ? nil : value
        end

        def utf8(value)
          string = value.to_s
          string = string.dup.force_encoding(Encoding::UTF_8) if string.encoding == Encoding::BINARY
          string.encode(Encoding::UTF_8, invalid: :replace, undef: :replace)
        end
      end
    end
  end
end
