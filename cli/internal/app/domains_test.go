package app

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	agdiremail "github.com/AgdirAS/agdirlabs-sdks/go"
)

// fakeResolver answers from maps keyed by FQDN; missing names are NXDOMAIN.
type fakeResolver struct {
	txt     map[string][]string
	mx      map[string][]*net.MX
	cname   map[string]string
	fail    error
	queries []string
}

func notFound(name string) error {
	return &net.DNSError{Err: "no such host", Name: name, IsNotFound: true}
}

func (r *fakeResolver) LookupTXT(_ context.Context, name string) ([]string, error) {
	r.queries = append(r.queries, "TXT "+name)
	if r.fail != nil {
		return nil, r.fail
	}
	if v, ok := r.txt[name]; ok {
		return v, nil
	}
	return nil, notFound(name)
}

func (r *fakeResolver) LookupMX(_ context.Context, name string) ([]*net.MX, error) {
	r.queries = append(r.queries, "MX "+name)
	if r.fail != nil {
		return nil, r.fail
	}
	if v, ok := r.mx[name]; ok {
		return v, nil
	}
	return nil, notFound(name)
}

func (r *fakeResolver) LookupCNAME(_ context.Context, name string) (string, error) {
	r.queries = append(r.queries, "CNAME "+name)
	if v, ok := r.cname[name]; ok {
		return v, nil
	}
	return "", notFound(name)
}

// healthyDNS publishes every record for a domain whose records live under apex+sub.
func healthyDNS(sub, apex string) *fakeResolver {
	s := ""
	if sub != "" {
		s = "." + sub
	}
	return &fakeResolver{
		txt: map[string][]string{
			"agdir._domainkey" + s + "." + apex: {"v=DKIM1; k=rsa; " + dkimValue},
			"send" + s + "." + apex:             {"v=spf1 include:amazonses.com include:_spf.google.com ~all"},
			"_dmarc" + s + "." + apex:           {"v=DMARC1; p=quarantine; rua=mailto:dmarc@" + apex},
		},
		mx: map[string][]*net.MX{
			"send" + s + "." + apex: {{Host: "feedback-smtp.eu-north-1.amazonses.com.", Pref: 10}},
		},
	}
}

func TestDomainsAddPrintsRecords(t *testing.T) {
	e := newEnv(t).withKey(keyFull)
	r := e.run("domains", "add", "acme.com").expect(t, 0)
	r.contains(t, "Added acme.com", "PURPOSE", "DKIM", "agdir._domainkey", dkimValue, "MX", "feedback-smtp.eu-north-1.amazonses.com", "10",
		"v=spf1 include:amazonses.com ~all", "_dmarc", "domains check acme.com")

	m := e.run("domains", "add", "mail.acme.com", "--json").expect(t, 0).json(t)
	recs := m["records"].([]any)
	if len(recs) != 4 || recs[0].(map[string]any)["purpose"] != "DKIM" || recs[1].(map[string]any)["name"] != "send.mail" {
		t.Fatalf("records = %v", recs)
	}
	if id := strings.TrimSpace(e.run("domains", "add", "x.io", "-q").expect(t, 0).stdout); !strings.HasPrefix(id, "d-") {
		t.Fatalf("quiet = %q", id)
	}
	e.run("domains", "add", "nodot").expect(t, ExitError).errContains(t, "validation_error")
	e.run("domains", "add").expect(t, ExitUsage).errContains(t, "accepts 1 arg(s)")
}

