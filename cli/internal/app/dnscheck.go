package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	agdiremail "github.com/AgdirAS/agdirlabs-sdks/go"
)

// Resolver performs the DNS lookups of `domains check`. *net.Resolver implements it.
type Resolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
	LookupMX(ctx context.Context, name string) ([]*net.MX, error)
	LookupCNAME(ctx context.Context, host string) (string, error)
}

// RecordCheck is the result of checking one DNS record.
type RecordCheck struct {
	Purpose  string `json:"purpose"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	FQDN     string `json:"fqdn"`
	Expected string `json:"expected"`
	Priority int    `json:"priority,omitempty"`
	// Required is false for recommended records (DMARC).
	Required bool `json:"required"`
	OK       bool `json:"ok"`
	// Found lists the values published at FQDN for this record type.
	Found []string `json:"found"`
	// Problem explains a failed check (or a note on a passed one).
	Problem string `json:"problem,omitempty"`
}

// checkRecords looks up each record of a domain and compares it with the expected value.
func checkRecords(ctx context.Context, r Resolver, domain string, records []agdiremail.DNSRecord) []RecordCheck {
	apex := zoneApex(domain, records)
	out := make([]RecordCheck, len(records))
	for i, rec := range records {
		c := RecordCheck{
			Purpose: rec.Purpose, Type: strings.ToUpper(rec.Type), Name: rec.Name,
			FQDN: recordFQDN(rec.Name, apex), Expected: rec.Value, Priority: rec.Priority,
			Required: !strings.EqualFold(rec.Purpose, "DMARC"), Found: []string{},
		}
		lctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		switch c.Type {
		case "TXT":
			checkTXT(lctx, r, &c)
		case "MX":
			checkMX(lctx, r, &c)
		case "CNAME":
			checkCNAME(lctx, r, &c)
		default:
			c.Problem = "cannot check " + c.Type + " records"
		}
		cancel()
		out[i] = c
	}
	return out
}

// zoneApex finds the domain the record names are relative to (the registrable domain): the
// API names records like "send" or "_dmarc" for acme.com and "send.mail" for mail.acme.com.
func zoneApex(domain string, records []agdiremail.DNSRecord) string {
	domain = strings.TrimSuffix(strings.ToLower(domain), ".")
	for _, rec := range records {
		name := strings.ToLower(rec.Name)
		var prefix string
		switch {
		case strings.Contains(name, "._domainkey"):
			prefix = name[:strings.Index(name, "._domainkey")+len("._domainkey")]
		case name == "send" || strings.HasPrefix(name, "send."):
			prefix = "send"
		case name == "_dmarc" || strings.HasPrefix(name, "_dmarc."):
			prefix = "_dmarc"
		default:
			continue
		}
		if name == prefix {
			return domain
		}
		sub := strings.TrimPrefix(name, prefix+".")
		if apex, ok := strings.CutPrefix(domain, sub+"."); ok {
			return apex
		}
	}
	return domain
}

func recordFQDN(name, apex string) string {
	name = strings.TrimSuffix(name, ".")
	if name == "" || name == "@" {
		return apex
	}
	if name == apex || strings.HasSuffix(name, "."+apex) {
		return name
	}
	return name + "." + apex
}

func describeLookupError(c *RecordCheck, err error) {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
		c.Problem = fmt.Sprintf("no %s record found at %s", c.Type, c.FQDN)
		return
	}
	c.Problem = "lookup failed: " + err.Error()
}

func checkTXT(ctx context.Context, r Resolver, c *RecordCheck) {
	found, err := r.LookupTXT(ctx, c.FQDN)
	if err != nil {
		describeLookupError(c, err)
		return
	}
	c.Found = append(c.Found, found...)
	expected := strings.TrimSpace(c.Expected)
	lower := strings.ToLower(expected)
	switch {
	case strings.HasPrefix(lower, "v=spf1"):
		var spf []string
		for _, v := range found {
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(v)), "v=spf1") {
				spf = append(spf, v)
			}
		}
		switch {
		case len(spf) == 0:
			c.Problem = "no SPF record (v=spf1 …) found at " + c.FQDN
		case len(spf) > 1:
			c.Problem = "several SPF records found; publish exactly one"
		default:
			if missing := missingMechanisms(expected, spf[0]); len(missing) > 0 {
				c.Problem = "SPF record does not contain " + strings.Join(missing, " ")
			} else {
				c.OK = true
			}
		}
	case strings.HasPrefix(lower, "v=dmarc1"):
		for _, v := range found {
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(v)), "v=dmarc1") {
				c.OK = true
			}
		}
		if !c.OK {
			c.Problem = "no DMARC record (v=DMARC1 …) found at " + c.FQDN
		}
	case dkimKey(expected) != "":
		want := dkimKey(expected)
		for _, v := range found {
			if dkimKey(v) == want {
				c.OK = true
			}
		}
		if !c.OK {
			c.Problem = "no TXT record with the expected DKIM public key (p=…) at " + c.FQDN
		}
	default:
		for _, v := range found {
			if normalizeTXT(v) == normalizeTXT(expected) {
				c.OK = true
			}
		}
		if !c.OK {
			c.Problem = "TXT value does not match"
		}
	}
	if !c.OK && len(found) == 0 && c.Problem == "" {
		c.Problem = fmt.Sprintf("no TXT record found at %s", c.FQDN)
	}
}

// missingMechanisms lists the include:/a/mx mechanisms of the expected SPF record that the
// published one lacks (the published record may authorise other senders too).
func missingMechanisms(expected, published string) []string {
	have := map[string]bool{}
	for _, t := range strings.Fields(strings.ToLower(published)) {
		have[strings.TrimLeft(t, "+")] = true
	}
	var missing []string
	for _, t := range strings.Fields(strings.ToLower(expected)) {
		t = strings.TrimLeft(t, "+")
		if strings.HasPrefix(t, "include:") && !have[t] {
			missing = append(missing, t)
		}
	}
	return missing
}

// dkimKey returns the p= tag of a DKIM record, without whitespace.
func dkimKey(v string) string {
	for _, tag := range strings.Split(v, ";") {
		k, val, ok := strings.Cut(strings.TrimSpace(tag), "=")
		if ok && strings.TrimSpace(k) == "p" {
			return strings.Join(strings.Fields(val), "")
		}
	}
	return ""
}

func normalizeTXT(v string) string {
	return strings.Join(strings.Fields(strings.Trim(strings.TrimSpace(v), `"`)), "")
}

