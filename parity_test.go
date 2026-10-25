package k9o

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func parityClient(t *testing.T, rt roundTripFunc, projectSlug string) *Client {
	t.Helper()
	opts := ClientOpts{BaseURL: "https://acme.api.k9o.io", HTTPClient: &http.Client{Transport: noProbe(rt)}}
	if projectSlug != "" {
		opts.ProjectSlug = projectSlug
	}
	c, kerr := NewClient(opts)
	if kerr != nil {
		t.Fatalf("NewClient: %v", kerr)
	}
	return c
}

func TestMigrationsRoutes(t *testing.T) {
	var gotURL, gotMethod string
	rt := roundTripFunc(func(req *http.Request) *http.Response {
		gotURL = req.URL.String()
		gotMethod = req.Method
		if req.Method == "GET" && strings.HasSuffix(req.URL.Path, "/migrations") {
			return jsonResponse(200, map[string]any{"data": []any{}, "meta": map[string]any{}})
		}
		return jsonResponse(200, map[string]any{"data": map[string]any{}, "meta": map[string]any{}})
	})
	c := parityClient(t, rt, "portal")
	ctx := context.Background()
	dropPosts := "posts"
	if _, _, kerr := c.Migrations.Apply(ctx, []Migration{{
		Version: 1,
		Slug:    "init",
		Up:      []Operation{{CreateTable: &CreateTableOp{Name: "posts", Columns: []ColumnSpec{{Name: "id", Type: "uuid", PrimaryKey: true}}}}},
		Down:    []Operation{{DropTable: &dropPosts}},
	}}); kerr != nil {
		t.Fatalf("Apply: %v", kerr)
	}
	if gotMethod != "POST" || gotURL != "https://acme.api.k9o.io/@portal/migrations/apply" {
		t.Fatalf("Apply %s %s", gotMethod, gotURL)
	}
	if _, _, kerr := c.Migrations.ApplyPending(ctx, "pend_x"); kerr != nil {
		t.Fatalf("ApplyPending: %v", kerr)
	}
	if gotURL != "https://acme.api.k9o.io/@portal/migrations/pending/pend_x/apply" {
		t.Fatalf("ApplyPending URL = %s", gotURL)
	}
}

func TestFunctionsRoutes(t *testing.T) {
	var gotURL string
	rt := roundTripFunc(func(req *http.Request) *http.Response {
		gotURL = req.URL.String()
		return jsonResponse(200, map[string]any{"data": map[string]any{"id": "fn_x", "name": "greet"}, "meta": map[string]any{}})
	})
	c := parityClient(t, rt, "portal")
	ctx := context.Background()
	if _, _, kerr := c.Functions.Deploy(ctx, DeployFunctionInput{Name: "greet", Runtime: "bun", Code: "..."}); kerr != nil {
		t.Fatalf("Deploy: %v", kerr)
	}
	if gotURL != "https://acme.api.k9o.io/@portal/functions" {
		t.Fatalf("Deploy URL = %s", gotURL)
	}
	if _, _, kerr := c.Functions.Invoke(ctx, "greet", map[string]any{"name": "World"}); kerr != nil {
		t.Fatalf("Invoke: %v", kerr)
	}
	if gotURL != "https://acme.api.k9o.io/@portal/functions/greet/invoke" {
		t.Fatalf("Invoke URL = %s", gotURL)
	}
}

func TestPoliciesRoutes(t *testing.T) {
	var gotURL, gotMethod string
	rt := roundTripFunc(func(req *http.Request) *http.Response {
		gotURL = req.URL.String()
		gotMethod = req.Method
		return jsonResponse(200, map[string]any{"data": map[string]any{"id": "pol_x"}, "meta": map[string]any{}})
	})
	c := parityClient(t, rt, "portal")
	ctx := context.Background()
	if _, _, kerr := c.Policies.Create(ctx, "rls", map[string]any{"a": 1}); kerr != nil {
		t.Fatalf("Create: %v", kerr)
	}
	if gotMethod != "POST" || gotURL != "https://acme.api.k9o.io/@portal/policies" {
		t.Fatalf("Create %s %s", gotMethod, gotURL)
	}
	if _, _, kerr := c.Policies.Publish(ctx, "pol_x"); kerr != nil {
		t.Fatalf("Publish: %v", kerr)
	}
	if gotURL != "https://acme.api.k9o.io/@portal/policies/pol_x/publish" {
		t.Fatalf("Publish URL = %s", gotURL)
	}
}

func TestAuditRoutes(t *testing.T) {
	var gotURL string
	rt := roundTripFunc(func(req *http.Request) *http.Response {
		gotURL = req.URL.String()
		return jsonResponse(200, map[string]any{"data": []any{}, "meta": map[string]any{}})
	})
	c := parityClient(t, rt, "portal")
	if _, _, kerr := c.Audit.Query(context.Background(), AuditQueryOpts{Type: "data.write.access.insert", Limit: 50}); kerr != nil {
		t.Fatalf("Query: %v", kerr)
	}
	if !strings.Contains(gotURL, "/@portal/audit-log") || !strings.Contains(gotURL, "type=data.write.access.insert") {
		t.Fatalf("Query URL = %s", gotURL)
	}
}

