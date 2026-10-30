# frozen_string_literal: true

module AgdirLabs
  module Email
    # Registers the `:agdir_email` ActionMailer delivery method, configured with
    # `config.action_mailer.agdir_email_settings = { api_key: … }`.
    class Railtie < ::Rails::Railtie
      # Must run before ActionMailer copies `config.action_mailer.*` onto ActionMailer::Base,
      # so that `agdir_email_settings=` exists by then.
      initializer "agdirlabs_email.add_delivery_method", before: "action_mailer.set_configs" do
        ActiveSupport.on_load(:action_mailer) do
          add_delivery_method :agdir_email, AgdirLabs::Email::DeliveryMethod
        end
      end
    end
  end
end