func TestDomainsListGetVerifyDelete(t *testing.T) {
	e := newEnv(t).withKey(keyFull)
	for _, n := range []string{"a.com", "b.com", "c.com"} {
		e.api.addDomain(n)
	}
	e.run("domains", "list").expect(t, 0).contains(t, "ID", "a.com", "b.com", "c.com", "not_started", "eu-north-1")

	r := e.run("domains", "list", "--limit", "2").expect(t, 0).contains(t, "a.com", "b.com", "More results: agdir-email domains list --limit 2 --after d-0002")
	if strings.Contains(r.stdout, "c.com") {
		t.Fatal("--limit ignored")
	}
	m := e.run("domains", "list", "--limit", "2", "--json").expect(t, 0).json(t)
	if m["has_more"] != true || m["next_cursor"] != "d-0002-0000-4000-8000-000000000000" || len(m["data"].([]any)) != 2 {
		t.Fatalf("json = %v", m)
	}
	m = e.run("domains", "list", "--json").expect(t, 0).json(t)
	if m["has_more"] != false || len(m["data"].([]any)) != 3 {
		t.Fatalf("json = %v", m)
	}
	e.run("domains", "list", "--limit", "500").expect(t, ExitUsage)

	// Names are resolved to IDs.
	e.run("domains", "get", "B.com").expect(t, 0).contains(t, "ID:", "d-0002", "Status:", "STATUS", "agdir._domainkey")
	e.run("domains", "verify", "b.com").expect(t, 0).contains(t, "Verification started")
	if e.api.count("POST", "/domains/d-0002-0000-4000-8000-000000000000/verify") != 1 {
		t.Fatal("verify not called with the ID")
	}
	e.run("domains", "get", "nope.com").expect(t, ExitError).errContains(t, "no domain named nope.com")

	e.run("domains", "delete", "a.com").expect(t, ExitUsage).errContains(t, "pass --yes")
	e.tty, e.stdin = true, "n\n"
	e.run("domains", "delete", "a.com").expect(t, ExitError).errContains(t, "aborted")
	e.stdin = "y\n"
	e.run("domains", "delete", "a.com").expect(t, 0).contains(t, "Deleted domain a.com")
	e.tty = false
	m = e.run("domains", "delete", "d-0003-0000-4000-8000-000000000000", "--yes", "--json").expect(t, 0).json(t)
	if m["deleted"] != true {
		t.Fatalf("json = %v", m)
	}
	if got := e.api.domainNames(); len(got) != 1 || got[0] != "b.com" {
		t.Fatalf("domains left: %v", got)
	}
}

func TestDomainsCheckAllGood(t *testing.T) {
	e := newEnv(t).withKey(keyFull)
	e.api.addDomain("acme.com")
	e.resolver = healthyDNS("", "acme.com")
	r := e.run("domains", "check", "acme.com").expect(t, 0)
	r.contains(t, "acme.com (status: not_started)", "✓  DKIM", "agdir._domainkey.acme.com", "✓  SPF", "send.acme.com", "✓  DMARC (recommended)",
		"All required records are published", "domains verify acme.com")
	if strings.Contains(r.stdout, "✗") {
		t.Fatalf("unexpected failure:\n%s", r.stdout)
	}
}

func TestDomainsCheckReportsProblems(t *testing.T) {
	e := newEnv(t).withKey(keyFull)
	e.api.addDomain("acme.com")
	res := healthyDNS("", "acme.com")
	delete(res.txt, "agdir._domainkey.acme.com")
	res.txt["send.acme.com"] = []string{"v=spf1 include:_spf.google.com ~all"}
	res.mx["send.acme.com"] = []*net.MX{{Host: "mx.other.net.", Pref: 5}}
	delete(res.txt, "_dmarc.acme.com")
	e.resolver = res

	r := e.run("domains", "check", "acme.com").expect(t, ExitError)
	r.contains(t,
		"✗  DKIM", "no TXT record found at agdir._domainkey.acme.com",
		"MX record points elsewhere; found: 5 mx.other.net; expected: feedback-smtp.eu-north-1.amazonses.com",
		"SPF record does not contain include:amazonses.com",
		"!  DMARC (recommended)",
		"3 required record(s) missing or wrong")

	m := e.run("domains", "check", "acme.com", "--json").expect(t, ExitError).json(t)
	if m["ok"] != false {
		t.Fatalf("json = %v", m)
	}
	recs := m["records"].([]any)
	dkim := recs[0].(map[string]any)
	if dkim["ok"] != false || dkim["fqdn"] != "agdir._domainkey.acme.com" || dkim["required"] != true || dkim["expected"] != dkimValue {
		t.Fatalf("dkim = %v", dkim)
	}
	if dmarc := recs[3].(map[string]any); dmarc["required"] != false {
		t.Fatalf("dmarc = %v", dmarc)
	}
}

