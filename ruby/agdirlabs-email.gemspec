# frozen_string_literal: true

require_relative "lib/agdirlabs/email/version"

Gem::Specification.new do |spec|
  spec.name = "agdirlabs-email"
  spec.version = AgdirLabs::Email::VERSION
  spec.authors = ["Agdir AS"]

  spec.summary = "Official Ruby SDK for Agdir Email — programmatic SMTP for developers, hosted in the EU."
  spec.description = "Send transactional email, manage sending domains and webhooks, and verify signed " \
                     "webhook events with the Agdir Email API. Includes an ActionMailer / mail gem delivery " \
                     "method. No runtime dependencies."
  spec.homepage = "https://agdir.email"
  spec.license = "MIT"
  spec.required_ruby_version = ">= 3.1"

  spec.metadata = {
    "homepage_uri" => spec.homepage,
    "source_code_uri" => "https://github.com/AgdirAS/agdirlabs-sdks/tree/main/ruby",
    "changelog_uri" => "https://github.com/AgdirAS/agdirlabs-sdks/blob/main/ruby/CHANGELOG.md",
    "bug_tracker_uri" => "https://github.com/AgdirAS/agdirlabs-sdks/issues",
    "documentation_uri" => "https://github.com/AgdirAS/agdirlabs-sdks/tree/main/ruby#readme",
    "rubygems_mfa_required" => "true"
  }

  spec.files = Dir["lib/**/*.rb", "sig/**/*.rbs", "README.md", "CHANGELOG.md", "LICENSE"]
  spec.require_paths = ["lib"]
end
