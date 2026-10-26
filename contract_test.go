package k9o

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

// contractReq is one request an SDK method put on the wire.
type contractReq struct {
	Method string
	Path   string // escaped path, as sent
	Query  map[string]string
	Body   []byte
	CType  string
}

// contractCase is one public method with representative arguments and the
// exact request the gateway route reads.
type contractCase struct {
	id     string
	call   func(ctx context.Context, c *Client)
	method string
	path   string
	query  map[string]string
	// body is the exact JSON body ("" = none). raw cases compare rawBody bytes.
	body    string
	rawBody string
	ctype   string // required Content-Type prefix when set
}

func str(s string) *string { return &s }
func boolp(b bool) *bool   { return &b }

func contractCases() []contractCase {
	t := 0.2
	return []contractCase{
		// projects (org plane)
		{id: "projects.list", call: func(ctx context.Context, c *Client) { c.Projects.List(ctx) }, method: "GET", path: "/projects"},
		{id: "projects.create", call: func(ctx context.Context, c *Client) {
			c.Projects.Create(ctx, CreateProjectInput{Slug: "demo", DisplayName: "Demo", CustodyMode: "PLATFORM_MANAGED"})
		}, method: "POST", path: "/projects", body: `{"slug":"demo","displayName":"Demo","custodyMode":"PLATFORM_MANAGED"}`},
		{id: "projects.get", call: func(ctx context.Context, c *Client) { c.Projects.Get(ctx, "demo") }, method: "GET", path: "/projects/demo"},
		{id: "projects.update", call: func(ctx context.Context, c *Client) {
			c.Projects.Update(ctx, "demo", UpdateProjectInput{DisplayName: str("Renamed")})
		}, method: "PATCH", path: "/projects/demo", body: `{"displayName":"Renamed"}`},

		// functions
		{id: "functions.uploadArtifact", call: func(ctx context.Context, c *Client) {
			c.Functions.UploadArtifact(ctx, []byte{0, 97, 115, 109}, "wasm")
		}, method: "POST", path: "/@portal/functions/artifacts", rawBody: "\x00asm", ctype: "application/wasm"},
		{id: "functions.deploy", call: func(ctx context.Context, c *Client) {
			c.Functions.Deploy(ctx, DeployFunctionInput{Name: "hello", Runtime: "bun", Tier: "sandbox", ArtifactKey: "functions/p/abc.tar", Version: "1"})
		}, method: "POST", path: "/@portal/functions", body: `{"name":"hello","runtime":"bun","tier":"sandbox","artifactKey":"functions/p/abc.tar","version":"1"}`},
		{id: "functions.list", call: func(ctx context.Context, c *Client) {
			c.Functions.List(ctx, ListFunctionsOptions{Limit: 10, Status: "active"})
		}, method: "GET", path: "/@portal/functions", query: map[string]string{"limit": "10", "status": "active"}},
		{id: "functions.get", call: func(ctx context.Context, c *Client) { c.Functions.Get(ctx, "hello") }, method: "GET", path: "/@portal/functions/hello"},
		{id: "functions.delete", call: func(ctx context.Context, c *Client) { c.Functions.Delete(ctx, "hello") }, method: "DELETE", path: "/@portal/functions/hello"},
		{id: "functions.invoke", call: func(ctx context.Context, c *Client) {
			c.Functions.Invoke(ctx, "hello", map[string]any{"a": 1})
		}, method: "POST", path: "/@portal/functions/hello/invoke", body: `{"payload":{"a":1}}`},
		{id: "functions.invoke.options", call: func(ctx context.Context, c *Client) {
			c.Functions.Invoke(ctx, "hello", nil, InvokeOptions{Background: true, OnBehalfOf: "u_1"})
		}, method: "POST", path: "/@portal/functions/hello/invoke", body: `{"background":true,"onBehalfOf":"u_1"}`},

		// policies
		{id: "policies.current", call: func(ctx context.Context, c *Client) { c.Policies.Current(ctx) }, method: "GET", path: "/@portal/policy"},
		{id: "policies.list", call: func(ctx context.Context, c *Client) { c.Policies.List(ctx, "published") }, method: "GET", path: "/@portal/policies", query: map[string]string{"status": "published"}},
		{id: "policies.create", call: func(ctx context.Context, c *Client) {
			c.Policies.Create(ctx, CreatePolicyInput{Policy: map[string]any{"collections": map[string]any{}}, EditorMode: "json"})
		}, method: "POST", path: "/@portal/policies", body: `{"policy":{"collections":{}},"editorMode":"json"}`},
		{id: "policies.validate", call: func(ctx context.Context, c *Client) {
			c.Policies.Validate(ctx, ValidatePolicyInput{Policy: map[string]any{"collections": map[string]any{}}})
		}, method: "POST", path: "/@portal/policies/validate", body: `{"policy":{"collections":{}}}`},
		{id: "policies.get", call: func(ctx context.Context, c *Client) { c.Policies.Get(ctx, "pol_1") }, method: "GET", path: "/@portal/policies/pol_1"},
		{id: "policies.update", call: func(ctx context.Context, c *Client) {
			c.Policies.Update(ctx, "pol_1", UpdatePolicyInput{Policy: map[string]any{"collections": map[string]any{}}})
		}, method: "PATCH", path: "/@portal/policies/pol_1", body: `{"policy":{"collections":{}}}`},
		{id: "policies.delete", call: func(ctx context.Context, c *Client) { c.Policies.Delete(ctx, "pol_1") }, method: "DELETE", path: "/@portal/policies/pol_1"},
		{id: "policies.publish", call: func(ctx context.Context, c *Client) { c.Policies.Publish(ctx, "pol_1") }, method: "POST", path: "/@portal/policies/pol_1/publish"},

		// audit
		{id: "audit.query", call: func(ctx context.Context, c *Client) { c.Audit.Query(ctx, AuditQueryOpts{Limit: 10}) }, method: "GET", path: "/@portal/audit-log", query: map[string]string{"limit": "10"}},
		{id: "audit.query.filters", call: func(ctx context.Context, c *Client) {
			c.Audit.Query(ctx, AuditQueryOpts{EventType: "auth.sign_in", ActorID: "u_1", After: "2026-01-01", Before: "2026-02-01", Cursor: "cur"})
		}, method: "GET", path: "/@portal/audit-log", query: map[string]string{"eventType": "auth.sign_in", "actorId": "u_1", "after": "2026-01-01", "before": "2026-02-01", "cursor": "cur"}},

		// orgs and the org plane
		{id: "orgs.list", call: func(ctx context.Context, c *Client) { c.Orgs.List(ctx) }, method: "GET", path: "/orgs"},
		{id: "orgs.get", call: func(ctx context.Context, c *Client) { c.Orgs.Get(ctx, "acme") }, method: "GET", path: "/orgs/acme"},
		{id: "orgs.restore", call: func(ctx context.Context, c *Client) { c.Orgs.Restore(ctx, "org_1") }, method: "POST", path: "/orgs/org_1/restore"},
		{id: "orgs.current", call: func(ctx context.Context, c *Client) { c.Orgs.Current(ctx) }, method: "GET", path: "/org"},
		{id: "org.get", call: func(ctx context.Context, c *Client) { c.Org.Get(ctx) }, method: "GET", path: "/org"},
		{id: "org.members.list", call: func(ctx context.Context, c *Client) { c.Org.Members.List(ctx, "active") }, method: "GET", path: "/members", query: map[string]string{"status": "active"}},
		{id: "org.members.invite", call: func(ctx context.Context, c *Client) { c.Org.Members.Invite(ctx, "a@b.co", "ORG_MEMBER") }, method: "POST", path: "/members", body: `{"email":"a@b.co","role":"ORG_MEMBER"}`},
		{id: "org.members.remove", call: func(ctx context.Context, c *Client) { c.Org.Members.Remove(ctx, "u_1") }, method: "DELETE", path: "/members/u_1"},
		{id: "org.groups.list", call: func(ctx context.Context, c *Client) { c.Org.Groups.List(ctx) }, method: "GET", path: "/groups"},
		{id: "org.groups.get", call: func(ctx context.Context, c *Client) { c.Org.Groups.Get(ctx, "grp_1") }, method: "GET", path: "/groups/grp_1"},
		{id: "org.auditLog.query", call: func(ctx context.Context, c *Client) {
			c.Org.AuditLog.Query(ctx, OrgAuditQueryOpts{Type: "data.write.org.role_granted", Limit: 10})
		}, method: "GET", path: "/audit-log", query: map[string]string{"type": "data.write.org.role_granted", "limit": "10"}},
		{id: "org.gate.roles.list", call: func(ctx context.Context, c *Client) { c.Org.Gate.Roles.List(ctx) }, method: "GET", path: "/role-grants"},
		{id: "org.gate.roles.grant", call: func(ctx context.Context, c *Client) {
			c.Org.Gate.Roles.Grant(ctx, GrantRoleInput{UserID: "u_1", Role: "ORG_ADMIN"})
		}, method: "POST", path: "/role-grants", body: `{"userId":"u_1","role":"ORG_ADMIN"}`},
		{id: "org.gate.roles.get", call: func(ctx context.Context, c *Client) { c.Org.Gate.Roles.Get(ctx, "g_1") }, method: "GET", path: "/role-grants/g_1"},
		{id: "org.gate.roles.revoke", call: func(ctx context.Context, c *Client) { c.Org.Gate.Roles.Revoke(ctx, "g_1", "") }, method: "DELETE", path: "/role-grants/g_1"},
		{id: "org.gate.roles.revoke.reason", call: func(ctx context.Context, c *Client) { c.Org.Gate.Roles.Revoke(ctx, "g_1", "left the company") }, method: "DELETE", path: "/role-grants/g_1", body: `{"decisionReason":"left the company"}`},
		{id: "org.gate.idpProviders.list", call: func(ctx context.Context, c *Client) { c.Org.Gate.IdpProviders.List(ctx) }, method: "GET", path: "/identity-providers"},
		{id: "org.gate.idpProviders.create", call: func(ctx context.Context, c *Client) {
			c.Org.Gate.IdpProviders.Create(ctx, CreateIdpInput{IdpProvider: "saml", DisplayName: "Okta", ClientID: "cid", ClientSecretRef: "ref_1", TierGating: "free_tier_ok"})
		}, method: "POST", path: "/identity-providers", body: `{"idpProvider":"saml","displayName":"Okta","clientId":"cid","clientSecretRef":"ref_1","tierGating":"free_tier_ok"}`},
		{id: "org.gate.idpProviders.get", call: func(ctx context.Context, c *Client) { c.Org.Gate.IdpProviders.Get(ctx, "idp_1") }, method: "GET", path: "/identity-providers/idp_1"},
		{id: "org.gate.idpProviders.configure", call: func(ctx context.Context, c *Client) {
			c.Org.Gate.IdpProviders.Configure(ctx, "idp_1", UpdateIdpInput{Enabled: boolp(true)})
		}, method: "PATCH", path: "/identity-providers/idp_1", body: `{"enabled":true}`},
		{id: "org.gate.idpProviders.delete", call: func(ctx context.Context, c *Client) { c.Org.Gate.IdpProviders.Delete(ctx, "idp_1") }, method: "DELETE", path: "/identity-providers/idp_1"},
		{id: "org.smtp.set", call: func(ctx context.Context, c *Client) {
			c.Org.SMTP.Set(ctx, SetSMTPInput{Host: "smtp.x.io", Port: 587, Username: "u", PasswordRef: "ref_1", FromAddress: "a@b.co", FromName: "A"})
		}, method: "PATCH", path: "/org", body: `{"smtpConfig":{"host":"smtp.x.io","port":587,"username":"u","passwordRef":"ref_1","fromAddress":"a@b.co","fromName":"A"}}`},
		{id: "org.smtp.test", call: func(ctx context.Context, c *Client) { c.Org.SMTP.Test(ctx, "a@b.co", "") }, method: "POST", path: "/org/smtp/test-email", body: `{"to":"a@b.co"}`},
		{id: "org.smtp.unset", call: func(ctx context.Context, c *Client) { c.Org.SMTP.Unset(ctx) }, method: "PATCH", path: "/org", body: `{"smtpConfig":null}`},
		{id: "org.defaultLLM.get", call: func(ctx context.Context, c *Client) { c.Org.DefaultLLMGet(ctx) }, method: "GET", path: "/default-llm"},
		{id: "org.defaultLLM.set", call: func(ctx context.Context, c *Client) { c.Org.DefaultLLMSet(ctx, "llama-3-8b") }, method: "PATCH", path: "/default-llm", body: `{"modelId":"llama-3-8b"}`},

		// me
		{id: "me.profile.get", call: func(ctx context.Context, c *Client) { c.Me.Profile.Get(ctx) }, method: "GET", path: "/me"},
		{id: "me.profile.update", call: func(ctx context.Context, c *Client) { c.Me.Profile.Update(ctx, "A") }, method: "PATCH", path: "/me", body: `{"displayName":"A"}`},
		{id: "me.sessions.list", call: func(ctx context.Context, c *Client) { c.Me.Sessions.List(ctx) }, method: "GET", path: "/me/sessions"},
		{id: "me.sessions.revoke", call: func(ctx context.Context, c *Client) { c.Me.Sessions.Revoke(ctx, "s_1") }, method: "POST", path: "/auth/sessions/s_1/revoke"},
		{id: "me.sessions.revokeAll", call: func(ctx context.Context, c *Client) { c.Me.Sessions.RevokeAll(ctx) }, method: "POST", path: "/me/sessions/revoke-all"},
		{id: "me.apiTokens.list", call: func(ctx context.Context, c *Client) { c.Me.APITokens.List(ctx) }, method: "GET", path: "/me/api-tokens"},
		{id: "me.apiTokens.create", call: func(ctx context.Context, c *Client) {
			c.Me.APITokens.Create(ctx, CreateAPITokenInput{Label: "ci", ScopeManifest: APITokenScopeManifest{Operations: []string{"read"}, Projects: []string{"portal"}}, TTLSeconds: 3600})
		}, method: "POST", path: "/me/api-tokens", body: `{"label":"ci","scopeManifest":{"operations":["read"],"projects":["portal"]},"ttlSeconds":3600}`},
		{id: "me.apiTokens.delete", call: func(ctx context.Context, c *Client) { c.Me.APITokens.Delete(ctx, "pat_1") }, method: "DELETE", path: "/me/api-tokens/pat_1"},

		// migrations
		{id: "migrations.list", call: func(ctx context.Context, c *Client) { c.Migrations.List(ctx) }, method: "GET", path: "/@portal/migrations"},
		{id: "migrations.get", call: func(ctx context.Context, c *Client) { c.Migrations.Get(ctx, "1") }, method: "GET", path: "/@portal/migrations/1"},
		{id: "migrations.apply", call: func(ctx context.Context, c *Client) {
			c.Migrations.Apply(ctx, []Migration{{Version: 1, Slug: "init", Up: []Operation{}, Down: []Operation{}}})
		}, method: "POST", path: "/@portal/migrations/apply", body: `{"migrations":[{"version":1,"slug":"init","up":[],"down":[]}]}`},
		{id: "migrations.queue", call: func(ctx context.Context, c *Client) {
			c.Migrations.Queue(ctx, []Migration{{Version: 1, Slug: "init", Up: []Operation{}, Down: []Operation{}}})
		}, method: "POST", path: "/@portal/migrations/queue", body: `{"migrations":[{"version":1,"slug":"init","up":[],"down":[]}]}`},
		{id: "migrations.listPending", call: func(ctx context.Context, c *Client) { c.Migrations.ListPending(ctx) }, method: "GET", path: "/@portal/migrations/pending"},
		{id: "migrations.applyPending", call: func(ctx context.Context, c *Client) { c.Migrations.ApplyPending(ctx, "pm_1") }, method: "POST", path: "/@portal/migrations/pending/pm_1/apply"},
		{id: "migrations.rejectPending", call: func(ctx context.Context, c *Client) { c.Migrations.RejectPending(ctx, "pm_1", "no") }, method: "POST", path: "/@portal/migrations/pending/pm_1/reject", body: `{"reason":"no"}`},
		{id: "migrations.deletePending", call: func(ctx context.Context, c *Client) { c.Migrations.DeletePending(ctx, "pm_1") }, method: "DELETE", path: "/@portal/migrations/pending/pm_1"},

		// storage
		{id: "storage.listBuckets", call: func(ctx context.Context, c *Client) { c.Storage.ListBuckets(ctx) }, method: "GET", path: "/@portal/storage/buckets"},
		{id: "storage.getBucket", call: func(ctx context.Context, c *Client) { c.Storage.GetBucket(ctx, "docs") }, method: "GET", path: "/@portal/storage/buckets/docs"},
		{id: "storage.createBucket", call: func(ctx context.Context, c *Client) {
			c.Storage.CreateBucket(ctx, "docs", &CreateBucketOptions{Public: boolp(false)})
		}, method: "POST", path: "/@portal/storage/buckets", body: `{"name":"docs","public":false}`},
		{id: "storage.updateBucket", call: func(ctx context.Context, c *Client) { c.Storage.UpdateBucket(ctx, "docs", BucketPatch{"public": true}) }, method: "PATCH", path: "/@portal/storage/buckets/docs", body: `{"public":true}`},
		{id: "storage.deleteBucket", call: func(ctx context.Context, c *Client) { c.Storage.DeleteBucket(ctx, "docs", true) }, method: "DELETE", path: "/@portal/storage/buckets/docs", query: map[string]string{"empty": "true"}},
		{id: "storage.object.upload", call: func(ctx context.Context, c *Client) { c.Storage.From("docs").Upload(ctx, "a.txt", "hello", nil) }, method: "POST", path: "/@portal/storage/buckets/docs/objects", ctype: "multipart/form-data"},
		{id: "storage.object.createSignedUrl", call: func(ctx context.Context, c *Client) {
			c.Storage.From("docs").CreateSignedURL(ctx, "a.txt", SignedURLOptions{ExpiresIn: 600})
		}, method: "POST", path: "/@portal/storage/buckets/docs/objects/sign", body: `{"path":"a.txt","expiresIn":600}`},
		{id: "storage.object.list", call: func(ctx context.Context, c *Client) { c.Storage.From("docs").List(ctx, "a") }, method: "GET", path: "/@portal/storage/buckets/docs/objects", query: map[string]string{"prefix": "a"}},
		{id: "storage.object.remove", call: func(ctx context.Context, c *Client) { c.Storage.From("docs").Remove(ctx, "a.txt") }, method: "DELETE", path: "/@portal/storage/buckets/docs/objects", query: map[string]string{"path": "a.txt"}},

		// ai and compute
		{id: "ai.embed", call: func(ctx context.Context, c *Client) { c.AI.Embed(ctx, EmbedInput{Input: []string{"a", "b"}}) }, method: "POST", path: "/@portal/ai/embeddings", body: `{"input":["a","b"]}`},
		{id: "ai.generate", call: func(ctx context.Context, c *Client) {
			c.AI.Generate(ctx, "hi", GenerateOptions{Model: "llama-3-8b", MaxTokens: 10, Temperature: &t})
		}, method: "POST", path: "/@portal/ai/generate", body: `{"prompt":"hi","model":"llama-3-8b","maxTokens":10,"temperature":0.2}`},
		{id: "compute.llms.list", call: func(ctx context.Context, c *Client) { c.Compute.LLMs.List(ctx) }, method: "GET", path: "/@portal/compute/llms"},
		{id: "compute.llms.read", call: func(ctx context.Context, c *Client) { c.Compute.LLMs.Read(ctx, "llama-3-8b") }, method: "GET", path: "/@portal/compute/llms/llama-3-8b"},
		{id: "compute.llms.invoke", call: func(ctx context.Context, c *Client) {
			c.Compute.LLMs.Invoke(ctx, "llama-3-8b", LLMInvokeInput{Prompt: "hi", Params: &LLMInvokeParams{MaxTokens: 10, Temperature: &t}})
		}, method: "POST", path: "/@portal/compute/llms/llama-3-8b/invoke", body: `{"prompt":"hi","maxTokens":10,"temperature":0.2}`},
		{id: "compute.llms.invokeStream", call: func(ctx context.Context, c *Client) {
			ev, errs := c.Compute.LLMs.InvokeStream(ctx, "llama-3-8b", LLMInvokeInput{Prompt: "hi"})
			for range ev {
			}
			for range errs {
			}
		}, method: "POST", path: "/@portal/compute/llms/llama-3-8b/invoke-stream", body: `{"prompt":"hi"}`},

		// studio
		{id: "studio.workflows.list", call: func(ctx context.Context, c *Client) {
			c.Studio.Workflows.List(ctx, WorkflowListFilters{Status: "active", Limit: 5, Cursor: "cur"})
		}, method: "GET", path: "/@portal/workflows", query: map[string]string{"status": "active", "limit": "5", "cursor": "cur"}},
		{id: "studio.workflows.create", call: func(ctx context.Context, c *Client) {
			c.Studio.Workflows.Create(ctx, CreateWorkflowInput{Name: "w", TriggerType: "manual", TriggerConfig: map[string]any{}, Actions: []WorkflowAction{{Index: 0, ActionType: "branch", Config: map[string]any{}}}})
		}, method: "POST", path: "/@portal/workflows", body: `{"name":"w","triggerType":"manual","triggerConfig":{},"actions":[{"index":0,"action_type":"branch","config":{}}]}`},
		{id: "studio.workflows.read", call: func(ctx context.Context, c *Client) { c.Studio.Workflows.Read(ctx, "wfl_1") }, method: "GET", path: "/@portal/workflows/wfl_1"},
		{id: "studio.workflows.update", call: func(ctx context.Context, c *Client) {
			c.Studio.Workflows.Update(ctx, "wfl_1", map[string]any{"name": "w2"})
		}, method: "PATCH", path: "/@portal/workflows/wfl_1", body: `{"name":"w2"}`},
		{id: "studio.workflows.delete", call: func(ctx context.Context, c *Client) { c.Studio.Workflows.Delete(ctx, "wfl_1") }, method: "DELETE", path: "/@portal/workflows/wfl_1"},
		{id: "studio.workflows.trigger", call: func(ctx context.Context, c *Client) {
			c.Studio.Workflows.Trigger(ctx, "wfl_1", map[string]any{"k": "v"})
		}, method: "POST", path: "/@portal/workflows/wfl_1/runs", body: `{"triggerContext":{"k":"v"}}`},
		{id: "studio.workflows.runs.list", call: func(ctx context.Context, c *Client) {
			c.Studio.Workflows.Runs.List(ctx, "wfl_1", WorkflowRunListFilters{Limit: 5})
		}, method: "GET", path: "/@portal/workflows/wfl_1/runs", query: map[string]string{"limit": "5"}},
		{id: "studio.workflows.runs.read", call: func(ctx context.Context, c *Client) { c.Studio.Workflows.Runs.Read(ctx, "wfl_1", "wfr_1") }, method: "GET", path: "/@portal/workflows/wfl_1/runs/wfr_1"},
		{id: "studio.workflows.runs.cancel", call: func(ctx context.Context, c *Client) { c.Studio.Workflows.Runs.Cancel(ctx, "wfl_1", "wfr_1") }, method: "POST", path: "/@portal/workflows/wfl_1/runs/wfr_1/cancel"},
		{id: "studio.dashboards.list", call: func(ctx context.Context, c *Client) { c.Studio.Dashboards.List(ctx, "active") }, method: "GET", path: "/@portal/dashboards", query: map[string]string{"status": "active"}},
		{id: "studio.dashboards.create", call: func(ctx context.Context, c *Client) { c.Studio.Dashboards.Create(ctx, CreateDashboardInput{Name: "d"}) }, method: "POST", path: "/@portal/dashboards", body: `{"name":"d"}`},
		{id: "studio.dashboards.read", call: func(ctx context.Context, c *Client) { c.Studio.Dashboards.Read(ctx, "dsh_1") }, method: "GET", path: "/@portal/dashboards/dsh_1"},
		{id: "studio.dashboards.update", call: func(ctx context.Context, c *Client) {
			c.Studio.Dashboards.Update(ctx, "dsh_1", map[string]any{"name": "d2"})
		}, method: "PATCH", path: "/@portal/dashboards/dsh_1", body: `{"name":"d2"}`},
		{id: "studio.dashboards.delete", call: func(ctx context.Context, c *Client) { c.Studio.Dashboards.Delete(ctx, "dsh_1") }, method: "DELETE", path: "/@portal/dashboards/dsh_1"},
		{id: "studio.dashboards.pin", call: func(ctx context.Context, c *Client) { c.Studio.Dashboards.Pin(ctx, "dsh_1") }, method: "POST", path: "/@portal/dashboards/dsh_1/pin"},
		{id: "studio.dashboards.unpin", call: func(ctx context.Context, c *Client) { c.Studio.Dashboards.Unpin(ctx, "dsh_1") }, method: "POST", path: "/@portal/dashboards/dsh_1/unpin"},

		// kibo vault (region is optional; sent only when given)
		{id: "kibo.vault.secrets.list", call: func(ctx context.Context, c *Client) { c.Kibo.Vault.Secrets.List(ctx, ListSecretsInput{Prefix: "db/"}) }, method: "GET", path: "/@portal/vault/secrets", query: map[string]string{"prefix": "db/"}},
		{id: "kibo.vault.secrets.get", call: func(ctx context.Context, c *Client) { c.Kibo.Vault.Secrets.Get(ctx, "db/pw", "") }, method: "GET", path: "/@portal/vault/secrets", query: map[string]string{"prefix": "db/pw", "limit": "100"}},
		{id: "kibo.vault.secrets.create", call: func(ctx context.Context, c *Client) {
			c.Kibo.Vault.Secrets.Create(ctx, CreateSecretInput{Name: "db/pw", Region: "de01", Value: "x"})
		}, method: "POST", path: "/@portal/vault/secrets/db%2Fpw", query: map[string]string{"region": "de01"}, body: `{"value":"eA=="}`},
		{id: "kibo.vault.secrets.create.noRegion", call: func(ctx context.Context, c *Client) {
			c.Kibo.Vault.Secrets.Create(ctx, CreateSecretInput{Name: "db/pw", Value: "x"})
		}, method: "POST", path: "/@portal/vault/secrets/db%2Fpw", body: `{"value":"eA=="}`},
		{id: "kibo.vault.secrets.revealChallenge", call: func(ctx context.Context, c *Client) { c.Kibo.Vault.Secrets.RevealChallenge(ctx, "db/pw", "") }, method: "POST", path: "/@portal/vault/secrets/db%2Fpw", query: map[string]string{"op": "reveal-challenge"}},
		{id: "kibo.vault.secrets.reveal", call: func(ctx context.Context, c *Client) { c.Kibo.Vault.Secrets.Reveal(ctx, "db/pw", "", "chal") }, method: "GET", path: "/@portal/vault/secrets/db%2Fpw"},
		{id: "kibo.vault.secrets.update", call: func(ctx context.Context, c *Client) {
			c.Kibo.Vault.Secrets.Update(ctx, "db/pw", UpdateSecretInput{Region: "de01", Value: "y"})
		}, method: "PUT", path: "/@portal/vault/secrets/db%2Fpw", query: map[string]string{"region": "de01"}, body: `{"value":"eQ=="}`},
		{id: "kibo.vault.secrets.rotate", call: func(ctx context.Context, c *Client) { c.Kibo.Vault.Secrets.Rotate(ctx, "db/pw", "") }, method: "POST", path: "/@portal/vault/secrets/db%2Fpw", query: map[string]string{"op": "rotate"}},
		{id: "kibo.vault.secrets.move", call: func(ctx context.Context, c *Client) { c.Kibo.Vault.Secrets.Move(ctx, "db/pw", "", "db/pw2") }, method: "POST", path: "/@portal/vault/secrets/db%2Fpw", query: map[string]string{"op": "move"}, body: `{"newPath":"db/pw2"}`},
		{id: "kibo.vault.secrets.delete", call: func(ctx context.Context, c *Client) { c.Kibo.Vault.Secrets.Delete(ctx, "db/pw", "") }, method: "DELETE", path: "/@portal/vault/secrets/db%2Fpw"},

		// gate (end-user auth)
		{id: "gate.signUp", call: func(ctx context.Context, c *Client) {
			c.Gate.SignUp(ctx, SignUpInput{Email: "a@b.co", Password: "pw", DisplayName: "A"})
		}, method: "POST", path: "/@portal/gate/signup", body: `{"email":"a@b.co","password":"pw","displayName":"A"}`},
		{id: "gate.signIn", call: func(ctx context.Context, c *Client) { c.Gate.SignIn(ctx, SignInInput{Email: "a@b.co", Password: "pw"}) }, method: "POST", path: "/@portal/gate/signin", body: `{"email":"a@b.co","password":"pw"}`},
		{id: "gate.signOut", call: func(ctx context.Context, c *Client) { c.Gate.SignOut(ctx, SignOutInput{RefreshToken: "rt"}) }, method: "POST", path: "/@portal/gate/signout", body: `{"refreshToken":"rt"}`},
		{id: "gate.whoami", call: func(ctx context.Context, c *Client) { c.Gate.Whoami(ctx) }, method: "GET", path: "/@portal/gate/whoami"},
		{id: "gate.refreshToken", call: func(ctx context.Context, c *Client) { c.Gate.RefreshToken(ctx, RefreshTokenInput{RefreshToken: "rt"}) }, method: "POST", path: "/@portal/gate/refresh", body: `{"refreshToken":"rt"}`},

		// realtime (SSE subscribe only)
		{id: "realtime.subscribe", call: func(ctx context.Context, c *Client) {
			ev, errs := c.Realtime.Subscribe(ctx, "collections.orders.*", SubscribeOptions{})
			for range ev {
			}
			for range errs {
			}
		}, method: "GET", path: "/@portal/realtime/subscribe", query: map[string]string{"topic": "collections.orders.*"}},

		// streams (user data streams)
		{id: "streams.create", call: func(ctx context.Context, c *Client) { c.Streams.Create(ctx, "Orders", 1073741824, 86400) }, method: "POST", path: "/@portal/data-streams/user", body: `{"displayName":"Orders","maxBytes":1073741824,"retentionSeconds":86400}`},
		{id: "streams.list", call: func(ctx context.Context, c *Client) { c.Streams.List(ctx) }, method: "GET", path: "/@portal/data-streams/user"},
		{id: "streams.get", call: func(ctx context.Context, c *Client) { c.Streams.Get(ctx, "orders") }, method: "GET", path: "/@portal/data-streams/user/orders"},
		{id: "streams.delete", call: func(ctx context.Context, c *Client) { c.Streams.Delete(ctx, "orders") }, method: "DELETE", path: "/@portal/data-streams/user/orders"},
		{id: "streams.publish", call: func(ctx context.Context, c *Client) {
			c.Streams.Publish(ctx, "orders", map[string]any{"id": 1}, "eu.new")
		}, method: "POST", path: "/@portal/data-streams/user/orders/publish", body: `{"data":{"id":1},"subject":"eu.new"}`},
		{id: "streams.replay", call: func(ctx context.Context, c *Client) {
			from, until := int64(10), int64(50)
			c.Streams.Replay(ctx, "orders", ReplayOptions{FromSequence: &from, UntilSequence: &until, Limit: 25})
		}, method: "POST", path: "/@portal/data-streams/user/orders/replay", body: `{"fromSequence":10,"untilSequence":50,"limit":25}`},
		{id: "streams.subscribe", call: func(ctx context.Context, c *Client) {
			ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
			defer cancel()
			ev, errs := c.Streams.Subscribe(ctx, "orders", "5")
			for range ev {
			}
			for range errs {
			}
		}, method: "GET", path: "/@portal/data-streams/user/orders/subscribe", query: map[string]string{"fromSequence": "5"}},

		// approvals, approval policies, schema collections
		{id: "approvals.list", call: func(ctx context.Context, c *Client) {
			c.Approvals.List(ctx, ListApprovalsOptions{Status: "pending_approval", Limit: 20})
		}, method: "GET", path: "/@portal/pending-approvals", query: map[string]string{"status": "pending_approval", "limit": "20"}},
		{id: "approvals.get", call: func(ctx context.Context, c *Client) { c.Approvals.Get(ctx, "apr_1") }, method: "GET", path: "/@portal/pending-approvals/apr_1"},
		{id: "approvals.approve", call: func(ctx context.Context, c *Client) {
			c.Approvals.Approve(ctx, "apr_1", ApproveInput{Reason: str("ok")})
		}, method: "POST", path: "/@portal/pending-approvals/apr_1/approve", body: `{"reason":"ok"}`},
		{id: "approvals.reject", call: func(ctx context.Context, c *Client) {
			c.Approvals.Reject(ctx, "apr_1", RejectInput{Reason: "duplicate"})
		}, method: "POST", path: "/@portal/pending-approvals/apr_1/reject", body: `{"reason":"duplicate"}`},
		{id: "approvals.bulk", call: func(ctx context.Context, c *Client) {
			c.Approvals.Bulk(ctx, BulkApprovalsInput{Action: "reject", Criteria: map[string]any{"eq": []any{"change.table", "orders"}}, Reason: "stale"})
		}, method: "POST", path: "/@portal/pending-approvals/bulk", body: `{"action":"reject","criteria":{"eq":["change.table","orders"]},"reason":"stale"}`},
		{id: "approvalPolicies.list", call: func(ctx context.Context, c *Client) { c.ApprovalPolicies.List(ctx) }, method: "GET", path: "/@portal/approval-policies"},
		{id: "approvalPolicies.create", call: func(ctx context.Context, c *Client) {
			c.ApprovalPolicies.Create(ctx, CreateApprovalPolicyInput{Collection: "orders", Operation: "update", Action: "queue", Predicate: map[string]any{"gt": []any{"change.row.amount", 1000}}})
		}, method: "POST", path: "/@portal/approval-policies", body: `{"collection":"orders","operation":"update","action":"queue","predicate":{"gt":["change.row.amount",1000]}}`},
		{id: "approvalPolicies.delete", call: func(ctx context.Context, c *Client) { c.ApprovalPolicies.Delete(ctx, "apo_1") }, method: "DELETE", path: "/@portal/approval-policies/apo_1"},
		{id: "schema.collections.list", call: func(ctx context.Context, c *Client) {
			c.Schema.Collections.List(ctx, ListCollectionsOptions{Limit: 10, Cursor: "abc"})
		}, method: "GET", path: "/@portal/schema/collections", query: map[string]string{"limit": "10", "cursor": "abc"}},

		// query and search
		{id: "query.positional", call: func(ctx context.Context, c *Client) {
			c.Query(ctx, "SELECT * FROM tasks WHERE done = $1", []any{false})
		}, method: "POST", path: "/@portal/query", body: `{"sql":"SELECT * FROM tasks WHERE done = $1","params":[false]}`},
		{id: "query.named", call: func(ctx context.Context, c *Client) {
			c.QueryNamed(ctx, "SELECT * FROM tasks WHERE done = :done", map[string]any{"done": true})
		}, method: "POST", path: "/@portal/query", body: `{"sql":"SELECT * FROM tasks WHERE done = $1","params":[true]}`},
		{id: "data.textSearch", call: func(ctx context.Context, c *Client) {
			c.Search(ctx, "posts", SearchInput{Column: "body", Q: "hello world", Limit: 5})
		}, method: "GET", path: "/@portal/data/posts", query: map[string]string{"textSearch": "body:hello world", "limit": "5"}},

		// data plane
		{id: "data.select", call: func(ctx context.Context, c *Client) {
			SelectExecute[map[string]any](ctx, c.From("posts").Select().Eq("status", "published").Limit(5))
		}, method: "GET", path: "/@portal/data/posts", query: map[string]string{"status": "eq.published", "limit": "5"}},
		{id: "data.insert", call: func(ctx context.Context, c *Client) { c.From("posts").Insert(ctx, map[string]any{"title": "x"}) }, method: "POST", path: "/@portal/data/posts", body: `{"title":"x"}`},
		{id: "data.update", call: func(ctx context.Context, c *Client) {
			c.From("posts").Update(map[string]any{"title": "y"}).Eq("id", "rec_1").Execute(ctx)
		}, method: "PATCH", path: "/@portal/data/posts/rec_1", body: `{"title":"y"}`},
		{id: "data.delete", call: func(ctx context.Context, c *Client) { c.From("posts").Delete().Eq("id", "rec_1").Execute(ctx) }, method: "DELETE", path: "/@portal/data/posts/rec_1"},
	}
}

