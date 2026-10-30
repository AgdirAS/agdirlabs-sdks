package agdiremail

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	mrand "math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type httpClient struct {
	apiKey     string
	baseURL    string
	client     *http.Client
	maxRetries int
	timeout    time.Duration
	sleep      func(ctx context.Context, d time.Duration) error
}

type request struct {
	method string
	path   string
	query  url.Values
	body   any
	header map[string]string
	// retryable: safe to repeat (GETs, and POSTs that carry an idempotency key).
	retryable bool
}

var retryStatuses = map[int]bool{408: true, 500: true, 502: true, 503: true, 504: true}

// maxResponseBytes caps how much of a response body is read.
const maxResponseBytes = 16 << 20

// backoff is exponential with jitter: ~0.5 s, 1 s, 2 s … capped at 8 s.
func backoff(attempt int) time.Duration {
	base := 500 * time.Millisecond << min(attempt, 4)
	base = min(base, 8*time.Second)
	return base/2 + time.Duration(mrand.Int64N(int64(base/2)+1))
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func parseRetryAfter(raw string) time.Duration {
	if raw == "" {
		return 0
	}
	if secs, err := strconv.ParseFloat(raw, 64); err == nil {
		return max(0, time.Duration(secs*float64(time.Second)))
	}
	if t, err := http.ParseTime(raw); err == nil {
		return max(0, time.Until(t))
	}
	return 0
}

// do sends one API request and decodes the JSON response into out, retrying rate limits,
// server errors and network failures when safe.
func (h *httpClient) do(ctx context.Context, req request, out any) error {
	u := h.baseURL + req.path
	if len(req.query) > 0 {
		u += "?" + req.query.Encode()
	}
	var body []byte
	if req.body != nil {
		var err error
		if body, err = json.Marshal(req.body); err != nil {
			return fmt.Errorf("agdiremail: encoding request: %w", err)
		}
	}

	for attempt := 0; ; attempt++ {
		canRetry := attempt < h.maxRetries
		status, respBody, retryAfter, err := h.attempt(ctx, req, u, body)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// A timed-out POST may have been accepted: only retry it when it is idempotent.
			if canRetry && req.retryable {
				if err := h.sleep(ctx, backoff(attempt)); err != nil {
					return err
				}
				continue
			}
			if errors.Is(err, context.DeadlineExceeded) {
				return &Error{Code: CodeTimeout, Message: fmt.Sprintf("request timed out after %s", h.timeout), Err: err}
			}
			return &Error{Code: CodeNetworkError, Message: "could not reach the Agdir Email API", Err: err}
		}

		if status >= 200 && status < 300 {
			if out == nil {
				return nil
			}
			if err := json.Unmarshal(respBody, out); err != nil {
				return &Error{Code: CodeUnknownError, Status: status, Message: "could not decode the API response", Err: err}
			}
			return nil
		}

		apiErr := toError(status, respBody, retryAfter)
		// Rate limits are refused before any work is done, so they are always safe to retry.
		retry := canRetry && (apiErr.Code == CodeRateLimitExceeded || (req.retryable && retryStatuses[status]))
		if !retry {
			return apiErr
		}
		wait := apiErr.RetryAfter
		if wait == 0 {
			wait = backoff(attempt)
		}
		if err := h.sleep(ctx, wait); err != nil {
			return err
		}
	}
}

func (h *httpClient) attempt(ctx context.Context, req request, u string, body []byte) (int, []byte, time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	hr, err := http.NewRequestWithContext(ctx, req.method, u, reader)
	if err != nil {
		return 0, nil, 0, err
	}
	hr.Header.Set("Authorization", "Bearer "+h.apiKey)
	hr.Header.Set("User-Agent", UserAgent)
	hr.Header.Set("Accept", "application/json")
	if body != nil {
		hr.Header.Set("Content-Type", "application/json")
	}
	for k, v := range req.header {
		if v != "" {
			hr.Header.Set(k, v)
		}
	}

	res, err := h.client.Do(hr)
	if err != nil {
		return 0, nil, 0, err
	}
	defer res.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes))
	if err != nil {
		return 0, nil, 0, err
	}
	return res.StatusCode, respBody, parseRetryAfter(res.Header.Get("Retry-After")), nil
}

func toError(status int, body []byte, retryAfter time.Duration) *Error {
	var parsed struct {
		Name    string `json:"name"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &parsed) // not JSON (e.g. a proxy error page): fall back to the status line
	e := &Error{Code: ErrorCode(parsed.Name), Status: status, Message: parsed.Message, RetryAfter: retryAfter}
	if e.Message == "" {
		e.Message = fmt.Sprintf("Request failed with HTTP %d", status)
	}
	if e.Code == "" {
		if status >= 500 {
			e.Code = CodeInternalServerError
		} else {
			e.Code = CodeUnknownError
		}
	}
	return e
}

// newIdempotencyKey returns a random UUID v4.
func newIdempotencyKey() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func listQuery(p *ListParams) url.Values {
	q := url.Values{}
	if p != nil {
		if p.Limit > 0 {
			q.Set("limit", strconv.Itoa(p.Limit))
		}
		if p.After != "" {
			q.Set("after", p.After)
		}
	}
	return q
}

func seg(s string) string { return url.PathEscape(s) }
