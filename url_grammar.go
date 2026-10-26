package k9o

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var slugRegex = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}$`)

// ValidateBaseURL validates a baseUrl. Returns the
// normalised form (trailing slash stripped) on success; returns
// *KError(K9O_INVALID_BASE_URL) on violation.
func ValidateBaseURL(baseURL string) (string, *KError) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", &KError{
			Code:    "K9O_INVALID_BASE_URL",
			Message: fmt.Sprintf("baseURL %q is not a valid URL: %v", baseURL, err),
			Context: map[string]any{"baseURL": baseURL},
		}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", &KError{
			Code:    "K9O_INVALID_BASE_URL",
			Message: fmt.Sprintf("baseURL %q uses unsupported scheme %q; expected http or https", baseURL, parsed.Scheme),
			Context: map[string]any{"baseURL": baseURL, "scheme": parsed.Scheme},
		}
	}
	if parsed.Host == "" {
		return "", &KError{
			Code:    "K9O_INVALID_BASE_URL",
			Message: fmt.Sprintf("baseURL %q missing host", baseURL),
			Context: map[string]any{"baseURL": baseURL},
		}
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", &KError{
			Code:    "K9O_INVALID_BASE_URL",
			Message: fmt.Sprintf("baseURL %q must not contain a path segment; got %q", baseURL, parsed.Path),
			Context: map[string]any{"baseURL": baseURL, "path": parsed.Path},
		}
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", &KError{
			Code:    "K9O_INVALID_BASE_URL",
			Message: fmt.Sprintf("baseURL %q must not contain query or fragment", baseURL),
			Context: map[string]any{"baseURL": baseURL},
		}
	}
	return fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host), nil
}

// DefaultBaseURL is the convenience default when no BaseURL is given. It is a
// default value only: it is never used to parse or match a host.
const DefaultBaseURL = "https://api.k9o.io"

// apiHost is the structure of a gateway API host, read from the host itself.
type apiHost struct {
	// OrgSlug is the first label of an org host (`acme` in
	// `acme.api.corp.internal`); "" for a platform host.
	OrgSlug string
	// Family is "api" or "api-staging".
	Family string
	// Region is the 4-character region code of a regional host (`api-de01`);
	// "" for a region-free host.
	Region string
	// Apex is everything after the gateway label; never matched against a list.
	Apex string
}

var apiLabelRegex = regexp.MustCompile(`^(api|api-staging)(?:-([a-z]{2}\d{2}))?$`)

// parseAPIHost reads the org slug and region out of a gateway host without
// knowing its apex: the gateway label is `api` or `api-staging`, optionally
// followed by `-<rgn>`; the label before it is the org slug and whatever
// follows is the apex. Returns nil for any other shape (localhost, an IP, a
// host with no gateway label); callers use such a host exactly as given.
func parseAPIHost(host string) *apiHost {
	labels := strings.Split(strings.ToLower(host), ".")
	for _, l := range labels {
		if l == "" {
			return nil
		}
	}
	mk := func(gatewayLabel, org string, rest []string) *apiHost {
		m := apiLabelRegex.FindStringSubmatch(gatewayLabel)
		if m == nil || len(rest) == 0 {
			return nil
		}
		return &apiHost{OrgSlug: org, Family: m[1], Region: m[2], Apex: strings.Join(rest, ".")}
	}
	if len(labels) >= 3 && slugRegex.MatchString(labels[0]) && !strings.HasSuffix(labels[0], "-") && apiLabelRegex.MatchString(labels[1]) {
		return mk(labels[1], labels[0], labels[2:])
	}
	if len(labels) >= 2 && apiLabelRegex.MatchString(labels[0]) {
		return mk(labels[0], "", labels[1:])
	}
	return nil
}

// ResolveBaseURL resolves the effective baseURL. baseURL is OPTIONAL: empty
// defaults to DefaultBaseURL. When the (validated) host is a region-free
// platform host (`api.<apex>` or `api-staging.<apex>`, any apex) and orgSlug is
// supplied, the org host `<orgSlug>.<host>` is composed. Any other host (an org
// host already, an internal or air-gapped host, localhost) passes through
// unchanged; orgSlug is then only sent as a header.
func ResolveBaseURL(baseURL, orgSlug string) (string, *KError) {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	validated, kerr := ValidateBaseURL(baseURL)
	if kerr != nil {
		return "", kerr
	}
	parsed, err := url.Parse(validated)
	if err != nil {
		return "", &KError{
			Code:    "K9O_INVALID_BASE_URL",
			Message: fmt.Sprintf("baseURL %q is not a valid URL: %v", validated, err),
			Context: map[string]any{"baseURL": validated},
		}
	}
	if h := parseAPIHost(parsed.Hostname()); h != nil && h.OrgSlug == "" && h.Region == "" && orgSlug != "" {
		if kerr := ValidateSlug(orgSlug, "org"); kerr != nil {
			return "", kerr
		}
		return fmt.Sprintf("%s://%s.%s", parsed.Scheme, orgSlug, parsed.Host), nil
	}
	return validated, nil
}

// ValidateSlug validates an org/project slug.
func ValidateSlug(slug, kind string) *KError {
	if !slugRegex.MatchString(slug) || strings.HasSuffix(slug, "-") {
		return &KError{
			Code:    "K9O_INVALID_FIELD_VALUE",
			Message: fmt.Sprintf("%sSlug %q does not match required format ^[a-z][a-z0-9-]{1,62}$ (must not end with hyphen)", kind, slug),
			Context: map[string]any{"slug": slug, "kind": kind},
		}
	}
	return nil
}

// ProjectPath builds a member-plane URL path:
// /@<slug>/<sub-resource>.
func ProjectPath(projectSlug, subResource string) string {
	cleaned := strings.TrimPrefix(subResource, "/")
	return fmt.Sprintf("/@%s/%s", projectSlug, cleaned)
}

// OrgPath builds an org-plane path: /<resource>.
func OrgPath(resource string) string {
	return "/" + strings.TrimPrefix(resource, "/")
}

// PlatformPath builds a platform-plane path: /<resource>.
func PlatformPath(resource string) string {
	return "/" + strings.TrimPrefix(resource, "/")
}
