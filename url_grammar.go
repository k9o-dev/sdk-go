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

// k9oOrgDerivableApexes are the bare platform apex hosts org-subdomain
// derivation applies to — production and staging share the
// "<orgSlug>.<apex>" wire-routing scheme (confirmed live). A third apex
// needs the same confirmation before joining this set.
var k9oOrgDerivableApexes = map[string]bool{
	"api.k9o.io":         true,
	"api-staging.k9o.io": true,
}

// ResolveBaseURL resolves the effective baseURL.
// baseURL is OPTIONAL — an empty string defaults to the
// platform apex "https://api.k9o.io". When the (validated) host is a bare
// apex in k9oOrgDerivableApexes and orgSlug is supplied, the SDK composes
// the canonical org-subdomain wire root "https://<orgSlug>.<apex>"
//. An explicit "<orgSlug>" subdomain, a self-hosted /
// Sovereign host, or a dev host (anything not in that set) passes through
// unchanged — orgSlug is intentionally NOT composed onto a non-apex host.
// Existing baseURL validation is preserved.
//
// Fixed 2026-08-30 (SDK-2): this used to silently ignore orgSlug for
// "api-staging.k9o.io" too, not just genuine self-hosted/custom hosts,
// since only the literal production apex string was checked.
func ResolveBaseURL(baseURL, orgSlug string) (string, *KError) {
	if baseURL == "" {
		baseURL = "https://api.k9o.io"
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
	if k9oOrgDerivableApexes[parsed.Hostname()] && orgSlug != "" {
		if kerr := ValidateSlug(orgSlug, "org"); kerr != nil {
			return "", kerr
		}
		return fmt.Sprintf("%s://%s.%s", parsed.Scheme, orgSlug, parsed.Hostname()), nil
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