func TestDomainsCheckSubdomain(t *testing.T) {
	e := newEnv(t).withKey(keyFull)
	e.api.addDomain("mail.acme.co.uk")
	res := healthyDNS("mail.acme", "co.uk") // fake API treats the last two labels as the apex
	e.resolver = res
	e.run("domains", "check", "mail.acme.co.uk").expect(t, 0).contains(t, "send.mail.acme.co.uk", "agdir._domainkey.mail.acme.co.uk")
	if res.queries[0] != "TXT agdir._domainkey.mail.acme.co.uk" {
		t.Fatalf("queries = %v", res.queries)
	}
}

func TestDomainsCheckLookupFailure(t *testing.T) {
	e := newEnv(t).withKey(keyFull)
	e.api.addDomain("acme.com")
	e.resolver = &fakeResolver{fail: errors.New("i/o timeout")}
	e.run("domains", "check", "acme.com").expect(t, ExitError).contains(t, "lookup failed: i/o timeout")
}

func TestCheckRecordRules(t *testing.T) {
	rec := func(purpose, typ, name, value string) agdiremail.DNSRecord {
		return agdiremail.DNSRecord{Purpose: purpose, Type: typ, Name: name, Value: value}
	}
	r := &fakeResolver{
		txt: map[string][]string{
			"send.x.com":             {"v=spf1 include:amazonses.com ~all", "v=spf1 -all"},
			"agdir._domainkey.x.com": {"p=OTHERKEY"},
			"verify.x.com":           {`"token-123"`},
		},
		cname: map[string]string{"link.x.com": "Track.Example.NET."},
	}
	checks := checkRecords(context.Background(), r, "x.com", []agdiremail.DNSRecord{
		rec("SPF", "TXT", "send", "v=spf1 include:amazonses.com ~all"),
		rec("DKIM", "TXT", "agdir._domainkey", "p=MYKEY"),
		rec("Other", "TXT", "verify", "token-123"),
		rec("Tracking", "CNAME", "link", "track.example.net"),
	})
	if checks[0].OK || !strings.Contains(checks[0].Problem, "several SPF records") {
		t.Errorf("multiple SPF: %+v", checks[0])
	}
	if checks[1].OK || !strings.Contains(checks[1].Problem, "expected DKIM public key") {
		t.Errorf("DKIM mismatch: %+v", checks[1])
	}
	if !checks[2].OK {
		t.Errorf("generic TXT: %+v", checks[2])
	}
	if !checks[3].OK || checks[3].Found[0] != "Track.Example.NET" {
		t.Errorf("CNAME: %+v", checks[3])
	}
}

func TestZoneApex(t *testing.T) {
	recs := func(names ...string) []agdiremail.DNSRecord {
		var out []agdiremail.DNSRecord
		for _, n := range names {
			out = append(out, agdiremail.DNSRecord{Name: n})
		}
		return out
	}
	for _, tc := range []struct {
		domain string
		names  []string
		want   string
	}{
		{"acme.com", []string{"agdir._domainkey", "send"}, "acme.com"},
		{"mail.acme.com", []string{"agdir._domainkey.mail", "send.mail"}, "acme.com"},
		{"a.b.acme.co.uk", []string{"_dmarc.a.b"}, "acme.co.uk"},
		{"acme.com", []string{"weird"}, "acme.com"},
	} {
		if got := zoneApex(tc.domain, recs(tc.names...)); got != tc.want {
			t.Errorf("zoneApex(%s, %v) = %s, want %s", tc.domain, tc.names, got, tc.want)
		}
	}
}
