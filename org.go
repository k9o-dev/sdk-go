package k9o

import (
	"context"
	"net/url"
)

// OrgNamespace exposes client.Org.* — the singular bound-org namespace
// (distinct from client.Orgs collection). Members /
// Groups / SMTP / AuditLog / Gate (roles + IdP providers) + DefaultLLM.
type OrgNamespace struct {
	config       *httpClientConfig
	Members      *OrgMembers
	Groups       *OrgGroups
	SMTP         *OrgSMTP
	AuditLog     *OrgAuditLog
	Gate         *OrgGate
	DefaultLLMNs *OrgDefaultLLM
}

func newOrgNamespace(cfg *httpClientConfig) *OrgNamespace {
	return &OrgNamespace{
		config:       cfg,
		Members:      &OrgMembers{config: cfg},
		Groups:       &OrgGroups{config: cfg},
		SMTP:         &OrgSMTP{config: cfg},
		AuditLog:     &OrgAuditLog{config: cfg},
		Gate:         &OrgGate{Roles: &OrgRoles{config: cfg}, IdpProviders: &OrgIdpProviders{config: cfg}},
		DefaultLLMNs: &OrgDefaultLLM{config: cfg},
	}
}

// Get reads the current org (slug + name + status + settings).
func (n *OrgNamespace) Get(ctx context.Context) (*Org, *Meta, *KError) {
	env, kerr := doRequest[Org](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("org")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// DefaultLLMGet reads the org-default LLM model id (empty when unset).
func (n *OrgNamespace) DefaultLLMGet(ctx context.Context) (string, *Meta, *KError) {
	return n.DefaultLLMNs.Get(ctx)
}

// DefaultLLMSet sets the org-default LLM (ORG_ADMIN; tier-gated).
func (n *OrgNamespace) DefaultLLMSet(ctx context.Context, modelID string) (string, *Meta, *KError) {
	return n.DefaultLLMNs.Set(ctx, modelID)
}

// OrgMembers — client.Org.Members.*
type OrgMembers struct{ config *httpClientConfig }

func (n *OrgMembers) List(ctx context.Context, status string) ([]map[string]any, *Meta, *KError) {
	q := map[string]string{}
	if status != "" {
		q["status"] = status
	}
	env, kerr := doRequest[[]map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("members"), Query: q})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *OrgMembers) Invite(ctx context.Context, email, role string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "POST", Path: OrgPath("members"), Body: map[string]any{"email": email, "role": role}})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *OrgMembers) Get(ctx context.Context, userID string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("members/" + url.PathEscape(userID))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *OrgMembers) Remove(ctx context.Context, userID string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "DELETE", Path: OrgPath("members/" + url.PathEscape(userID))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// OrgGroups — client.Org.Groups.*
type OrgGroups struct{ config *httpClientConfig }

func (n *OrgGroups) List(ctx context.Context) ([]map[string]any, *Meta, *KError) {
	env, kerr := doRequest[[]map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("groups")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *OrgGroups) Create(ctx context.Context, name string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "POST", Path: OrgPath("groups"), Body: map[string]any{"name": name}})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *OrgGroups) Get(ctx context.Context, groupID string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("groups/" + url.PathEscape(groupID))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *OrgGroups) Delete(ctx context.Context, groupID string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "DELETE", Path: OrgPath("groups/" + url.PathEscape(groupID))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// OrgAuditLog — client.Org.AuditLog.* (org-plane; distinct from project Audit).
type OrgAuditLog struct{ config *httpClientConfig }

func (n *OrgAuditLog) Query(ctx context.Context, opts AuditQueryOpts) ([]AuditEvent, *Meta, *KError) {
	env, kerr := doRequest[[]AuditEvent](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("audit-log"), Query: opts.toQuery()})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// OrgGate — client.Org.Gate.*
type OrgGate struct {
	Roles        *OrgRoles
	IdpProviders *OrgIdpProviders
}

// OrgRoles — client.Org.Gate.Roles.*
type OrgRoles struct{ config *httpClientConfig }

func (n *OrgRoles) List(ctx context.Context, userID, role string) ([]map[string]any, *Meta, *KError) {
	q := map[string]string{}
	if userID != "" {
		q["userId"] = userID
	}
	if role != "" {
		q["role"] = role
	}
	env, kerr := doRequest[[]map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("org-role-grants"), Query: q})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *OrgRoles) Grant(ctx context.Context, userID, role string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "POST", Path: OrgPath("org-role-grants"), Body: map[string]any{"userId": userID, "role": role}})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *OrgRoles) Get(ctx context.Context, grantID string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("org-role-grants/" + url.PathEscape(grantID))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *OrgRoles) Revoke(ctx context.Context, grantID string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "DELETE", Path: OrgPath("org-role-grants/" + url.PathEscape(grantID))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// OrgIdpProviders — client.Org.Gate.IdpProviders.*
type OrgIdpProviders struct{ config *httpClientConfig }

func (n *OrgIdpProviders) List(ctx context.Context) ([]map[string]any, *Meta, *KError) {
	env, kerr := doRequest[[]map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("idp-providers")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *OrgIdpProviders) Create(ctx context.Context, name, kind string, config map[string]any) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "POST", Path: OrgPath("idp-providers"), Body: map[string]any{"name": name, "kind": kind, "config": config}})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *OrgIdpProviders) Get(ctx context.Context, idpID string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("idp-providers/" + url.PathEscape(idpID))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *OrgIdpProviders) Configure(ctx context.Context, idpID string, patch map[string]any) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "PATCH", Path: OrgPath("idp-providers/" + url.PathEscape(idpID)), Body: patch})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *OrgIdpProviders) Delete(ctx context.Context, idpID string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "DELETE", Path: OrgPath("idp-providers/" + url.PathEscape(idpID))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// OrgSMTP — client.Org.SMTP.*
type OrgSMTP struct{ config *httpClientConfig }

// SetSMTPInput is the body for OrgSMTP.Set (password routed via Kibo enclave).
type SetSMTPInput struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	FromEmail    string `json:"fromEmail"`
	FromName     string `json:"fromName"`
	Encryption   string `json:"encryption"`
	ReplyToEmail string `json:"replyToEmail,omitempty"`
}

func (n *OrgSMTP) Get(ctx context.Context) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("org/smtp")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *OrgSMTP) Set(ctx context.Context, input SetSMTPInput) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "PATCH", Path: OrgPath("org"), Body: map[string]any{"smtpConfig": input}})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *OrgSMTP) Test(ctx context.Context, to, fromOverride string) (map[string]any, *Meta, *KError) {
	body := map[string]any{"to": to}
	if fromOverride != "" {
		body["fromOverride"] = fromOverride
	}
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "POST", Path: OrgPath("org/smtp/test-email"), Body: body})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *OrgSMTP) Unset(ctx context.Context) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "PATCH", Path: OrgPath("org"), Body: map[string]any{"smtpConfig": nil}})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// OrgDefaultLLM — client.Org.DefaultLLMNs.*
type OrgDefaultLLM struct{ config *httpClientConfig }

func (n *OrgDefaultLLM) Get(ctx context.Context) (string, *Meta, *KError) {
	env, kerr := doRequest[string](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("org/default-llm")})
	if kerr != nil {
		return "", nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func (n *OrgDefaultLLM) Set(ctx context.Context, modelID string) (string, *Meta, *KError) {
	env, kerr := doRequest[string](ctx, n.config, &requestInit{Method: "PATCH", Path: OrgPath("org/default-llm"), Body: map[string]any{"modelId": modelID}})
	if kerr != nil {
		return "", nil, kerr
	}
	return env.Data, &env.Meta, nil
}
