package k9o

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// captureHandler captures the incoming request into the supplied map
// and returns the supplied response body + status.
func captureHandler(captured map[string]any, status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		captured["method"] = r.Method
		captured["path"] = r.URL.Path
		captured["rawQuery"] = r.URL.RawQuery
		captured["headers"] = r.Header.Clone()
		if r.Body != nil {
			bodyBytes, _ := io.ReadAll(r.Body)
			captured["body"] = string(bodyBytes)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

func TestNewClientConstruct(t *testing.T) {
	c, kerr := NewClient(ClientOpts{BaseURL: "https://api.k9o.io"})
	if kerr != nil {
		t.Fatalf("NewClient: %v", kerr)
	}
	if c.config.BaseURL != "https://api.k9o.io" {
		t.Errorf("BaseURL: got %q", c.config.BaseURL)
	}
	if c.Projects == nil || c.Gate == nil {
		t.Errorf("Namespaces should be initialised")
	}
}

func TestNewClientRejectsInvalidBaseURL(t *testing.T) {
	_, kerr := NewClient(ClientOpts{BaseURL: "https://api.k9o.io/v2"})
	if kerr == nil {
		t.Fatal("expected error for path-bearing baseURL")
	}
	if kerr.Code != "K9O_INVALID_BASE_URL" {
		t.Errorf("Code: got %q", kerr.Code)
	}
}

func TestProjectsList(t *testing.T) {
	captured := map[string]any{}
	server := httptest.NewServer(captureHandler(captured, 200, `{"data":[],"meta":{"cursor":{"next":null}}}`))
	defer server.Close()

	c, _ := NewClient(ClientOpts{BaseURL: server.URL, OrgSlug: "acme"})
	rows, _, kerr := c.Projects.List(context.Background())
	if kerr != nil {
		t.Fatalf("List: %v", kerr)
	}
	if len(rows) != 0 {
		t.Errorf("expected empty list, got %v", rows)
	}
	if captured["method"] != "GET" {
		t.Errorf("method: got %v", captured["method"])
	}
	if captured["path"] != "/projects" {
		t.Errorf("path: got %v", captured["path"])
	}
}

func TestProjectsCreateValidatesSlug(t *testing.T) {
	c, _ := NewClient(ClientOpts{BaseURL: "https://api.k9o.io", OrgSlug: "acme"})
	_, _, kerr := c.Projects.Create(context.Background(), CreateProjectInput{Slug: "Bad-Slug", Name: "x"})
	if kerr == nil {
		t.Fatal("expected validation error")
	}
	if kerr.Code != "K9O_INVALID_FIELD_VALUE" {
		t.Errorf("Code: got %q", kerr.Code)
	}
}

func TestProjectsGet(t *testing.T) {
	captured := map[string]any{}
	respBody := `{"data":{"id":"proj_x","slug":"member-portal","name":"MP","custodyMode":"managed","createdAt":"2026-01-01T00:00:00Z","orgId":"org_y"},"meta":{}}`
	server := httptest.NewServer(captureHandler(captured, 200, respBody))
	defer server.Close()

	c, _ := NewClient(ClientOpts{BaseURL: server.URL})
	p, _, kerr := c.Projects.Get(context.Background(), "member-portal")
	if kerr != nil {
		t.Fatalf("Get: %v", kerr)
	}
	if captured["path"] != "/@member-portal/project" {
		t.Errorf("path: got %v", captured["path"])
	}
	if p.Slug != "member-portal" {
		t.Errorf("Slug: got %q", p.Slug)
	}
	if p.CustodyMode != "managed" {
		t.Errorf("CustodyMode: got %q", p.CustodyMode)
	}
}

func TestDataPlaneSelect(t *testing.T) {
	captured := map[string]any{}
	server := httptest.NewServer(captureHandler(captured, 200, `{"data":[{"id":"r1","title":"hi"}],"meta":{"cursor":{"next":"abc"}}}`))
	defer server.Close()

	c, _ := NewClient(ClientOpts{BaseURL: server.URL, ProjectSlug: "member-portal"})

	type Post struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	rows, meta, kerr := SelectExecute[Post](context.Background(),
		c.From("posts").Select().Eq("status", "published").Gt("views", 100).Limit(20),
	)
	if kerr != nil {
		t.Fatalf("Select: %v", kerr)
	}
	if len(rows) != 1 || rows[0].Title != "hi" {
		t.Errorf("rows: got %v", rows)
	}
	if meta == nil || meta.Cursor == nil || meta.Cursor.Next == nil || *meta.Cursor.Next != "abc" {
		t.Errorf("meta.Cursor.Next: got %v", meta.Cursor)
	}

	if captured["path"] != "/@member-portal/data/posts" {
		t.Errorf("path: got %v", captured["path"])
	}
	rq, _ := captured["rawQuery"].(string)
	if !strings.Contains(rq, "status=eq.published") {
		t.Errorf("rawQuery missing status: %q", rq)
	}
	if !strings.Contains(rq, "views=gt.100") {
		t.Errorf("rawQuery missing views: %q", rq)
	}
	if !strings.Contains(rq, "limit=20") {
		t.Errorf("rawQuery missing limit: %q", rq)
	}
}

func TestDataPlaneSelectRequiresProjectSlug(t *testing.T) {
	c, _ := NewClient(ClientOpts{BaseURL: "https://api.k9o.io"})
	_, _, kerr := SelectExecute[any](context.Background(), c.From("posts").Select())
	if kerr == nil {
		t.Fatal("expected error")
	}
	if kerr.Code != "K9O_MISSING_PROJECT_CONTEXT" {
		t.Errorf("Code: got %q", kerr.Code)
	}
}

func TestDataPlaneInsert(t *testing.T) {
	captured := map[string]any{}
	server := httptest.NewServer(captureHandler(captured, 200, `{"data":{"id":"rec_x","title":"hi"},"meta":{}}`))
	defer server.Close()

	c, _ := NewClient(ClientOpts{BaseURL: server.URL, ProjectSlug: "member-portal"})
	env, kerr := c.From("posts").Insert(context.Background(), map[string]any{"title": "hi"})
	if kerr != nil {
		t.Fatalf("Insert: %v", kerr)
	}
	if captured["method"] != "POST" {
		t.Errorf("method: got %v", captured["method"])
	}
	if captured["path"] != "/@member-portal/data/posts" {
		t.Errorf("path: got %v", captured["path"])
	}
	body, _ := captured["body"].(string)
	var parsed map[string]any
	_ = json.Unmarshal([]byte(body), &parsed)
	if parsed["title"] != "hi" {
		t.Errorf("body title: got %v", parsed)
	}
	if env.Data == nil {
		t.Errorf("env.Data should not be nil")
	}
}

func TestDataPlaneUpdateRequiresIDFilter(t *testing.T) {
	c, _ := NewClient(ClientOpts{BaseURL: "https://api.k9o.io", ProjectSlug: "member-portal"})
	_, kerr := c.From("posts").Update(map[string]any{"title": "x"}).Execute(context.Background())
	if kerr == nil {
		t.Fatal("expected error")
	}
	if kerr.Code != "K9O_INVALID_FIELD_VALUE" {
		t.Errorf("Code: got %q", kerr.Code)
	}
}

func TestErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(429)
		_, _ = io.WriteString(w, `{"code":"K9O_TIER_CAP_HIT","message":"cap exceeded","context":{"dimension":"mau"},"recommendation":"upgrade","requestId":"req_x"}`)
	}))
	defer server.Close()

	c, _ := NewClient(ClientOpts{BaseURL: server.URL, OrgSlug: "acme"})
	_, _, kerr := c.Projects.List(context.Background())
	if kerr == nil {
		t.Fatal("expected error")
	}
	if kerr.Code != "K9O_TIER_CAP_HIT" {
		t.Errorf("Code: got %q", kerr.Code)
	}
	if kerr.Status != 429 {
		t.Errorf("Status: got %d", kerr.Status)
	}
	if kerr.Recommendation != "upgrade" {
		t.Errorf("Recommendation: got %q", kerr.Recommendation)
	}
	if kerr.RequestID != "req_x" {
		t.Errorf("RequestID: got %q", kerr.RequestID)
	}
}

