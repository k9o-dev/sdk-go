package k9o

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// countingRT returns the status at index = call number (last entry repeats),
// counting invocations so a test can assert how many attempts were made.
type countingRT struct {
	calls    int
	statuses []int
}

func (rt *countingRT) RoundTrip(_ *http.Request) (*http.Response, error) {
	idx := rt.calls
	rt.calls++
	if idx >= len(rt.statuses) {
		idx = len(rt.statuses) - 1
	}
	status := rt.statuses[idx]
	body := `{"code":"K9O_UPSTREAM_UNAVAILABLE","message":"down"}`
	if status >= 200 && status < 300 {
		body = `{"data":null,"meta":{}}`
	} else if status == 404 {
		body = `{"code":"K9O_NOT_FOUND","message":"nope"}`
	} else if status == 429 {
		body = `{"code":"K9O_RATE_LIMITED","message":"slow"}`
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

func retryCfg(rt http.RoundTripper) *httpClientConfig {
	return &httpClientConfig{
		BaseURL:    "https://acme.api.k9o.io",
		HTTPClient: &http.Client{Transport: noProbe(rt)},
		Retry:      &RetryConfig{MaxRetries: 3, Backoff: "none"},
	}
}

func TestRetryGetSucceedsAfterTransient(t *testing.T) {
	rt := &countingRT{statuses: []int{503, 503, 200}}
	_, kerr := doRequest[any](context.Background(), retryCfg(rt), &requestInit{Method: "GET", Path: "/orgs"})
	if kerr != nil {
		t.Fatalf("expected success after retries, got %v", kerr)
	}
	if rt.calls != 3 {
		t.Fatalf("expected 3 attempts, got %d", rt.calls)
	}
}

func TestRetryGetExhausts(t *testing.T) {
	rt := &countingRT{statuses: []int{503}}
	_, kerr := doRequest[any](context.Background(), retryCfg(rt), &requestInit{Method: "GET", Path: "/orgs"})
	if kerr == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if rt.calls != 4 {
		t.Fatalf("expected 4 attempts (3 retries + 1), got %d", rt.calls)
	}
}

func TestRetryPostNotRetried(t *testing.T) {
	rt := &countingRT{statuses: []int{503}}
	_, kerr := doRequest[any](context.Background(), retryCfg(rt), &requestInit{Method: "POST", Path: "/projects", Body: map[string]any{"x": 1}})
	if kerr == nil {
		t.Fatal("expected error")
	}
	if rt.calls != 1 {
		t.Fatalf("POST must not be retried; expected 1 attempt, got %d", rt.calls)
	}
}

func TestRetryGetSemantic4xxNotRetried(t *testing.T) {
	rt := &countingRT{statuses: []int{404}}
	_, kerr := doRequest[any](context.Background(), retryCfg(rt), &requestInit{Method: "GET", Path: "/orgs/missing"})
	if kerr == nil {
		t.Fatal("expected error")
	}
	if rt.calls != 1 {
		t.Fatalf("404 is semantic; expected 1 attempt, got %d", rt.calls)
	}
}

func TestRetryGet429Retried(t *testing.T) {
	rt := &countingRT{statuses: []int{429, 200}}
	_, kerr := doRequest[any](context.Background(), retryCfg(rt), &requestInit{Method: "GET", Path: "/orgs"})
	if kerr != nil {
		t.Fatalf("expected success, got %v", kerr)
	}
	if rt.calls != 2 {
		t.Fatalf("expected 2 attempts, got %d", rt.calls)
	}
}
