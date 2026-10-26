package k9o

import (
	"context"
	"net/url"
	"strconv"
)

// OrgNamespace exposes client.Org.* — the singular bound-org namespace
// (distinct from client.Orgs collection). Members / Groups / SMTP / AuditLog /
// Gate (roles + IdP providers) + DefaultLLM.
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

// DefaultLLMGet reads the org-default LLM: GET /default-llm.
func (n *OrgNamespace) DefaultLLMGet(ctx context.Context) (*OrgDefaultLLMRecord, *Meta, *KError) {
	return n.DefaultLLMNs.Get(ctx)
}

// DefaultLLMSet sets the org-default LLM (ORG_ADMIN or ORG_OWNER; tier-gated).
func (n *OrgNamespace) DefaultLLMSet(ctx context.Context, modelID string) (*OrgDefaultLLMSetResult, *Meta, *KError) {
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

func (n *OrgGroups) Get(ctx context.Context, groupID string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("groups/" + url.PathEscape(groupID))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// OrgAuditLog — client.Org.AuditLog.* (org-plane; distinct from project Audit).
type OrgAuditLog struct{ config *httpClientConfig }

// OrgAuditQueryOpts are the filters of OrgAuditLog.Query; the org-plane route
// reads only these two.
type OrgAuditQueryOpts struct {
	// Type filters by event type.
	Type string
	// Limit is the page size (1 to 500, default 100).
	Limit int
}

// Query reads the org audit log: GET /audit-log.
func (n *OrgAuditLog) Query(ctx context.Context, opts OrgAuditQueryOpts) ([]AuditEvent, *Meta, *KError) {
	q := map[string]string{}
	if opts.Type != "" {
		q["type"] = opts.Type
	}
	if opts.Limit > 0 {
		q["limit"] = strconv.Itoa(opts.Limit)
	}
	env, kerr := doRequest[[]AuditEvent](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("audit-log"), Query: q})
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

// List lists role grants: GET /role-grants.
func (n *OrgRoles) List(ctx context.Context) ([]map[string]any, *Meta, *KError) {
	env, kerr := doRequest[[]map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("role-grants")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// GrantRoleInput is the body of OrgRoles.Grant.
type GrantRoleInput struct {
	UserID string `json:"userId"`
	// Role is "ORG_OWNER", "ORG_ADMIN", "ORG_BILLING_ADMIN", "ORG_AUDITOR" or "ORG_MEMBER".
	Role           string `json:"role"`
	DecisionReason string `json:"decisionReason,omitempty"`
}

// Grant grants an org role: POST /role-grants.
func (n *OrgRoles) Grant(ctx context.Context, input GrantRoleInput) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "POST", Path: OrgPath("role-grants"), Body: input})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Get reads one grant: GET /role-grants/<id>.
func (n *OrgRoles) Get(ctx context.Context, grantID string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("role-grants/" + url.PathEscape(grantID))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Revoke revokes a grant (soft delete): DELETE /role-grants/<id>. A non-empty
// decisionReason is sent as the request body {decisionReason}.
func (n *OrgRoles) Revoke(ctx context.Context, grantID string, decisionReason string) (map[string]any, *Meta, *KError) {
	var body any
	if decisionReason != "" {
		body = map[string]any{"decisionReason": decisionReason}
	}
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "DELETE", Path: OrgPath("role-grants/" + url.PathEscape(grantID)), Body: body})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// OrgIdpProviders — client.Org.Gate.IdpProviders.*
type OrgIdpProviders struct{ config *httpClientConfig }

// CreateIdpInput is the body of OrgIdpProviders.Create. ClientSecretRef is a
// reference to a stored secret, never the secret itself.
type CreateIdpInput struct {
	// IdpProvider is the provider kind: "google", "github", "microsoft", "oidc",
	// "saml", "scim", "ldap", "active_directory", "apple", "facebook" or "twitter".
	IdpProvider     string `json:"idpProvider"`
	DisplayName     string `json:"displayName"`
	ClientID        string `json:"clientId"`
	ClientSecretRef string `json:"clientSecretRef"`
	// TierGating is "free_tier_ok" or "ridge_plus".
	TierGating   string         `json:"tierGating"`
	RedirectURI  string         `json:"redirectUri,omitempty"`
	MetadataURL  *string        `json:"metadataUrl,omitempty"`
	AttributeMap map[string]any `json:"attributeMap,omitempty"`
	Enabled      *bool          `json:"enabled,omitempty"`
}

// UpdateIdpInput is the body of OrgIdpProviders.Configure: only the fields set
// are sent.
type UpdateIdpInput struct {
	DisplayName     *string        `json:"displayName,omitempty"`
	ClientID        *string        `json:"clientId,omitempty"`
	ClientSecretRef *string        `json:"clientSecretRef,omitempty"`
	RedirectURI     *string        `json:"redirectUri,omitempty"`
	MetadataURL     *string        `json:"metadataUrl,omitempty"`
	AttributeMap    map[string]any `json:"attributeMap,omitempty"`
	Enabled         *bool          `json:"enabled,omitempty"`
}

// List lists configured identity providers: GET /identity-providers.
func (n *OrgIdpProviders) List(ctx context.Context) ([]map[string]any, *Meta, *KError) {
	env, kerr := doRequest[[]map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("identity-providers")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Create creates an IdP integration: POST /identity-providers.
func (n *OrgIdpProviders) Create(ctx context.Context, input CreateIdpInput) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "POST", Path: OrgPath("identity-providers"), Body: input})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Get reads one provider: GET /identity-providers/<id>.
func (n *OrgIdpProviders) Get(ctx context.Context, idpID string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("identity-providers/" + url.PathEscape(idpID))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Configure updates a provider's settings or enables/disables it:
// PATCH /identity-providers/<id>.
func (n *OrgIdpProviders) Configure(ctx context.Context, idpID string, patch UpdateIdpInput) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "PATCH", Path: OrgPath("identity-providers/" + url.PathEscape(idpID)), Body: patch})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Delete removes a provider: DELETE /identity-providers/<id>.
func (n *OrgIdpProviders) Delete(ctx context.Context, idpID string) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "DELETE", Path: OrgPath("identity-providers/" + url.PathEscape(idpID))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// OrgSMTP — client.Org.SMTP.*
type OrgSMTP struct{ config *httpClientConfig }

// SetSMTPInput is the org SMTP configuration, as the gateway takes it in the
// smtpConfig field of PATCH /org. The password is never sent: PasswordRef
// references a stored secret.
type SetSMTPInput struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	// Username is the SMTP login.
	Username string `json:"username"`
	// PasswordRef is a reference to the stored SMTP password (an opaque token).
	PasswordRef string `json:"passwordRef"`
	FromAddress string `json:"fromAddress"`
	FromName    string `json:"fromName,omitempty"`
}

