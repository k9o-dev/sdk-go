package k9o

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// httpClientConfig is the per-instance HTTP config bound on Client.
type httpClientConfig struct {
	BaseURL        string
	APIKey         string
	OrgSlug        string
	ProjectSlug    string
	DefaultHeaders map[string]string
	HTTPClient     *http.Client
	Retry          *RetryConfig

	region regionState // resolved regional origin cache (region.go)
}

// requestInit carries per-call request options.
type requestInit struct {
	Method string
	Path   string
	Body   any
	// RawBody, when non-nil, is sent as-is (no JSON marshalling) with
	// RawContentType as Content-Type (multipart sets its own boundary).
	RawBody        []byte
	RawContentType string
	Bearer         string // overrides config.APIKey for this call
	Headers        map[string]string
	Query          map[string]string
	// NoCredential strips Authorization and Cookie (including any set via
	// DefaultHeaders) so the call is sent unauthenticated.
	NoCredential bool
}

func (c *httpClientConfig) buildHeaders(init *requestInit) http.Header {
	h := http.Header{}
	h.Set("Accept", "application/json")
	for k, v := range c.DefaultHeaders {
		h.Set(k, v)
	}
	if c.ProjectSlug != "" {
		h.Set("X-K9O-Project-Slug", c.ProjectSlug)
	}
	if c.OrgSlug != "" {
		h.Set("X-K9O-Org-Slug", c.OrgSlug)
	}
	for k, v := range init.Headers {
		h.Set(k, v)
	}
	token := init.Bearer
	if token == "" {
		token = c.APIKey
	}
	if token != "" {
		h.Set("Authorization", "Bearer "+token)
	}
	if init.NoCredential {
		h.Del("Authorization")
		h.Del("Cookie")
	}
	return h
}

// doRequest issues an HTTP request with retry on transient failures, and
// decodes the response into an Envelope. Idempotent methods are retried
// with backoff per cfg.Retry; semantic / non-idempotent
// failures return immediately. Generic over the data shape.
func doRequest[T any](ctx context.Context, cfg *httpClientConfig, init *requestInit) (*Envelope[T], *KError) {
	rc := cfg.Retry
	if rc == nil {
		rc = defaultRetryConfig()
	}
	var lastErr *KError
	for attempt := 0; attempt <= rc.MaxRetries; attempt++ {
		env, kerr := doRequestOnce[T](ctx, cfg, init)
		if kerr == nil {
			return env, nil
		}
		lastErr = kerr
		if attempt == rc.MaxRetries || !isRetryable(kerr, init.Method) {
			return nil, kerr
		}
		delay := computeDelayMs(rc, attempt)
		if delay > 0 {
			timer := time.NewTimer(time.Duration(delay) * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, &KError{
					Code:    "K9O_UPSTREAM_UNAVAILABLE",
					Message: fmt.Sprintf("context cancelled during retry backoff: %v", ctx.Err()),
				}
			case <-timer.C:
			}
		}
	}
	return nil, lastErr
}

// doRequestOnce is a single attempt. doRequest wraps it in retry-with-backoff.
func doRequestOnce[T any](ctx context.Context, cfg *httpClientConfig, init *requestInit) (*Envelope[T], *KError) {
	urlStr := strings.TrimRight(cfg.resolveOrigin(ctx), "/") + init.Path
	if len(init.Query) > 0 {
		params := []string{}
		for k, v := range init.Query {
			params = append(params, k+"="+url.QueryEscape(v))
		}
		urlStr = urlStr + "?" + strings.Join(params, "&")
	}

	var bodyReader io.Reader
	headers := cfg.buildHeaders(init)
	if init.RawBody != nil {
		bodyReader = bytes.NewReader(init.RawBody)
		headers.Set("Content-Type", init.RawContentType)
	} else if init.Body != nil {
		bodyBytes, err := json.Marshal(init.Body)
		if err != nil {
			return nil, &KError{
				Code:    "K9O_INTERNAL_ERROR",
				Message: fmt.Sprintf("failed to marshal request body: %v", err),
			}
		}
		bodyReader = bytes.NewReader(bodyBytes)
		headers.Set("Content-Type", "application/json")
	}

	req, err := http.NewRequestWithContext(ctx, init.Method, urlStr, bodyReader)
	if err != nil {
		return nil, &KError{
			Code:    "K9O_INTERNAL_ERROR",
			Message: fmt.Sprintf("failed to build request: %v", err),
		}
	}
	req.Header = headers

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, &KError{
			Code:    "K9O_UPSTREAM_UNAVAILABLE",
			Message: fmt.Sprintf("network error reaching %s: %v", urlStr, err),
			Context: map[string]any{"url": urlStr},
		}
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &KError{
			Code:    "K9O_INTERNAL_ERROR",
			Message: fmt.Sprintf("failed to read response body from %s: %v", urlStr, err),
			Status:  resp.StatusCode,
		}
	}

	// 204 No Content
	if resp.StatusCode == 204 {
		var zero T
		return &Envelope[T]{Data: zero, Meta: Meta{}}, nil
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var env Envelope[T]
		if err := json.Unmarshal(respBytes, &env); err != nil {
			return nil, &KError{
				Code:    "K9O_INTERNAL_ERROR",
				Message: fmt.Sprintf("malformed envelope from %s: %v", urlStr, err),
				Context: map[string]any{"url": urlStr, "status": resp.StatusCode},
				Status:  resp.StatusCode,
			}
		}
		return &env, nil
	}

	// Non-2xx — try KError shape
	var wire KErrorWire
	if err := json.Unmarshal(respBytes, &wire); err == nil && wire.Code != "" && wire.Message != "" {
		return nil, kerrorFromWire(&wire, resp.StatusCode)
	}
	return nil, &KError{
		Code:    "K9O_INTERNAL_ERROR",
		Message: fmt.Sprintf("upstream returned %d with non-KError body", resp.StatusCode),
		Context: map[string]any{"url": urlStr, "status": resp.StatusCode, "body": string(respBytes)},
		Status:  resp.StatusCode,
	}
}