// bodyContracts are the bodies of routes whose OpenAPI entry declares none: [required, properties]
// (null properties = open). Each is what the gateway handler reads
// (services/gateway/src/routes/migrations.ts parseApplyBody and the reject handler's optional
// `reason`; collection-records.ts for the record body, whose fields are the collection's columns).
var bodyContracts = map[string][]any{
	"migrations.apply":         {[]string{"migrations"}, []string{"migrations"}},
	"migrations.queue":         {[]string{"migrations"}, []string{"migrations"}},
	"migrations.rejectPending": {[]string{}, []string{"reason"}},
	"approvals.approve":        {[]string{}, []string{"reason"}},
	"approvals.reject":         {[]string{"reason"}, []string{"reason"}},
	"approvals.bulk":           {[]string{"action", "criteria"}, []string{"action", "criteria", "reason"}},
	"approvalPolicies.create":  {[]string{"collection", "operation", "action", "predicate"}, []string{"collection", "operation", "action", "predicate"}},
	"data.insert":              {[]string{}, nil},
	"data.update":              {[]string{}, nil},
}

// PENDING_ROUTES are routes the SDK already calls that are not in services/gateway/openapi.json yet.
// They are exempt from the OpenAPI route check (the dump marks them "pending") until the route lands;
// TestPendingRoutesAreStillPending fails once one appears in the OpenAPI, and the entry must then be removed.
var PENDING_ROUTES = map[string]string{}

