package agdiremail

import (
	"context"
	"iter"
)

// ListParams selects a page of a list endpoint. A nil *ListParams means the defaults.
type ListParams struct {
	// Limit is the page size, 1–100. Zero uses the API default.
	Limit int
	// After is a cursor: the NextCursor of the previous page.
	After string
}

// Page is one page of a list.
type Page[T any] struct {
	Data    []T  `json:"data"`
	HasMore bool `json:"has_more"`
	// NextCursor is passed as ListParams.After to fetch the next page; empty on the last page.
	NextCursor string `json:"next_cursor,omitempty"`
}

type wirePage[W any] struct {
	HasMore bool `json:"has_more"`
	Data    []W  `json:"data"`
}

func toPage[W, T any](w wirePage[W], conv func(W) T, id func(T) string) *Page[T] {
	p := &Page[T]{Data: make([]T, len(w.Data)), HasMore: w.HasMore}
	for i, item := range w.Data {
		p.Data[i] = conv(item)
	}
	if w.HasMore && len(p.Data) > 0 {
		p.NextCursor = id(p.Data[len(p.Data)-1])
	}
	return p
}

func same[T any](v T) T { return v }

// paginate walks every page, yielding each item. It stops at the first error,
// which is yielded with the zero value of T.
func paginate[T any](ctx context.Context, params *ListParams, fetch func(context.Context, *ListParams) (*Page[T], error)) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		p := ListParams{}
		if params != nil {
			p = *params
		}
		for {
			page, err := fetch(ctx, &p)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			for _, item := range page.Data {
				if !yield(item, nil) {
					return
				}
			}
			if page.NextCursor == "" {
				return
			}
			p.After = page.NextCursor
		}
	}
}
