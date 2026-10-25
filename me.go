package k9o

import (
	"context"
	"net/url"
	"strconv"
)

// MeNamespace exposes client.Me.* (operator-self; cross-org).
// Sub-namespaces: Profile / Sessions / APITokens /
// Notifications / AuditLog / DataExport.
type MeNamespace struct {
	Profile       *MeProfile
	Sessions      *MeSessions
	APITokens     *MeAPITokens
	Notifications *MeNotifications
	AuditLog      *MeAuditLog
	DataExport    *MeDataExport
}

func newMeNamespace(cfg *httpClientConfig) *MeNamespace {
	return &MeNamespace{
		Profile:       &MeProfile{config: cfg},
		Sessions:      &MeSessions{config: cfg},
		APITokens:     &MeAPITokens{config: cfg},
		Notifications: &MeNotifications{config: cfg},
		AuditLog:      &MeAuditLog{config: cfg},
		DataExport:    &MeDataExport{config: cfg},
	}
}

// MeProfile — client.Me.Profile.*
type MeProfile struct{ config *httpClientConfig }

func (n *MeProfile) Get(ctx context.Context) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("me")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *MeProfile) Update(ctx context.Context, displayName string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "PATCH", Path: OrgPath("me"), Body: map[string]any{"displayName": displayName}})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// MeSessions — client.Me.Sessions.*
type MeSessions struct{ config *httpClientConfig }

func (n *MeSessions) List(ctx context.Context) ([]map[string]any, *Meta, *KError) {
	env, kerr := doRequest[[]map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("me/sessions")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *MeSessions) Revoke(ctx context.Context, sessionID string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "DELETE", Path: OrgPath("me/sessions/" + url.PathEscape(sessionID))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *MeSessions) RevokeAll(ctx context.Context) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "DELETE", Path: OrgPath("me/sessions")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// MeAPITokens — client.Me.APITokens.*
type MeAPITokens struct{ config *httpClientConfig }

// CreateAPITokenInput is the body for APITokens.Create.
type CreateAPITokenInput struct {
	Name          string `json:"name"`
	ExpiresInDays int    `json:"expiresInDays,omitempty"`
	Scope         string `json:"scope,omitempty"`
}

func (n *MeAPITokens) List(ctx context.Context) ([]map[string]any, *Meta, *KError) {
	env, kerr := doRequest[[]map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("me/api-tokens")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *MeAPITokens) Create(ctx context.Context, input CreateAPITokenInput) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "POST", Path: OrgPath("me/api-tokens"), Body: input})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *MeAPITokens) Delete(ctx context.Context, tokenID string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "DELETE", Path: OrgPath("me/api-tokens/" + url.PathEscape(tokenID))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// MeNotifications — client.Me.Notifications.*
type MeNotifications struct{ config *httpClientConfig }

func (n *MeNotifications) List(ctx context.Context, unreadOnly bool, limit int) ([]map[string]any, *Meta, *KError) {
	q := map[string]string{}
	if unreadOnly {
		q["unreadOnly"] = "true"
	}
	if limit > 0 {
		q["limit"] = strconv.Itoa(limit)
	}
	env, kerr := doRequest[[]map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("me/notifications"), Query: q})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *MeNotifications) MarkRead(ctx context.Context, notificationID string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "PATCH", Path: OrgPath("me/notifications/" + url.PathEscape(notificationID)), Body: map[string]any{"read": true}})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *MeNotifications) MarkAllRead(ctx context.Context) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "PATCH", Path: OrgPath("me/notifications"), Body: map[string]any{"read": true}})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// MeAuditLog — client.Me.AuditLog.*
type MeAuditLog struct{ config *httpClientConfig }

func (n *MeAuditLog) Query(ctx context.Context, opts AuditQueryOpts) ([]map[string]any, *Meta, *KError) {
	env, kerr := doRequest[[]map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("me/audit-log"), Query: opts.toQuery()})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// MeDataExport — client.Me.DataExport.*
type MeDataExport struct{ config *httpClientConfig }

func (n *MeDataExport) Request(ctx context.Context, format string) (map[string]any, *Meta, *KError) {
	if format == "" {
		format = "json"
	}
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "POST", Path: OrgPath("me/data-export"), Body: map[string]any{"format": format}})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *MeDataExport) Status(ctx context.Context, jobID string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("me/data-export/" + url.PathEscape(jobID))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}