func pendingRoute(method, path string) bool {
	if strings.HasPrefix(path, "/@portal/") {
		path = "/@{projectSlug}/" + strings.TrimPrefix(path, "/@portal/")
	}
	_, ok := PENDING_ROUTES[method+" "+path]
	return ok
}

func TestPendingRoutesAreStillPending(t *testing.T) {
	raw, err := os.ReadFile("../../services/gateway/openapi.json")
	if err != nil {
		t.Skip("gateway openapi.json is not next to this package")
	}
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	for key := range PENDING_ROUTES {
		method, path, _ := strings.Cut(key, " ")
		if _, ok := spec.Paths[path][strings.ToLower(method)]; ok {
			t.Fatalf("%s is in the OpenAPI now: remove it from PENDING_ROUTES", key)
		}
	}
}

func TestContractRequests(t *testing.T) {
	var dump *os.File
	if p := os.Getenv("K9O_CONTRACT_DUMP"); p != "" {
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		dump = f
	}
	for _, tc := range contractCases() {
		tc := tc
		t.Run(tc.id, func(t *testing.T) {
			var got []contractReq
			rt := roundTripFunc(func(req *http.Request) *http.Response {
				var body []byte
				if req.Body != nil {
					body, _ = io.ReadAll(req.Body)
				}
				q := map[string]string{}
				for k, v := range req.URL.Query() {
					q[k] = v[0]
				}
				got = append(got, contractReq{Method: req.Method, Path: req.URL.EscapedPath(), Query: q, Body: body, CType: req.Header.Get("Content-Type")})
				if strings.HasSuffix(req.URL.Path, "/invoke-stream") || strings.HasSuffix(req.URL.Path, "/realtime/subscribe") || strings.HasSuffix(req.URL.Path, "/subscribe") {
					return sseResponse("data: [DONE]\n\n")
				}
				return jsonResponse(200, map[string]any{"data": nil, "meta": map[string]any{}})
			})
			c := p6Client(t, rt)
			tc.call(context.Background(), c)
			if len(got) != 1 {
				t.Fatalf("want exactly one request, got %d: %+v", len(got), got)
			}
			r := got[0]
			if r.Method != tc.method || r.Path != tc.path {
				t.Fatalf("request = %s %s, want %s %s", r.Method, r.Path, tc.method, tc.path)
			}
			if len(r.Query) != len(tc.query) {
				t.Fatalf("query = %v, want %v", r.Query, tc.query)
			}
			for k, v := range tc.query {
				if r.Query[k] != v {
					t.Fatalf("query %q = %q, want %q (all: %v)", k, r.Query[k], v, r.Query)
				}
			}
			if tc.ctype != "" && !strings.HasPrefix(r.CType, tc.ctype) {
				t.Fatalf("Content-Type = %q, want prefix %q", r.CType, tc.ctype)
			}
			switch {
			case tc.rawBody != "":
				if string(r.Body) != tc.rawBody {
					t.Fatalf("raw body = %q, want %q", r.Body, tc.rawBody)
				}
			case tc.body != "":
				var want, have any
				if err := json.Unmarshal([]byte(tc.body), &want); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(r.Body, &have); err != nil {
					t.Fatalf("body is not JSON: %q", r.Body)
				}
				if !bytes.Equal(mustJSON(want), mustJSON(have)) {
					t.Fatalf("body = %s, want %s", r.Body, tc.body)
				}
			case tc.ctype == "" && len(r.Body) != 0:
				t.Fatalf("unexpected body %q", r.Body)
			}
			if dump != nil {
				keys := make([]string, 0, len(r.Query))
				for k := range r.Query {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				entry := map[string]any{"id": tc.id, "method": r.Method, "path": r.Path, "query": keys, "raw": tc.rawBody != "" || strings.HasPrefix(tc.ctype, "multipart/")}
				if tc.body != "" {
					var b any
					_ = json.Unmarshal(r.Body, &b)
					entry["body"] = b
				}
				if pendingRoute(r.Method, r.Path) {
					entry["pending"] = true
				}
				if c, ok := bodyContracts[tc.id]; ok {
					entry["contract"] = c
				}
				if strings.HasPrefix(tc.id, "kibo.vault.secrets.") {
					entry["extraQuery"] = []string{"region"}
				}
				_ = json.NewEncoder(dump).Encode(entry)
			}
		})
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
