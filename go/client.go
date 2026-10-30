package agdiremail

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

// Version is the version of this SDK.
const Version = "0.1.0"

// UserAgent is sent as the User-Agent header on every request.
const UserAgent = "agdirlabs-email-go/" + Version

// DefaultBaseURL is the Agdir Email API endpoint used when no base URL is configured.
const DefaultBaseURL = "https://api.agdir.email"

// Defaults for the client options.
const (
	DefaultMaxRetries = 2
	DefaultTimeout    = 30 * time.Second
)

// Environment variables read by [NewClient] when the corresponding option is not given.
const (
	EnvAPIKey  = "AGDIR_EMAIL_API_KEY"
	EnvBaseURL = "AGDIR_EMAIL_BASE_URL"
)

// ErrMissingAPIKey is returned by [NewClient] when no API key was given and
// AGDIR_EMAIL_API_KEY is unset or empty.
var ErrMissingAPIKey = errors.New("agdiremail: missing API key: use WithAPIKey or set AGDIR_EMAIL_API_KEY. Create one at https://app.agdir.email")

// Client is an Agdir Email API client. It is safe for concurrent use.
type Client struct {
	// Emails sends emails and reads their delivery status.
	Emails *EmailsService
	// Domains manages sending domains.
	Domains *DomainsService
	// Webhooks manages webhook endpoints and their deliveries.
	Webhooks *WebhooksService

	http *httpClient
}

// Option configures a [Client].
type Option func(*config)

type config struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	maxRetries int
	timeout    time.Duration
}

// WithAPIKey sets the API key (re_…). Defaults to the AGDIR_EMAIL_API_KEY environment variable.
func WithAPIKey(key string) Option { return func(c *config) { c.apiKey = key } }

// WithBaseURL sets the API base URL. Defaults to AGDIR_EMAIL_BASE_URL, else [DefaultBaseURL].
func WithBaseURL(url string) Option { return func(c *config) { c.baseURL = url } }

// WithHTTPClient sets the HTTP client used for requests (proxies, custom transports, tests).
// Defaults to [http.DefaultClient]. Per-attempt timeouts are applied through the request
// context, so the client's own Timeout can stay zero.
func WithHTTPClient(hc *http.Client) Option { return func(c *config) { c.httpClient = hc } }

// WithMaxRetries sets how many times a failed request is retried (rate limits, server errors,
// network failures). Default 2; 0 disables retries.
func WithMaxRetries(n int) Option { return func(c *config) { c.maxRetries = n } }

// WithTimeout sets the timeout of each attempt. Default 30 s.
func WithTimeout(d time.Duration) Option { return func(c *config) { c.timeout = d } }

// NewClient creates a client. Without options it reads AGDIR_EMAIL_API_KEY and
// AGDIR_EMAIL_BASE_URL from the environment; it returns [ErrMissingAPIKey] if no key is found.
func NewClient(opts ...Option) (*Client, error) {
	cfg := config{maxRetries: DefaultMaxRetries, timeout: DefaultTimeout}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.apiKey == "" {
		cfg.apiKey = os.Getenv(EnvAPIKey)
	}
	if cfg.apiKey == "" {
		return nil, ErrMissingAPIKey
	}
	if cfg.baseURL == "" {
		cfg.baseURL = os.Getenv(EnvBaseURL)
	}
	if cfg.baseURL == "" {
		cfg.baseURL = DefaultBaseURL
	}
	if cfg.maxRetries < 0 {
		return nil, errors.New("agdiremail: max retries must not be negative")
	}
	if cfg.timeout <= 0 {
		return nil, errors.New("agdiremail: timeout must be positive")
	}
	if cfg.httpClient == nil {
		cfg.httpClient = http.DefaultClient
	}

	hc := &httpClient{
		apiKey:     cfg.apiKey,
		baseURL:    strings.TrimRight(cfg.baseURL, "/"),
		client:     cfg.httpClient,
		maxRetries: cfg.maxRetries,
		timeout:    cfg.timeout,
		sleep:      sleepContext,
	}
	return &Client{
		Emails:   &EmailsService{http: hc},
		Domains:  &DomainsService{http: hc},
		Webhooks: &WebhooksService{http: hc, Deliveries: &WebhookDeliveriesService{http: hc}},
		http:     hc,
	}, nil
}

// BaseURL returns the API base URL the client sends requests to.
func (c *Client) BaseURL() string { return c.http.baseURL }
