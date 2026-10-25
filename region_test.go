package k9o

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// noProbe wraps a stub transport so the regional HEAD probe is answered with a
// plain 200 (no redirect) and never reaches the wrapped stub. Existing tests that
// count/inspect "the" recorded request use it so they see only the API call.
func noProbe(rt http.RoundTripper) http.RoundTripper {
	return probeSkipRT{rt}
}

type probeSkipRT struct{ next http.RoundTripper }

func (p probeSkipRT) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodHead && req.URL.Path == "/" {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: http.NoBody, Request: req}, nil
	}
	return p.next.RoundTrip(req)
}

type rec struct {
	method, url, auth, cookie, custom string
}

type regionRT struct {
	mu       sync.Mutex
	reqs     []rec
	location string // Location to answer the probe with; "" = 200 no redirect
	status   int    // probe status (default 308 when location != "")
	failHead int    // number of probes to fail with a network error first
}

func (r *regionRT) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reqs = append(r.reqs, rec{req.Method, req.URL.String(), req.Header.Get("Authorization"), req.Header.Get("Cookie"), req.Header.Get("X-Custom")})
	if req.Method == http.MethodHead {
		if r.failHead > 0 {
			r.failHead--
			return nil, errors.New("boom")
		}
		h := http.Header{}
		st := 200
		if r.location != "" {
			st = 308
			if r.status != 0 {
				st = r.status
			}
			h.Set("Location", r.location)
		}
		return &http.Response{StatusCode: st, Header: h, Body: http.NoBody, Request: req}, nil
	}
	return jsonResponse(200, map[string]any{"data": map[string]any{}, "meta": map[string]any{}}), nil
}

func (r *regionRT) heads() []rec {
	var out []rec
	for _, q := range r.reqs {
		if q.method == http.MethodHead {
			out = append(out, q)
		}
	}
	return out
}

func regionClient(t *testing.T, base string, rt *regionRT) *Client {
	t.Helper()
	c, kerr := NewClient(ClientOpts{
		BaseURL:        base,
		APIKey:         "sk_secret",
		ProjectSlug:    "portal",
		DefaultHeaders: map[string]string{"X-Custom": "secret-default", "Cookie": "sid=1"},
		HTTPClient:     &http.Client{Transport: rt},
	})
	if kerr != nil {
		t.Fatalf("NewClient: %v", kerr)
	}
	return c
}

func TestRegionProbeFirstThenRegionalAndCached(t *testing.T) {
	rt := &regionRT{location: "https://acme.api-de01.k9o.io/"}
	c := regionClient(t, "https://acme.api.k9o.io", rt)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if _, kerr := c.Gate.SignIn(ctx, SignInInput{Email: "a@b.c", Password: "pw"}); kerr != nil {
			t.Fatalf("SignIn: %v", kerr)
		}
	}
	if len(rt.reqs) != 3 {
		t.Fatalf("want 1 probe + 2 calls, got %d: %+v", len(rt.reqs), rt.reqs)
	}
	p := rt.reqs[0]
	if p.method != "HEAD" || p.url != "https://acme.api.k9o.io/" {
		t.Fatalf("probe wrong: %+v", p)
	}
	if p.auth != "" || p.cookie != "" || p.custom != "" {
		t.Fatalf("probe carried credentials/default headers: %+v", p)
	}
	for _, q := range rt.reqs[1:] {
		if q.method != "POST" || q.url != "https://acme.api-de01.k9o.io/@portal/gate/signin" || q.auth != "Bearer sk_secret" {
			t.Fatalf("call wrong: %+v", q)
		}
	}
	if n := len(rt.heads()); n != 1 {
		t.Fatalf("want one probe, got %d", n)
	}
}

func TestRegionStagingApexFamily(t *testing.T) {
	rt := &regionRT{location: "https://acme.api-staging-de01.k9o.io/"}
	c := regionClient(t, "https://acme.api-staging.k9o.io", rt)
	c.Gate.Whoami(context.Background())
	if got := rt.reqs[1].url; got != "https://acme.api-staging-de01.k9o.io/@portal/gate/whoami" {
		t.Fatalf("got %s", got)
	}
}

func TestRegionRelativeLocationResolvesAgainstBase(t *testing.T) {
	// A relative Location resolves to the base host itself, which is not regional: refused.
	rt := &regionRT{location: "/elsewhere"}
	c := regionClient(t, "https://acme.api.k9o.io", rt)
	c.Gate.Whoami(context.Background())
	if got := rt.reqs[1].url; !strings.HasPrefix(got, "https://acme.api.k9o.io/") {
		t.Fatalf("got %s", got)
	}
}

func TestRegionNoRedirectKeepsConfigured(t *testing.T) {
	rt := &regionRT{}
	c := regionClient(t, "https://acme.api.k9o.io", rt)
	c.Gate.Whoami(context.Background())
	c.Gate.Whoami(context.Background())
	if len(rt.heads()) != 1 {
		t.Fatalf("definite no-redirect answer must be cached, heads=%d", len(rt.heads()))
	}
	if got := rt.reqs[1].url; got != "https://acme.api.k9o.io/@portal/gate/whoami" {
		t.Fatalf("got %s", got)
	}
}

