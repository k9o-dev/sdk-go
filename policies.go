package k9o

import (
	"context"
	"net/url"
)

// PolicyBundle is the policy bundle record.
type PolicyBundle struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Version     int            `json:"version"`
	Status      string         `json:"status"`
	Rules       map[string]any `json:"rules"`
	CreatedAt   string         `json:"createdAt"`
	PublishedAt *string        `json:"publishedAt,omitempty"`
	CreatedBy   string         `json:"createdBy"`
}

// PoliciesNamespace exposes client.Policies.*.
type PoliciesNamespace struct {
	config *httpClientConfig
}

// Current reads the currently-active policy bundle (nil data when none).
func (n *PoliciesNamespace) Current(ctx context.Context) (*PolicyBundle, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "policies")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[*PolicyBundle](ctx, n.config, &requestInit{Method: "GET", Path: ProjectPath(slug, "policy")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// List returns policy bundles.
func (n *PoliciesNamespace) List(ctx context.Context, status string) ([]PolicyBundle, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "policies")
	if kerr != nil {
		return nil, nil, kerr
	}
	q := map[string]string{}
	if status != "" {
		q["status"] = status
	}
	env, kerr := doRequest[[]PolicyBundle](ctx, n.config, &requestInit{Method: "GET", Path: ProjectPath(slug, "policies"), Query: q})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Create creates a draft policy bundle.
func (n *PoliciesNamespace) Create(ctx context.Context, name string, rules map[string]any) (*PolicyBundle, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "policies")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[PolicyBundle](ctx, n.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "policies"), Body: map[string]any{"name": name, "rules": rules}})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Validate validates a rules bundle without persisting it.
func (n *PoliciesNamespace) Validate(ctx context.Context, rules map[string]any) (map[string]any, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "policies")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "policies/validate"), Body: map[string]any{"rules": rules}})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Get returns a policy bundle by id.
func (n *PoliciesNamespace) Get(ctx context.Context, bundleID string) (*PolicyBundle, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "policies")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[PolicyBundle](ctx, n.config, &requestInit{Method: "GET", Path: ProjectPath(slug, "policies/"+url.PathEscape(bundleID))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Update patches a policy bundle's mutable fields.
func (n *PoliciesNamespace) Update(ctx context.Context, bundleID string, patch map[string]any) (*PolicyBundle, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "policies")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[PolicyBundle](ctx, n.config, &requestInit{Method: "PATCH", Path: ProjectPath(slug, "policies/"+url.PathEscape(bundleID)), Body: patch})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Delete deletes a policy bundle.
func (n *PoliciesNamespace) Delete(ctx context.Context, bundleID string) (map[string]any, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "policies")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "DELETE", Path: ProjectPath(slug, "policies/"+url.PathEscape(bundleID))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Publish publishes a draft bundle (makes it the active policy).
func (n *PoliciesNamespace) Publish(ctx context.Context, bundleID string) (*PolicyBundle, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "policies")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[PolicyBundle](ctx, n.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "policies/"+url.PathEscape(bundleID)+"/publish")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}
