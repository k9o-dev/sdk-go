package k9o

import (
	"context"
	"net/url"
	"strconv"
)

// FunctionRecord is the function record returned by gateway endpoints.
type FunctionRecord struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Runtime     string  `json:"runtime"`
	Tier        string  `json:"tier"`
	EntryPoint  *string `json:"entryPoint"`
	Version     string  `json:"version"`
	Status      string  `json:"status"`
	Source      string  `json:"source"`
	Description *string `json:"description"`
	CreatedAt   string  `json:"createdAt"`
	DeployedAt  *string `json:"deployedAt"`
	RetiredAt   *string `json:"retiredAt"`
	ErrorDetail *string `json:"errorDetail"`
}

// DeployFunctionInput is the body for Functions.Deploy. The function's bytes
// are uploaded first (UploadArtifact returns the ArtifactKey); deploy
// references that artifact.
type DeployFunctionInput struct {
	// Name is the function name (1 to 128 characters).
	Name string `json:"name"`
	// Runtime is the function runtime, e.g. "bun", "python", "go", "rust".
	Runtime string `json:"runtime"`
	// Tier is "gateway" (WebAssembly in the gateway) or "sandbox" (an OCI image archive).
	Tier string `json:"tier"`
	// ArtifactKey is the content-addressed key UploadArtifact returned.
	ArtifactKey string `json:"artifactKey"`
	// Version is the version label for this deploy (1 to 64 characters).
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

// ListFunctionsOptions are the optional query filters of Functions.List.
type ListFunctionsOptions struct {
	Limit  int
	Cursor string
	// Status is "deploying", "active", "error" or "retired".
	Status string
	Source string
	Tier   string
}

// InvokeOptions are the optional invoke controls; field names are the
// gateway's body properties.
type InvokeOptions struct {
	// Background runs the function asynchronously instead of waiting for the result.
	Background bool              `json:"background,omitempty"`
	Delegation *InvokeDelegation `json:"delegation,omitempty"`
	OnBehalfOf string            `json:"onBehalfOf,omitempty"`
	ConfinedTo string            `json:"confinedTo,omitempty"`
}

// InvokeDelegation carries a delegation token for an invoke.
type InvokeDelegation struct {
	Token           string `json:"token"`
	InvokeSessionID string `json:"invokeSessionId,omitempty"`
}

// ArtifactUpload is the result of UploadArtifact.
type ArtifactUpload struct {
	ArtifactKey string `json:"artifactKey"`
}

// FunctionsNamespace exposes client.Functions.*.
type FunctionsNamespace struct {
	config *httpClientConfig
}

// UploadArtifact uploads a function artifact and returns its content-addressed
// ArtifactKey, to pass to Deploy: POST /@<slug>/functions/artifacts with the
// raw bytes. kind is "wasm" (a WebAssembly module for the gateway tier, up to
// 16 MiB) or "tar" (an OCI image archive for the sandbox tier, up to 256 MiB).
// Uploading the same bytes again returns the same key.
func (n *FunctionsNamespace) UploadArtifact(ctx context.Context, data []byte, kind string) (*ArtifactUpload, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "functions")
	if kerr != nil {
		return nil, nil, kerr
	}
	var contentType string
	switch kind {
	case "wasm":
		contentType = "application/wasm"
	case "tar":
		contentType = "application/x-tar"
	default:
		return nil, nil, &KError{
			Code:    "K9O_INVALID_FIELD_VALUE",
			Message: `artifact kind must be "wasm" or "tar"`,
			Context: map[string]any{"field": "kind", "reason": "enum"},
		}
	}
	if data == nil {
		data = []byte{}
	}
	env, kerr := doRequest[ArtifactUpload](ctx, n.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "functions/artifacts"), RawBody: data, RawContentType: contentType})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Deploy deploys or redeploys a function from an uploaded artifact. The record
// comes back with status "deploying"; the function is ready when it flips to "active".
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

// List returns deployed functions (cursor pagination).
func (n *FunctionsNamespace) List(ctx context.Context, opts ListFunctionsOptions) ([]FunctionRecord, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "functions")
	if kerr != nil {
		return nil, nil, kerr
	}
	q := map[string]string{}
	if opts.Limit > 0 {
		q["limit"] = strconv.Itoa(opts.Limit)
	}
	if opts.Cursor != "" {
		q["cursor"] = opts.Cursor
	}
	if opts.Status != "" {
		q["status"] = opts.Status
	}
	if opts.Source != "" {
		q["source"] = opts.Source
	}
	if opts.Tier != "" {
		q["tier"] = opts.Tier
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

// Delete retires a function. The result carries retired and cooldownUntil; the
// name enters a 24h cooldown.
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

// Invoke invokes a function. payload is sent as the "payload" field of the
// request body and passed to the function runtime verbatim (omitted when nil);
// the optional InvokeOptions add the gateway's other invoke controls.
func (n *FunctionsNamespace) Invoke(ctx context.Context, name string, payload any, opts ...InvokeOptions) (map[string]any, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "functions")
	if kerr != nil {
		return nil, nil, kerr
	}
	body := map[string]any{}
	if len(opts) > 0 {
		raw, err := jsonMarshalToMap(opts[0])
		if err != nil {
			return nil, nil, &KError{Code: "K9O_INTERNAL_ERROR", Message: "failed to marshal invoke options: " + err.Error()}
		}
		body = raw
	}
	if payload != nil {
		body["payload"] = payload
	}
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "functions/"+url.PathEscape(name)+"/invoke"), Body: body})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}
