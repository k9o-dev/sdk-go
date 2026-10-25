package k9o

import "testing"

func TestValidateBaseURLAccepts(t *testing.T) {
	cases := map[string]string{
		"https://api.k9o.io":          "https://api.k9o.io",
		"https://acme.api.k9o.io":     "https://acme.api.k9o.io",
		"http://localhost:8080":       "http://localhost:8080",
		"https://api.k9o.io/":         "https://api.k9o.io",
	}
	for input, expected := range cases {
		got, kerr := ValidateBaseURL(input)
		if kerr != nil {
			t.Errorf("ValidateBaseURL(%q) unexpected error: %v", input, kerr)
			continue
		}
		if got != expected {
			t.Errorf("ValidateBaseURL(%q): got %q, want %q", input, got, expected)
		}
	}
}

func TestValidateBaseURLRejects(t *testing.T) {
	rejects := []string{
		"https://api.k9o.io/v2",
		"api.k9o.io",
		"wss://api.k9o.io",
		"https://api.k9o.io?x=1",
	}
	for _, input := range rejects {
		_, kerr := ValidateBaseURL(input)
		if kerr == nil {
			t.Errorf("ValidateBaseURL(%q) should have errored", input)
		} else if kerr.Code != "K9O_INVALID_BASE_URL" {
			t.Errorf("ValidateBaseURL(%q) wrong code: %q", input, kerr.Code)
		}
	}
}

func TestResolveBaseURL(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		orgSlug string
		want    string
	}{
		{"omitted+orgSlug derives subdomain", "", "acme", "https://acme.api.k9o.io"},
		{"explicit apex+orgSlug derives subdomain", "https://api.k9o.io", "acme", "https://acme.api.k9o.io"},
		{"apex with trailing slash + orgSlug", "https://api.k9o.io/", "member-portal", "https://member-portal.api.k9o.io"},
		{"omitted no orgSlug -> apex default", "", "", "https://api.k9o.io"},
		{"apex no orgSlug -> unchanged", "https://api.k9o.io", "", "https://api.k9o.io"},
		{"explicit subdomain -> unchanged", "https://acme.api.k9o.io", "acme", "https://acme.api.k9o.io"},
		{"explicit subdomain ignores orgSlug mismatch", "https://acme.api.k9o.io", "other", "https://acme.api.k9o.io"},
		{"self-hosted host -> unchanged", "https://platform.acme.internal", "acme", "https://platform.acme.internal"},
		{"dev host -> unchanged", "http://localhost:8080", "acme", "http://localhost:8080"},
		{"staging apex+orgSlug derives subdomain", "https://api-staging.k9o.io", "acme", "https://acme.api-staging.k9o.io"},
		{"explicit staging subdomain -> unchanged", "https://acme.api-staging.k9o.io", "acme", "https://acme.api-staging.k9o.io"},
		{"staging apex no orgSlug -> unchanged", "https://api-staging.k9o.io", "", "https://api-staging.k9o.io"},
	}
	for _, tc := range cases {
		got, kerr := ResolveBaseURL(tc.baseURL, tc.orgSlug)
		if kerr != nil {
			t.Errorf("%s: ResolveBaseURL(%q, %q) unexpected error: %v", tc.name, tc.baseURL, tc.orgSlug, kerr)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: ResolveBaseURL(%q, %q) = %q, want %q", tc.name, tc.baseURL, tc.orgSlug, got, tc.want)
		}
	}
}

func TestResolveBaseURLRejects(t *testing.T) {
	// Invalid baseURL passthrough of validation error.
	if _, kerr := ResolveBaseURL("https://api.k9o.io/v2", "acme"); kerr == nil || kerr.Code != "K9O_INVALID_BASE_URL" {
		t.Errorf("ResolveBaseURL with path should error K9O_INVALID_BASE_URL, got %v", kerr)
	}
	// Invalid orgSlug on the apex derive path.
	if _, kerr := ResolveBaseURL("https://api.k9o.io", "Acme"); kerr == nil || kerr.Code != "K9O_INVALID_FIELD_VALUE" {
		t.Errorf("ResolveBaseURL with bad orgSlug should error K9O_INVALID_FIELD_VALUE, got %v", kerr)
	}
	// Invalid orgSlug on the staging apex derive path.
	if _, kerr := ResolveBaseURL("https://api-staging.k9o.io", "Bad_Slug"); kerr == nil || kerr.Code != "K9O_INVALID_FIELD_VALUE" {
		t.Errorf("ResolveBaseURL with bad orgSlug on staging apex should error K9O_INVALID_FIELD_VALUE, got %v", kerr)
	}
}

func TestValidateSlug(t *testing.T) {
	accepts := []string{"acme", "member-portal", "a1", "a-1-z"}
	for _, s := range accepts {
		if kerr := ValidateSlug(s, "test"); kerr != nil {
			t.Errorf("ValidateSlug(%q) unexpected error: %v", s, kerr)
		}
	}
	rejects := []string{"Acme", "1acme", "-acme", "acme-", "a"}
	for _, s := range rejects {
		if kerr := ValidateSlug(s, "test"); kerr == nil {
			t.Errorf("ValidateSlug(%q) should have errored", s)
		}
	}
}

func TestProjectPath(t *testing.T) {
	got := ProjectPath("member-portal", "data/posts")
	want := "/@member-portal/data/posts"
	if got != want {
		t.Errorf("ProjectPath: got %q, want %q", got, want)
	}
	got = ProjectPath("acme", "/project")
	if got != "/@acme/project" {
		t.Errorf("ProjectPath strip leading slash: got %q", got)
	}
}

func TestOrgPath(t *testing.T) {
	if got := OrgPath("projects"); got != "/projects" {
		t.Errorf("OrgPath: got %q", got)
	}
}