func checkMX(ctx context.Context, r Resolver, c *RecordCheck) {
	mxs, err := r.LookupMX(ctx, c.FQDN)
	if err != nil {
		describeLookupError(c, err)
		return
	}
	sort.Slice(mxs, func(i, j int) bool { return mxs[i].Pref < mxs[j].Pref })
	want := hostKey(c.Expected)
	for _, mx := range mxs {
		c.Found = append(c.Found, fmt.Sprintf("%d %s", mx.Pref, strings.TrimSuffix(mx.Host, ".")))
		if hostKey(mx.Host) == want {
			c.OK = true
			if c.Priority != 0 && int(mx.Pref) != c.Priority {
				c.Problem = fmt.Sprintf("priority is %d (expected %d); this still works", mx.Pref, c.Priority)
			}
		}
	}
	if !c.OK {
		if len(mxs) == 0 {
			c.Problem = fmt.Sprintf("no MX record found at %s", c.FQDN)
		} else {
			c.Problem = "MX record points elsewhere"
		}
	}
}

func checkCNAME(ctx context.Context, r Resolver, c *RecordCheck) {
	target, err := r.LookupCNAME(ctx, c.FQDN)
	if err != nil {
		describeLookupError(c, err)
		return
	}
	c.Found = append(c.Found, strings.TrimSuffix(target, "."))
	if hostKey(target) == hostKey(c.Expected) {
		c.OK = true
	} else {
		c.Problem = "CNAME points elsewhere"
	}
}

func hostKey(h string) string { return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(h), ".")) }

// nameserverResolver queries one DNS server directly (bypassing the system resolver's cache).
func nameserverResolver(server string) *net.Resolver {
	if _, _, err := net.SplitHostPort(server); err != nil {
		server = net.JoinHostPort(server, "53")
	}
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, server)
		},
	}
}
