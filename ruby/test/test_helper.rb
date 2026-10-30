# frozen_string_literal: true

$LOAD_PATH.unshift File.expand_path("../lib", __dir__)
require "agdirlabs/email"
require "minitest/autorun"
require "webmock/minitest"

module TestSupport
  BASE = "https://api.agdir.email"

  # Unit tests never really sleep between retries; they record the requested pauses instead.
  module RecordedPauses
    def pause(seconds)
      return super if Thread.current[:agdir_real_sleep]

      TestSupport.pauses << seconds
    end
  end

  def self.pauses = (@pauses ||= [])

  def client(**options)
    AgdirLabs::Email::Client.new(api_key: "re_test", **options)
  end

  def json(body, status: 200, headers: {})
    { status: status, body: JSON.generate(body), headers: { "content-type" => "application/json" }.merge(headers) }
  end

  def api_error(status, name, message = "nope", headers: {})
    json({ statusCode: status, name: name, message: message }, status: status, headers: headers)
  end

  def with_env(vars)
    saved = vars.keys.to_h { |k| [k, ENV.fetch(k, nil)] }
    vars.each { |k, v| ENV[k] = v }
    yield
  ensure
    saved.each { |k, v| ENV[k] = v }
  end
end

AgdirLabs::Email::Transport.prepend(TestSupport::RecordedPauses)

module Minitest
  class Test
    include TestSupport

    def setup
      super
      TestSupport.pauses.clear
    end
  end
end