func TestRegionNon308IgnoresLocation(t *testing.T) {
	rt := &regionRT{location: "https://acme.api-de01.k9o.io/", status: 302}
	c := regionClient(t, "https://acme.api.k9o.io", rt)
	c.Gate.Whoami(context.Background())
	if got := rt.reqs[1].url; !strings.HasPrefix(got, "https://acme.api.k9o.io/") {
		t.Fatalf("got %s", got)
	}
}

func TestRegionNeverProbesNonRegionFreeHosts(t *testing.T) {
	for _, base := range []string{
		"https://acme.api-de01.k9o.io",
		"https://acme.api-staging-de01.k9o.io",
		"https://api.k9o.io",
		"https://api.example.com",
		"https://acme.api.k9o.io.evil.com",
		"https://evil-acme.api.k9o.io.example.org",
		"http://localhost:8080",
	} {
		rt := &regionRT{location: "https://acme.api-de01.k9o.io/"}
		c := regionClient(t, base, rt)
		c.Gate.Whoami(context.Background())
		if len(rt.heads()) != 0 {
			t.Errorf("%s: must not be probed", base)
		}
		if len(rt.reqs) != 1 || !strings.HasPrefix(rt.reqs[0].url, base+"/") {
			t.Errorf("%s: used as given expected, got %+v", base, rt.reqs)
		}
	}
}

func TestRegionHostileLocationsRefused(t *testing.T) {
	for name, loc := range map[string]string{
		"evil host":       "https://evil.example.com/",
		"evil suffix":     "https://acme.api-de01.k9o.io.evil.com/",
		"other org":       "https://other.api-de01.k9o.io/",
		"wrong apex":      "https://acme.api-staging-de01.k9o.io/",
		"http":            "http://acme.api-de01.k9o.io/",
		"userinfo":        "https://u:p@acme.api-de01.k9o.io/",
		"port":            "https://acme.api-de01.k9o.io:8443/",
		"long region":     "https://acme.api-deu01.k9o.io/",
		"short region":    "https://acme.api-d1.k9o.io/",
		"digits region":   "https://acme.api-1234.k9o.io/",
		"empty region":    "https://acme.api-.k9o.io/",
		"nested subhost":  "https://x.acme.api-de01.k9o.io/",
		"scheme relative": "//evil.example.com/",
	} {
		rt := &regionRT{location: loc}
		c := regionClient(t, "https://acme.api.k9o.io", rt)
		c.Gate.SignIn(context.Background(), SignInInput{Email: "a@b.c", Password: "pw"})
		last := rt.reqs[len(rt.reqs)-1]
		if !strings.HasPrefix(last.url, "https://acme.api.k9o.io/") {
			t.Errorf("%s: credentialed call went to %s", name, last.url)
		}
	}
}

func TestRegionProbeNetworkFailureNotCached(t *testing.T) {
	rt := &regionRT{location: "https://acme.api-de01.k9o.io/", failHead: 1}
	c := regionClient(t, "https://acme.api.k9o.io", rt)
	ctx := context.Background()
	c.Gate.Whoami(ctx)
	if got := rt.reqs[len(rt.reqs)-1].url; !strings.HasPrefix(got, "https://acme.api.k9o.io/") {
		t.Fatalf("first call should use configured base, got %s", got)
	}
	c.Gate.Whoami(ctx)
	if got := rt.reqs[len(rt.reqs)-1].url; !strings.HasPrefix(got, "https://acme.api-de01.k9o.io/") {
		t.Fatalf("second call should re-probe and go regional, got %s", got)
	}
	if n := len(rt.heads()); n != 2 {
		t.Fatalf("want 2 probes, got %d", n)
	}
}

func TestRegionStreamingPathUsesRegionalOrigin(t *testing.T) {
	rt := &regionRT{location: "https://acme.api-de01.k9o.io/"}
	cfg := &httpClientConfig{BaseURL: "https://acme.api.k9o.io", APIKey: "k", HTTPClient: &http.Client{Transport: sseRT{rt}}}
	events, errs := streamSSE[map[string]any](context.Background(), cfg, &requestInit{Method: "GET", Path: "/@portal/stream"})
	for range events {
	}
	for range errs {
	}
	if len(rt.reqs) != 2 || rt.reqs[0].method != "HEAD" || rt.reqs[0].auth != "" ||
		rt.reqs[1].url != "https://acme.api-de01.k9o.io/@portal/stream" {
		t.Fatalf("stream: %+v", rt.reqs)
	}
}

type sseRT struct{ inner *regionRT }

func (s sseRT) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodHead {
		return s.inner.RoundTrip(req)
	}
	s.inner.mu.Lock()
	s.inner.reqs = append(s.inner.reqs, rec{req.Method, req.URL.String(), req.Header.Get("Authorization"), "", ""})
	s.inner.mu.Unlock()
	return sseResponse("data: {\"a\":1}\n\ndata: [DONE]\n\n"), nil
}

func TestRegionProbeDoesNotChangeClientRedirectBehaviour(t *testing.T) {
	hc := &http.Client{Transport: &regionRT{}}
	cfg := &httpClientConfig{BaseURL: "https://acme.api.k9o.io", HTTPClient: hc}
	cfg.resolveOrigin(context.Background())
	if hc.CheckRedirect != nil {
		t.Fatal("configured client's CheckRedirect was modified")
	}
}
