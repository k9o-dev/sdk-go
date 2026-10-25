package k9o

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

// Regional host resolution. The region-free org host
// `<org>.api[-staging].k9o.io` redirects GET/HEAD only and refuses requests
// carrying a body, Authorization or cookie, so the SDK learns the region with
// ONE unauthenticated HEAD against it (redirects NOT followed), takes the 308
// Location, caches the regional origin and sends every request there.

var (
	regionFreeOrgHost = regexp.MustCompile(`^([a-z][a-z0-9-]{1,62})\.(api|api-staging)\.k9o\.io$`)
	regionCode        = regexp.MustCompile(`^[a-z]{2}\d{2}$`)
)

// regionState is the per-client cache of the resolved origin.
type regionState struct {
	mu     sync.Mutex
	done   bool
	origin string
}

// regionalOriginFromLocation returns the regional origin a Location may name
// for host, or "" if it is not exactly that. Pure.
func regionalOriginFromLocation(host, location, base string) string {
	m := regionFreeOrgHost.FindStringSubmatch(host)
	if m == nil {
		return ""
	}
	b, err := url.Parse(base)
	if err != nil {
		return ""
	}
	l, err := url.Parse(location)
	if err != nil {
		return ""
	}
	u := b.ResolveReference(l)
	if u.Scheme != "https" || u.User != nil || u.Port() != "" || strings.Contains(u.Host, ":") {
		return ""
	}
	org, apex := m[1], m[2]
	prefix := org + "." + apex + "-"
	h := u.Hostname()
	if !strings.HasPrefix(h, prefix) || !strings.HasSuffix(h, ".k9o.io") {
		return ""
	}
	rgn := strings.TrimSuffix(strings.TrimPrefix(h, prefix), ".k9o.io")
	if !regionCode.MatchString(rgn) {
		return ""
	}
	return "https://" + h
}

// probeClient returns a copy of the configured client that never follows
// redirects and never sends cookies. The configured client is not modified.
func probeClient(hc *http.Client) *http.Client {
	if hc == nil {
		hc = http.DefaultClient
	}
	c := *hc
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	c.Jar = nil
	return &c
}

// resolveOrigin returns the origin requests go to: the cached regional origin
// once learned, else the configured BaseURL.
func (c *httpClientConfig) resolveOrigin(ctx context.Context) string {
	configured := c.BaseURL
	c.region.mu.Lock()
	defer c.region.mu.Unlock()
	if c.region.done {
		return c.region.origin
	}
	u, err := url.Parse(configured)
	if err != nil || !regionFreeOrgHost.MatchString(u.Hostname()) {
		c.region.done, c.region.origin = true, configured
		return configured
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, strings.TrimRight(configured, "/")+"/", nil)
	if err != nil {
		return configured
	}
	req.Header.Set("Accept", "*/*")
	resp, err := probeClient(c.HTTPClient).Do(req)
	if err != nil {
		return configured // network failure: not cached
	}
	defer resp.Body.Close()
	origin := configured
	if resp.StatusCode == http.StatusPermanentRedirect {
		if o := regionalOriginFromLocation(u.Hostname(), resp.Header.Get("Location"), configured); o != "" {
			origin = o
		}
	}
	c.region.done, c.region.origin = true, origin
	return origin
}
