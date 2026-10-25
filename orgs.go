package k9o

import (
	"context"
	"net/url"
)

// Org is the org record.
type Org struct {
	ID        string  `json:"id"`
	Slug      string  `json:"slug"`
	Name      string  `json:"name"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt *string `json:"updatedAt,omitempty"`
}

// OrgsNamespace exposes client.Orgs.* (collection-style).
// Distinct from client.Org (singular bound-org namespace).
type OrgsNamespace struct {
	config *httpClientConfig
}

// List returns orgs accessible to the caller.
func (n *OrgsNamespace) List(ctx context.Context) ([]Org, *Meta, *KError) {
	env, kerr := doRequest[[]Org](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("orgs")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Get returns a single org by slug.
func (n *OrgsNamespace) Get(ctx context.Context, orgSlug string) (*Org, *Meta, *KError) {
	env, kerr := doRequest[Org](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("orgs/" + url.PathEscape(orgSlug))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Restore restores a soft-deleted org.
func (n *OrgsNamespace) Restore(ctx context.Context, orgID string) (*Org, *Meta, *KError) {
	env, kerr := doRequest[Org](ctx, n.config, &requestInit{Method: "POST", Path: OrgPath("orgs/" + url.PathEscape(orgID) + "/restore")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Current returns the org bound to the caller's current session context.
func (n *OrgsNamespace) Current(ctx context.Context) (*Org, *Meta, *KError) {
	env, kerr := doRequest[Org](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("org")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}