func TestOrgsRoutes(t *testing.T) {
	var gotURL, gotMethod string
	rt := roundTripFunc(func(req *http.Request) *http.Response {
		gotURL = req.URL.String()
		gotMethod = req.Method
		if strings.HasSuffix(req.URL.Path, "/orgs") {
			return jsonResponse(200, map[string]any{"data": []any{}, "meta": map[string]any{}})
		}
		return jsonResponse(200, map[string]any{"data": map[string]any{"id": "org_x"}, "meta": map[string]any{}})
	})
	c := parityClient(t, rt, "")
	ctx := context.Background()
	if _, _, kerr := c.Orgs.List(ctx); kerr != nil {
		t.Fatalf("List: %v", kerr)
	}
	if gotURL != "https://acme.api.k9o.io/orgs" {
		t.Fatalf("List URL = %s", gotURL)
	}
	if _, _, kerr := c.Orgs.Restore(ctx, "org_x"); kerr != nil {
		t.Fatalf("Restore: %v", kerr)
	}
	if gotMethod != "POST" || gotURL != "https://acme.api.k9o.io/orgs/org_x/restore" {
		t.Fatalf("Restore %s %s", gotMethod, gotURL)
	}
}

func TestMeRoutes(t *testing.T) {
	var gotURL, gotMethod string
	rt := roundTripFunc(func(req *http.Request) *http.Response {
		gotURL = req.URL.String()
		gotMethod = req.Method
		return jsonResponse(200, map[string]any{"data": map[string]any{}, "meta": map[string]any{}})
	})
	c := parityClient(t, rt, "")
	ctx := context.Background()
	if _, _, kerr := c.Me.Profile.Get(ctx); kerr != nil {
		t.Fatalf("Profile.Get: %v", kerr)
	}
	if gotURL != "https://acme.api.k9o.io/me" {
		t.Fatalf("Profile.Get URL = %s", gotURL)
	}
	if _, _, kerr := c.Me.Sessions.RevokeAll(ctx); kerr != nil {
		t.Fatalf("Sessions.RevokeAll: %v", kerr)
	}
	if gotMethod != "DELETE" || gotURL != "https://acme.api.k9o.io/me/sessions" {
		t.Fatalf("RevokeAll %s %s", gotMethod, gotURL)
	}
	if _, _, kerr := c.Me.APITokens.Create(ctx, CreateAPITokenInput{Name: "ci"}); kerr != nil {
		t.Fatalf("APITokens.Create: %v", kerr)
	}
	if gotURL != "https://acme.api.k9o.io/me/api-tokens" {
		t.Fatalf("APITokens.Create URL = %s", gotURL)
	}
}

func TestOrgFullRoutes(t *testing.T) {
	var gotURL, gotMethod, gotBody string
	rt := roundTripFunc(func(req *http.Request) *http.Response {
		gotURL = req.URL.String()
		gotMethod = req.Method
		if req.Body != nil {
			b := make([]byte, req.ContentLength)
			_, _ = req.Body.Read(b)
			gotBody = string(b)
		}
		if strings.HasSuffix(req.URL.Path, "/members") && req.Method == "GET" {
			return jsonResponse(200, map[string]any{"data": []any{}, "meta": map[string]any{}})
		}
		if strings.HasSuffix(req.URL.Path, "/default-llm") {
			return jsonResponse(200, map[string]any{"data": "mistral-7b", "meta": map[string]any{}})
		}
		return jsonResponse(200, map[string]any{"data": map[string]any{}, "meta": map[string]any{}})
	})
	c := parityClient(t, rt, "")
	ctx := context.Background()
	if _, _, kerr := c.Org.Members.Invite(ctx, "x@y.com", "ORG_MEMBER"); kerr != nil {
		t.Fatalf("Members.Invite: %v", kerr)
	}
	if gotMethod != "POST" || gotURL != "https://acme.api.k9o.io/members" {
		t.Fatalf("Members.Invite %s %s", gotMethod, gotURL)
	}
	if _, _, kerr := c.Org.Gate.Roles.Grant(ctx, "u_a", "ORG_ADMIN"); kerr != nil {
		t.Fatalf("Gate.Roles.Grant: %v", kerr)
	}
	if gotURL != "https://acme.api.k9o.io/org-role-grants" {
		t.Fatalf("Roles.Grant URL = %s", gotURL)
	}
	if _, _, kerr := c.Org.SMTP.Set(ctx, SetSMTPInput{Host: "smtp.x.com", Port: 587, Username: "u", Password: "p", FromEmail: "n@x.com", FromName: "X", Encryption: "starttls"}); kerr != nil {
		t.Fatalf("SMTP.Set: %v", kerr)
	}
	if gotMethod != "PATCH" || gotURL != "https://acme.api.k9o.io/org" || !strings.Contains(gotBody, "smtpConfig") {
		t.Fatalf("SMTP.Set %s %s body=%s", gotMethod, gotURL, gotBody)
	}
	if _, _, kerr := c.Org.DefaultLLMGet(ctx); kerr != nil {
		t.Fatalf("DefaultLLMGet: %v", kerr)
	}
	if gotURL != "https://acme.api.k9o.io/org/default-llm" {
		t.Fatalf("DefaultLLMGet URL = %s", gotURL)
	}
}
