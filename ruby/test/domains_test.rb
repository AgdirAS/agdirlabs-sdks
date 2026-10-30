# frozen_string_literal: true

require "test_helper"

class DomainsTest < Minitest::Test
  E = AgdirLabs::Email

  def domain_json(id = "d_1", name = "acme.com", records: true)
    json = { object: "domain", id: id, name: name, status: "pending", created_at: "2026-10-30T10:00:00Z",
             region: "eu-north-1", capabilities: { sending: "enabled", receiving: "disabled" },
             open_tracking: false, click_tracking: true }
    return json unless records

    json.merge(records: [
                 { record: "DKIM", name: "agdir._domainkey", type: "TXT", ttl: "Auto", status: "pending",
                   value: "p=MIG" },
                 { record: "SPF", name: "send", type: "MX", ttl: "Auto", status: "pending",
                   value: "feedback-smtp.eu-north-1.amazonses.com", priority: 10 }
               ])
  end

  def test_create
    stub = stub_request(:post, "#{BASE}/domains").with(body: { name: "acme.com" }).to_return(json(domain_json))
    domain = client.domains.create(name: "acme.com")
    assert_requested stub
    assert_equal "pending", domain.status
    refute_predicate domain, :verified?
    assert domain.click_tracking
    assert_equal %w[DKIM SPF], domain.records.map(&:purpose)
    assert_equal 10, domain.records[1].priority
    assert_nil domain.records[0].priority
    assert_equal "agdir._domainkey", domain.records[0].name
  end

  def test_create_with_region
    stub = stub_request(:post, "#{BASE}/domains").with(body: { name: "acme.com", region: "eu-north-1" })
                                                 .to_return(json(domain_json))
    client.domains.create(name: "acme.com", region: "eu-north-1")
    assert_requested stub
  end

  def test_get_verify_delete
    stub_request(:get, "#{BASE}/domains/d_1").to_return(json(domain_json))
    stub_request(:post, "#{BASE}/domains/d_1/verify").to_return(json({ object: "domain", id: "d_1" }))
    stub_request(:delete, "#{BASE}/domains/d_1").to_return(json({ object: "domain", id: "d_1", deleted: true }))

    assert_equal 2, client.domains.get("d_1").records.size
    assert_equal E::Result.new(id: "d_1", deleted: false), client.domains.verify("d_1")
    deleted = client.domains.delete("d_1")
    assert_predicate deleted, :deleted?
  end

  def test_list_page
    stub_request(:get, "#{BASE}/domains?limit=2")
      .to_return(json({ object: "list", has_more: true, data: [domain_json("d_1"), domain_json("d_2")] }))
    page = client.domains.list(limit: 2)
    assert_predicate page, :has_more?
    assert_equal "d_2", page.next_cursor
    assert_equal %w[d_1 d_2], page.map(&:id)
    assert_equal 2, page.size
    assert_nil page.first.records
    assert_predicate page, :frozen?
  end

  def test_last_page_has_no_cursor
    stub_request(:get, "#{BASE}/domains").to_return(json({ object: "list", has_more: false, data: [domain_json] }))
    assert_nil client.domains.list.next_cursor
  end

  def test_list_all_pages_lazily
    stub_request(:get, "#{BASE}/domains?limit=1")
      .to_return(json({ object: "list", has_more: true, data: [domain_json("d_1", records: false)] }))
    stub_request(:get, "#{BASE}/domains?limit=1&after=d_1")
      .to_return(json({ object: "list", has_more: true, data: [domain_json("d_2", records: false)] }))
    stub_request(:get, "#{BASE}/domains?limit=1&after=d_2")
      .to_return(json({ object: "list", has_more: false, data: [domain_json("d_3", records: false)] }))

    enum = client.domains.list_all(limit: 1)
    assert_kind_of Enumerator, enum
    assert_not_requested :get, "#{BASE}/domains?limit=1"

    assert_equal "d_1", enum.first.id
    assert_not_requested :get, "#{BASE}/domains?limit=1&after=d_1"

    assert_equal %w[d_1 d_2 d_3], enum.map(&:id)
  end
end
