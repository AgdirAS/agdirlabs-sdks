# frozen_string_literal: true

require "json"
require "net/http"
require "openssl"
require "securerandom"
require "time"
require "uri"

module AgdirLabs
  # The official Ruby SDK for Agdir Email (https://agdir.email).
  #
  #   client = AgdirLabs::Email::Client.new(api_key: "re_…")
  #   client.emails.send(from: "Acme <hi@acme.com>", to: "ana@example.com", subject: "Hi", html: "<p>Hi</p>")
  module Email
  end
end

require_relative "email/version"
require_relative "email/errors"
require_relative "email/models"
require_relative "email/wire"
require_relative "email/transport"
require_relative "email/resources/emails"
require_relative "email/resources/domains"
require_relative "email/resources/webhooks"
require_relative "email/client"
require_relative "email/webhook"
require_relative "email/delivery_method"
require_relative "email/railtie" if defined?(Rails::Railtie)
