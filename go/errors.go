package agdiremail

import (
	"fmt"
	"time"
)

// ErrorCode is a machine-readable error code: the API's error name, or a client-side code.
type ErrorCode string

// Error codes returned by the Agdir Email API.
const (
	CodeMissingAPIKey                ErrorCode = "missing_api_key"                // 401: no Authorization header
	CodeInvalidAPIKey                ErrorCode = "invalid_api_key"                // 403: unknown or revoked key
	CodeRestrictedAPIKey             ErrorCode = "restricted_api_key"             // 401: sending-only key on a read endpoint
	CodeValidationError              ErrorCode = "validation_error"               // 422, or 403 for unverified domain / paused team / domain limit
	CodeNotFound                     ErrorCode = "not_found"                      // 404
	CodeInvalidIdempotentRequest     ErrorCode = "invalid_idempotent_request"     // 409: same key, different body
	CodeConcurrentIdempotentRequests ErrorCode = "concurrent_idempotent_requests" // 409: same key still in flight
	CodeDailyQuotaExceeded           ErrorCode = "daily_quota_exceeded"           // 429, never retried
	CodeMonthlyQuotaExceeded         ErrorCode = "monthly_quota_exceeded"         // 429, never retried
	CodeRateLimitExceeded            ErrorCode = "rate_limit_exceeded"            // 429, always retried
	CodeInternalServerError          ErrorCode = "internal_server_error"          // 5xx
	CodeUnknownError                 ErrorCode = "unknown_error"                  // non-JSON 4xx body
)

// Client-side error codes (Status is 0: no response was received).
const (
	CodeNetworkError ErrorCode = "network_error"
	CodeTimeout      ErrorCode = "timeout"
)

// Error is returned by every client method when a request fails.
//
//	var apiErr *agdiremail.Error
//	if errors.As(err, &apiErr) && apiErr.Code == agdiremail.CodeValidationError { … }
//
// Cancelling the caller's context returns the context's error instead.
type Error struct {
	// Code is machine-readable, e.g. CodeValidationError or CodeRateLimitExceeded.
	Code ErrorCode
	// Status is the HTTP status, or 0 when no response was received.
	Status int
	// Message is a human-readable explanation.
	Message string
	// RetryAfter is how long the API asked us to wait (from the retry-after header), or 0.
	RetryAfter time.Duration
	// Err is the underlying transport error for CodeNetworkError and CodeTimeout.
	Err error
}

func (e *Error) Error() string {
	if e.Status == 0 {
		return fmt.Sprintf("agdiremail: %s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("agdiremail: %s (HTTP %d): %s", e.Code, e.Status, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

// WebhookVerificationError is returned by [VerifyWebhook] when a request's signature,
// timestamp or payload is invalid.
type WebhookVerificationError struct {
	Reason string
}

func (e *WebhookVerificationError) Error() string {
	return "agdiremail: webhook verification failed: " + e.Reason
}