func TestGateSignInRoutesCorrectly(t *testing.T) {
	captured := map[string]any{}
	server := httptest.NewServer(captureHandler(captured, 200, `{"data":{"user":{"id":"eu_x"},"session":{}},"meta":{}}`))
	defer server.Close()

	c, _ := NewClient(ClientOpts{BaseURL: server.URL, ProjectSlug: "member-portal"})
	_, kerr := c.Gate.SignIn(context.Background(), SignInInput{Email: "x@y.com", Password: "z"})
	if kerr != nil {
		t.Fatalf("SignIn: %v", kerr)
	}
	if captured["path"] != "/@member-portal/gate/signin" {
		t.Errorf("path: got %v", captured["path"])
	}
	body, _ := captured["body"].(string)
	var parsed map[string]string
	_ = json.Unmarshal([]byte(body), &parsed)
	if parsed["email"] != "x@y.com" {
		t.Errorf("body email: got %v", parsed)
	}
}

func TestGateRequiresProjectSlug(t *testing.T) {
	c, _ := NewClient(ClientOpts{BaseURL: "https://api.k9o.io"})
	_, kerr := c.Gate.Whoami(context.Background())
	if kerr == nil {
		t.Fatal("expected error")
	}
	if kerr.Code != "K9O_MISSING_PROJECT_CONTEXT" {
		t.Errorf("Code: got %q", kerr.Code)
	}
}

