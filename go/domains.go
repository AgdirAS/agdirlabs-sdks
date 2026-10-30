package agdiremail

import (
	"context"
	"errors"
	"iter"
)

// DomainsService manages sending domains. Creating, verifying and deleting domains
// requires a full-access API key.
type DomainsService struct {
	http *httpClient
}

// Create adds a sending domain. Publish the returned Records in DNS, then call Verify.
func (s *DomainsService) Create(ctx context.Context, params *CreateDomainRequest) (*Domain, error) {
	if params == nil {
		return nil, errors.New("agdiremail: Domains.Create: params is nil")
	}
	var w wireDomain
	if err := s.http.do(ctx, request{method: "POST", path: "/domains", body: params}, &w); err != nil {
		return nil, err
	}
	d := w.toDomain()
	return &d, nil
}

// Get reads a domain, including its DNS records.
func (s *DomainsService) Get(ctx context.Context, id string) (*Domain, error) {
	var w wireDomain
	if err := s.http.do(ctx, request{method: "GET", path: "/domains/" + seg(id), retryable: true}, &w); err != nil {
		return nil, err
	}
	d := w.toDomain()
	return &d, nil
}

// List returns one page of domains (without DNS records).
func (s *DomainsService) List(ctx context.Context, params *ListParams) (*Page[Domain], error) {
	var w wirePage[wireDomain]
	if err := s.http.do(ctx, request{method: "GET", path: "/domains", query: listQuery(params), retryable: true}, &w); err != nil {
		return nil, err
	}
	return toPage(w, wireDomain.toDomain, func(d Domain) string { return d.ID }), nil
}

// ListAll iterates over every domain, fetching further pages as needed.
// Iteration stops after the first error.
//
//	for d, err := range client.Domains.ListAll(ctx, nil) {
//		if err != nil { return err }
//		fmt.Println(d.Name, d.Status)
//	}
func (s *DomainsService) ListAll(ctx context.Context, params *ListParams) iter.Seq2[Domain, error] {
	return paginate(ctx, params, s.List)
}

// Verify starts a DNS check now instead of waiting for the next automatic one.
func (s *DomainsService) Verify(ctx context.Context, id string) (*IDResult, error) {
	var out IDResult
	if err := s.http.do(ctx, request{method: "POST", path: "/domains/" + seg(id) + "/verify", retryable: true}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete removes a domain.
func (s *DomainsService) Delete(ctx context.Context, id string) (*DeleteResult, error) {
	var out DeleteResult
	if err := s.http.do(ctx, request{method: "DELETE", path: "/domains/" + seg(id)}, &out); err != nil {
		return nil, err
	}
	out.Deleted = true
	return &out, nil
}