// Set sets or updates the org SMTP configuration: PATCH /org with {smtpConfig}.
func (n *OrgSMTP) Set(ctx context.Context, input SetSMTPInput) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "PATCH", Path: OrgPath("org"), Body: map[string]any{"smtpConfig": input}})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Test sends a diagnostic email to verify the configuration:
// POST /org/smtp/test-email with {to, subject?}. subject may be empty.
func (n *OrgSMTP) Test(ctx context.Context, to, subject string) (map[string]any, *Meta, *KError) {
	body := map[string]any{"to": to}
	if subject != "" {
		body["subject"] = subject
	}
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "POST", Path: OrgPath("org/smtp/test-email"), Body: body})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Unset clears the org-level SMTP configuration: PATCH /org with {smtpConfig: null}.
func (n *OrgSMTP) Unset(ctx context.Context) (map[string]any, *Meta, *KError) {
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "PATCH", Path: OrgPath("org"), Body: map[string]any{"smtpConfig": nil}})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// OrgDefaultLLM — client.Org.DefaultLLMNs.*
type OrgDefaultLLM struct{ config *httpClientConfig }

// OrgDefaultLLMRecord is the org-default LLM record.
type OrgDefaultLLMRecord struct {
	OrgID          string  `json:"orgId"`
	DefaultModelID string  `json:"defaultModelId"`
	UpdatedAt      string  `json:"updatedAt"`
	UpdatedBy      *string `json:"updatedBy"`
}

// OrgDefaultLLMSetResult is the result of setting the org-default LLM.
type OrgDefaultLLMSetResult struct {
	OrgID          string `json:"orgId"`
	DefaultModelID string `json:"defaultModelId"`
}

// Get reads the org-default LLM: GET /default-llm. Any accepted member may read it.
func (n *OrgDefaultLLM) Get(ctx context.Context) (*OrgDefaultLLMRecord, *Meta, *KError) {
	env, kerr := doRequest[OrgDefaultLLMRecord](ctx, n.config, &requestInit{Method: "GET", Path: OrgPath("default-llm")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Set sets the org-default LLM: PATCH /default-llm with {modelId}. ORG_ADMIN or
// ORG_OWNER only; the model must be available to the org's tier.
func (n *OrgDefaultLLM) Set(ctx context.Context, modelID string) (*OrgDefaultLLMSetResult, *Meta, *KError) {
	env, kerr := doRequest[OrgDefaultLLMSetResult](ctx, n.config, &requestInit{Method: "PATCH", Path: OrgPath("default-llm"), Body: map[string]any{"modelId": modelID}})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}
