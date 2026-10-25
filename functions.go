package k9o

import (
	"context"
	"net/url"
)

// FunctionRecord is the function record returned by gateway endpoints.
type FunctionRecord struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Runtime   string  `json:"runtime"`
	Status    string  `json:"status"`
	Tier      string  `json:"tier"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt *string `json:"updatedAt,omitempty"`
}

// DeployFunctionInput is the body for Functions.Deploy.
type DeployFunctionInput struct {
	Name    string         `json:"name"`
	Runtime string         `json:"runtime"`
	Code    string         `json:"code"`
	Tier    string         `json:"tier,omitempty"`
	Config  map[string]any `json:"config,omitempty"`
}

// FunctionsNamespace exposes client.Functions.*.
type FunctionsNamespace struct {
	config *httpClientConfig
}

// Deploy deploys or redeploys a function.
func (n *FunctionsNamespace) Deploy(ctx context.Context, input DeployFunctionInput) (*FunctionRecord, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "functions")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[FunctionRecord](ctx, n.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "functions"), Body: input})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// List returns deployed functions.
func (n *FunctionsNamespace) List(ctx context.Context, status string) ([]FunctionRecord, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "functions")
	if kerr != nil {
		return nil, nil, kerr
	}
	q := map[string]string{}
	if status != "" {
		q["status"] = status
	}
	env, kerr := doRequest[[]FunctionRecord](ctx, n.config, &requestInit{Method: "GET", Path: ProjectPath(slug, "functions"), Query: q})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Get returns a function by name.
func (n *FunctionsNamespace) Get(ctx context.Context, name string) (*FunctionRecord, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "functions")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[FunctionRecord](ctx, n.config, &requestInit{Method: "GET", Path: ProjectPath(slug, "functions/"+url.PathEscape(name))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Delete retires a function (24h name cooldown).
func (n *FunctionsNamespace) Delete(ctx context.Context, name string) (map[string]any, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "functions")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "DELETE", Path: ProjectPath(slug, "functions/"+url.PathEscape(name))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Invoke invokes a function; payload is passed to the runtime verbatim.
func (n *FunctionsNamespace) Invoke(ctx context.Context, name string, payload any) (map[string]any, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "functions")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "functions/"+url.PathEscape(name)+"/invoke"), Body: payload})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}
