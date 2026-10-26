package k9o

import (
	"context"
	"net/url"
)

// MeNamespace exposes client.Me.* (operator-self; cross-org).
// Sub-namespaces: Profile / Sessions / APITokens.
type MeNamespace struct {
	Profile   *MeProfile
	Sessions  *MeSessions
	APITokens *MeAPITokens
}

func newMeNamespace(cfg *httpClientConfig) *MeNamespace {
	return &MeNamespace{
		Profile:   &MeProfile{config: cfg},
		Sessions:  &MeSessions{config: cfg},
		APITokens: &MeAPITokens{config: cfg},
	}
}

// MeProfile — client.Me.Profile.*
type MeProfile struct{ config *httpClientConfig }

// Get reads the caller's profile: GET /me.
func (n *MeProfile) Get(ctx context.Context) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("me")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Update changes the caller's display name: PATCH /me with {displayName}.
func (n *MeProfile) Update(ctx context.Context, displayName string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "PATCH", Path: OrgPath("me"), Body: map[string]any{"displayName": displayName}})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// MeSessions — client.Me.Sessions.*
type MeSessions struct{ config *httpClientConfig }

// List lists the caller's sessions: GET /me/sessions.
func (n *MeSessions) List(ctx context.Context) ([]map[string]any, *Meta, *KError) {
	env, kerr := doRequest[[]map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("me/sessions")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Revoke revokes one session: POST /auth/sessions/<id>/revoke. The result
// carries {revoked}.
func (n *MeSessions) Revoke(ctx context.Context, sessionID string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "POST", Path: OrgPath("auth/sessions/" + url.PathEscape(sessionID) + "/revoke")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// RevokeAll revokes every session of the caller: POST /me/sessions/revoke-all.
// The result carries {revoked} (the count).
func (n *MeSessions) RevokeAll(ctx context.Context) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "POST", Path: OrgPath("me/sessions/revoke-all")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// MeAPITokens — client.Me.APITokens.*
type MeAPITokens struct{ config *httpClientConfig }

// APITokenScopeManifest is the scope of a personal API token. Every dimension
// is optional; an unset (nil) dimension is unconstrained. Projects must name
// exactly one project.
type APITokenScopeManifest struct {
	// Operations is any of "read", "write", "admin".
	Operations      []string            `json:"operations,omitempty"`
	Projects        []string            `json:"projects,omitempty"`
	ResourceClasses []string            `json:"resourceClasses,omitempty"`
	Collections     map[string][]string `json:"collections,omitempty"`
	Cidrs           []string            `json:"cidrs,omitempty"`
}

// CreateAPITokenInput is the body for APITokens.Create; property names are the
// gateway's.
type CreateAPITokenInput struct {
	// Label names the token (1 to 80 characters).
	Label         string                `json:"label"`
	ScopeManifest APITokenScopeManifest `json:"scopeManifest"`
	// TTLSeconds is the token lifetime; omitted uses the gateway default.
	TTLSeconds int `json:"ttlSeconds,omitempty"`
}

// List lists the caller's API tokens: GET /me/api-tokens.
func (n *MeAPITokens) List(ctx context.Context) ([]map[string]any, *Meta, *KError) {
	env, kerr := doRequest[[]map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("me/api-tokens")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Create mints an API token: POST /me/api-tokens. The result carries the token
// record plus its one-time "plaintext".
func (n *MeAPITokens) Create(ctx context.Context, input CreateAPITokenInput) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "POST", Path: OrgPath("me/api-tokens"), Body: input})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Delete revokes an API token: DELETE /me/api-tokens/<id>.
func (n *MeAPITokens) Delete(ctx context.Context, tokenID string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "DELETE", Path: OrgPath("me/api-tokens/" + url.PathEscape(tokenID))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}
