package k9o

import (
	"context"
	"net/url"
)

// PolicyBundle is the policy bundle record the gateway returns.
type PolicyBundle struct {
	ID           string         `json:"id"`
	ProjectID    string         `json:"projectId"`
	Status       string         `json:"status"`
	Policy       map[string]any `json:"policy"`
	Rules        any            `json:"rules,omitempty"`
	CreatedBy    string         `json:"createdBy"`
	CreatedAt    string         `json:"createdAt"`
	PublishedAt  *string        `json:"publishedAt"`
	SupersededAt *string        `json:"supersededAt"`
	SupersededBy *string        `json:"supersededBy"`
	Source       string         `json:"source"`
	// EditorMode is "visual", "json" or "unspecified".
	EditorMode       string  `json:"editorMode"`
	SupersededReason *string `json:"supersededReason"`
}

// CreatePolicyInput is the body of Policies.Create. Policy is the policy
// document: sections keyed by resource kind ("collections", "storage",
// "tools", "functions", "secrets", "streams", "analytics"); other keys are
// allowed.
type CreatePolicyInput struct {
	Policy map[string]any `json:"policy"`
	Rules  any            `json:"rules,omitempty"`
	// EditorMode is "visual", "json" or "unspecified".
	EditorMode string `json:"editorMode,omitempty"`
}

// UpdatePolicyInput is the body of Policies.Update: it replaces a draft
// bundle's policy document.
type UpdatePolicyInput struct {
	Policy     map[string]any `json:"policy"`
	Rules      any            `json:"rules,omitempty"`
	EditorMode string         `json:"editorMode,omitempty"`
}

// ValidatePolicyInput is the body of Policies.Validate.
type ValidatePolicyInput struct {
	Policy map[string]any `json:"policy"`
	Rules  any            `json:"rules,omitempty"`
}

// ValidatePolicyResult is the result of Policies.Validate.
type ValidatePolicyResult struct {
	Valid    bool               `json:"valid"`
	Errors   []PolicyDiagnostic `json:"errors"`
	Warnings []PolicyDiagnostic `json:"warnings"`
}

// PolicyDiagnostic is one error or warning of a validated policy.
type PolicyDiagnostic struct {
	Rule    string `json:"rule"`
	Message string `json:"message"`
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
func (n *PoliciesNamespace) Create(ctx context.Context, input CreatePolicyInput) (*PolicyBundle, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "policies")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[PolicyBundle](ctx, n.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "policies"), Body: input})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Validate lints a candidate policy document without persisting it.
func (n *PoliciesNamespace) Validate(ctx context.Context, input ValidatePolicyInput) (*ValidatePolicyResult, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "policies")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[ValidatePolicyResult](ctx, n.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "policies/validate"), Body: input})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
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

// Update replaces a draft bundle's policy document. Published bundles cannot be
// patched (publish, then republish).
func (n *PoliciesNamespace) Update(ctx context.Context, bundleID string, patch UpdatePolicyInput) (*PolicyBundle, *Meta, *KError) {
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

// Delete deletes a draft policy bundle.
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
