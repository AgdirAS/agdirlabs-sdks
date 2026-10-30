# frozen_string_literal: true

module AgdirLabs
  module Email
    module Resources
      # `client.domains`
      class Domains
        # @api private
        def initialize(transport)
          @transport = transport
        end

        # Adds a sending domain. Publish the returned `records` in DNS, then call {#verify}.
        # @param name [String] e.g. `"acme.com"` or `"mail.acme.com"`
        # @param region [String, nil]
        # @return [Domain] with `records`
        # @raise [Error]
        def create(name:, region: nil)
          Wire.domain(@transport.request("POST", "/domains", body: { name: name, region: region }.compact,
                                                             retryable: false))
        end

        # @param id [String]
        # @return [Domain] with `records`
        # @raise [Error]
        def get(id)
          Wire.domain(@transport.request("GET", "/domains/#{Resources.segment(id)}", retryable: true))
        end

        # One page of domains.
        # @param limit [Integer, nil] 1–100
        # @param after [String, nil] cursor: {Page#next_cursor} of the previous page
        # @return [Page<Domain>] items without `records`
        # @raise [Error]
        def list(limit: nil, after: nil)
          res = @transport.request("GET", "/domains", query: { limit: limit, after: after }, retryable: true)
          Wire.page(res) { |d| Wire.domain(d.except("records")) }
        end

        # Every domain, fetching further pages lazily as you iterate.
        # @return [Enumerator<Domain>]
        def list_all(limit: nil, after: nil)
          Resources.paginate(limit: limit, after: after) { |l, a| list(limit: l, after: a) }
        end

        # Starts a DNS check now instead of waiting for the next automatic one.
        # @param id [String]
        # @return [Result]
        # @raise [Error]
        def verify(id)
          res = @transport.request("POST", "/domains/#{Resources.segment(id)}/verify", retryable: true)
          Result.new(id: Wire.str(res["id"]), deleted: false)
        end

        # @param id [String]
        # @return [Result] with `deleted: true`
        # @raise [Error]
        def delete(id)
          res = @transport.request("DELETE", "/domains/#{Resources.segment(id)}", retryable: false)
          Result.new(id: Wire.str(res["id"]), deleted: true)
        end
      end
    end
  end
end