func TestGateHosted(t *testing.T) {
	captured := map[string]any{}
	server := httptest.NewServer(captureHandler(captured, 200, `{"data":{"signInUrl":"https://login.example.com","rpId":"example.com"},"meta":{}}`))
	defer server.Close()

	c, _ := NewClient(ClientOpts{BaseURL: server.URL, ProjectSlug: "member-portal", APIKey: "k9o_secret_token"})
	info, _, kerr := c.Gate.Hosted(context.Background())
	if kerr != nil {
		t.Fatalf("Hosted: %v", kerr)
	}
	if captured["method"] != "GET" || captured["path"] != "/@member-portal/gate/hosted" {
		t.Errorf("verb/path: got %v %v", captured["method"], captured["path"])
	}
	h := captured["headers"].(http.Header)
	if h.Get("Authorization") != "" || h.Get("Cookie") != "" {
		t.Errorf("credential sent: %v", h)
	}
	if info.SignInUrl != "https://login.example.com" || info.RpId != "example.com" {
		t.Errorf("info: %+v", info)
	}
}

func TestGateHostedUniform404(t *testing.T) {
	server := httptest.NewServer(captureHandler(map[string]any{}, 404, `{"code":"K9O_PROJECT_NOT_FOUND","message":"Project not found"}`))
	defer server.Close()
	c, _ := NewClient(ClientOpts{BaseURL: server.URL, ProjectSlug: "nope"})
	_, _, kerr := c.Gate.Hosted(context.Background())
	if kerr == nil || kerr.Status != 404 || kerr.Code != "K9O_PROJECT_NOT_FOUND" {
		t.Fatalf("got %+v", kerr)
	}
}

func TestGateHostedRequiresProjectSlugNoRequest(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer server.Close()
	c, _ := NewClient(ClientOpts{BaseURL: server.URL})
	_, _, kerr := c.Gate.Hosted(context.Background())
	if kerr == nil || kerr.Code != "K9O_MISSING_PROJECT_CONTEXT" {
		t.Fatalf("got %+v", kerr)
	}
	if called {
		t.Error("request was sent")
	}
}
