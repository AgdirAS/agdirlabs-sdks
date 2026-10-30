# frozen_string_literal: true

module AgdirLabs
  module Email
    # Default API base URL.
    DEFAULT_BASE_URL = "https://api.agdir.email"

    # Agdir Email API client. Thread-safe: share one instance.
    #
    #   client = AgdirLabs::Email::Client.new(api_key: "re_…")
    #   client.emails.send(from: "Acme <hi@acme.com>", to: "ana@example.com", subject: "Hi", html: "<p>Hi</p>")
    class Client
      # @return [Resources::Emails]
      attr_reader :emails
      # @return [Resources::Domains]
      attr_reader :domains
      # @return [Resources::Webhooks]
      attr_reader :webhooks
      # @return [String]
      attr_reader :base_url
      # @return [Integer]
      attr_reader :max_retries
      # @return [Numeric]
      attr_reader :timeout

      # @param api_key [String, nil] defaults to `ENV["AGDIR_EMAIL_API_KEY"]`
      # @param base_url [String, nil] defaults to `ENV["AGDIR_EMAIL_BASE_URL"]`, else https://api.agdir.email
      # @param max_retries [Integer] retries for rate limits, server errors and network failures; 0 disables
      # @param timeout [Numeric] seconds per attempt
      # @param http_client [#call, nil] custom HTTP client:
      #   `call(method:, url:, headers:, body:, timeout:)` returning an {HttpResponse}. Defaults to {NetHttpClient}.
      # @raise [ConfigurationError] when no API key is available
      def initialize(api_key: nil, base_url: nil, max_retries: 2, timeout: 30, http_client: nil)
        api_key = present(api_key) || present(ENV.fetch("AGDIR_EMAIL_API_KEY", nil))
        unless api_key
          raise ConfigurationError,
                "Missing API key: pass api_key: or set AGDIR_EMAIL_API_KEY. Create one at https://app.agdir.email."
        end
        unless max_retries.is_a?(Integer) && !max_retries.negative?
          raise ConfigurationError, "max_retries must be a non-negative Integer"
        end
        raise ConfigurationError, "timeout must be a positive number" unless timeout.is_a?(Numeric) && timeout.positive?

        @base_url = (present(base_url) || present(ENV.fetch("AGDIR_EMAIL_BASE_URL", nil)) || DEFAULT_BASE_URL).freeze
        @max_retries = max_retries
        @timeout = timeout
        transport = Transport.new(api_key: api_key, base_url: @base_url, max_retries: max_retries, timeout: timeout,
                                  http_client: http_client || NetHttpClient.new)
        @emails = Resources::Emails.new(transport)
        @domains = Resources::Domains.new(transport)
        @webhooks = Resources::Webhooks.new(transport)
      end

      # Keeps the API key out of logs and consoles.
      def inspect = "#<#{self.class.name} base_url=#{@base_url.inspect}>"

      private

      # Empty values (e.g. `AGDIR_EMAIL_BASE_URL=` in a .env file) count as unset.
      def present(value)
        value.nil? || value.to_s.empty? ? nil : value.to_s
      end
    end
  end
end
