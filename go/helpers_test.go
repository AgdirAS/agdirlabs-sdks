package agdiremail

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type recorded struct {
	Method string
	Path   string
	Query  map[string][]string
	Header http.Header
	Body   any
}

type reply struct {
	Status int
	Body   any    // JSON-encoded unless raw is set
	Raw    string // sent as-is (text/html)
	Header map[string]string
	Delay  time.Duration
}

type fakeAPI struct {
	t        *testing.T
	mu       sync.Mutex
	replies  []reply
	requests []recorded
	server   *httptest.Server
	sleeps   []time.Duration
}

// newFake starts a test server answering with the scripted replies in order and returns a
// client pointed at it whose retry sleeps are recorded instead of slept.
func newFake(t *testing.T, replies ...reply) (*fakeAPI, *Client) {
	t.Helper()
	f := &fakeAPI{t: t, replies: replies}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	c, err := NewClient(WithAPIKey("re_test"), WithBaseURL(f.server.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}
	c.http.sleep = func(ctx context.Context, d time.Duration) error {
		f.mu.Lock()
		f.sleeps = append(f.sleeps, d)
		f.mu.Unlock()
		return ctx.Err()
	}
	return f, c
}

func (f *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &body); err != nil {
			f.t.Errorf("request body is not JSON: %s", raw)
		}
	}
	f.mu.Lock()
	f.requests = append(f.requests, recorded{r.Method, r.URL.Path, r.URL.Query(), r.Header.Clone(), body})
	if len(f.replies) == 0 {
		f.mu.Unlock()
		f.t.Errorf("no reply scripted for %s %s", r.Method, r.URL.Path)
		w.WriteHeader(599)
		return
	}
	rep := f.replies[0]
	f.replies = f.replies[1:]
	f.mu.Unlock()

	if rep.Delay > 0 {
		select {
		case <-time.After(rep.Delay):
		case <-r.Context().Done():
			return
		}
	}
	for k, v := range rep.Header {
		w.Header().Set(k, v)
	}
	status := rep.Status
	if status == 0 {
		status = 200
	}
	if rep.Raw != "" {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, rep.Raw)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	b := rep.Body
	if b == nil {
		b = map[string]any{}
	}
	_ = json.NewEncoder(w).Encode(b)
}

func (f *fakeAPI) req(i int) recorded {
	f.t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if i >= len(f.requests) {
		f.t.Fatalf("expected request #%d, got %d requests", i, len(f.requests))
	}
	return f.requests[i]
}

func (f *fakeAPI) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

func apiError(status int, name, message string) reply {
	return reply{Status: status, Body: map[string]any{"statusCode": status, "name": name, "message": message}}
}

type M = map[string]any
