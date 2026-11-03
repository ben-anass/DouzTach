package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ben-anass/DouzTach/client/validation"
)

const (
	defaultMaxResponseBytes = 8 << 20 // 8 MiB
	defaultUserAgent        = "douztech-cli"
)

// Client is a small, reusable JSON HTTP client for the DouzTach backend. It
// deliberately knows nothing about endpoint paths or payload schemas until the
// backend contract is published.
type Client struct {
	baseURL          *url.URL
	httpClient       *http.Client
	tokenProvider    func() string
	userAgent        string
	maxResponseBytes int64
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient supplies a custom HTTP client (useful for tests and proxies).
// The client value is copied so the caller's configuration is not mutated.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc == nil {
			return
		}
		clone := *hc
		c.httpClient = &clone
	}
}

// WithTimeout sets the request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		if timeout > 0 && c.httpClient != nil {
			c.httpClient.Timeout = timeout
		}
	}
}

// WithTokenProvider supplies a function called for every request. Returning an
// empty string omits the Authorization header.
func WithTokenProvider(provider func() string) Option {
	return func(c *Client) { c.tokenProvider = provider }
}

// WithUserAgent overrides the User-Agent header.
func WithUserAgent(userAgent string) Option {
	return func(c *Client) {
		if strings.TrimSpace(userAgent) != "" {
			c.userAgent = strings.TrimSpace(userAgent)
		}
	}
}

// WithMaxResponseBytes sets the maximum accepted response size.
func WithMaxResponseBytes(limit int64) Option {
	return func(c *Client) {
		if limit > 0 {
			c.maxResponseBytes = limit
		}
	}
}

// NewClient validates baseURL and constructs a client. An empty or invalid URL
// is an error; callers should check configuration before constructing a client.
func NewClient(baseURL string, opts ...Option) (*Client, error) {
	normalized, err := validation.BaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if normalized == "" {
		return nil, ErrNotConfigured
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return nil, fmt.Errorf("parse API URL: %w", err)
	}
	c := &Client{
		baseURL: parsed,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		userAgent:        defaultUserAgent,
		maxResponseBytes: defaultMaxResponseBytes,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	if c.httpClient == nil {
		c.httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	if c.maxResponseBytes <= 0 {
		c.maxResponseBytes = defaultMaxResponseBytes
	}
	return c, nil
}

// BaseURL returns the configured base URL without credentials.
func (c *Client) BaseURL() string {
	if c == nil || c.baseURL == nil {
		return ""
	}
	return c.baseURL.String()
}

// PingResult describes a reachability check.
type PingResult struct {
	StatusCode int
	Duration   time.Duration
}

// Ping checks whether the configured base URL answers at all. Any HTTP
// response counts as reachable; the status code is returned for diagnostics.
// This does not assume a /health endpoint.
func (c *Client) Ping(ctx context.Context) (PingResult, error) {
	if c == nil || c.baseURL == nil {
		return PingResult{}, ErrNotConfigured
	}
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL.String(), nil)
	if err != nil {
		return PingResult{}, fmt.Errorf("build connectivity request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return PingResult{}, fmt.Errorf("connect to API: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return PingResult{StatusCode: resp.StatusCode, Duration: time.Since(start)}, nil
}

// Do performs a JSON request. path must be relative to the configured base URL
// (for example "/tasks"). query may be nil. body is JSON-encoded when non-nil.
// out may be nil when the response body is not needed.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	if c == nil || c.baseURL == nil {
		return ErrNotConfigured
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if !validMethod(method) {
		return fmt.Errorf("invalid HTTP method %q", method)
	}
	requestURL, err := c.requestURL(path, query)
	if err != nil {
		return err
	}

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL.String(), reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.tokenProvider != nil {
		token := strings.TrimSpace(c.tokenProvider())
		if token != "" {
			if strings.ContainsAny(token, "\r\n") {
				return errors.New("authentication token contains invalid characters")
			}
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	data, err := readResponse(resp.Body, c.maxResponseBytes)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseAPIError(resp.StatusCode, method, path, resp.Header, data)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusResetContent {
		return nil
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return fmt.Errorf("%s %s: %w", method, path, ErrEmptyResponse)
	}
	if err := decodeJSON(data, out); err != nil {
		return fmt.Errorf("%s %s: decode response: %w", method, path, err)
	}
	return nil
}

// Get is a convenience wrapper around Do.
func (c *Client) Get(ctx context.Context, path string, query url.Values, out any) error {
	return c.Do(ctx, http.MethodGet, path, query, nil, out)
}

// Post is a convenience wrapper around Do.
func (c *Client) Post(ctx context.Context, path string, body, out any) error {
	return c.Do(ctx, http.MethodPost, path, nil, body, out)
}

// Patch is a convenience wrapper around Do.
func (c *Client) Patch(ctx context.Context, path string, body, out any) error {
	return c.Do(ctx, http.MethodPatch, path, nil, body, out)
}

// Delete is a convenience wrapper around Do.
func (c *Client) Delete(ctx context.Context, path string, out any) error {
	return c.Do(ctx, http.MethodDelete, path, nil, nil, out)
}

func (c *Client) requestURL(path string, query url.Values) (*url.URL, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("request path is empty")
	}
	if strings.ContainsAny(path, "?#") {
		return nil, errors.New("request path must not contain a query or fragment")
	}
	if strings.Contains(path, "://") || strings.HasPrefix(path, "//") {
		return nil, errors.New("request path must be relative to the configured API URL")
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == ".." {
			return nil, errors.New("request path must not contain '..'")
		}
	}
	for _, r := range path {
		if r < 0x20 || r == 0x7f {
			return nil, errors.New("request path contains control characters")
		}
	}

	joined := c.baseURL.JoinPath(strings.TrimPrefix(path, "/"))
	if query != nil {
		joined.RawQuery = query.Encode()
	}
	return joined, nil
}

func readResponse(reader io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		limit = defaultMaxResponseBytes
	}
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, ErrResponseTooLarge
	}
	return data, nil
}

func decodeJSON(data []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}

func validMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead:
		return true
	default:
		return false
	}
}
