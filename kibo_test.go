package k9o

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// captureVaultHandler is like captureHandler in client_test.go but also
// records the raw (still-percent-encoded) request-target, since
// r.URL.Path/r.URL.RawQuery decode escaped segments (e.g. "%2F" -> "/")
// and would hide whether the client actually escaped a hierarchical secret
// name onto a single :name URL segment on the wire.
func captureVaultHandler(captured map[string]any, status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		captured["method"] = r.Method
		captured["requestURI"] = r.RequestURI
		captured["headers"] = r.Header.Clone()
		if r.Body != nil {
			bodyBytes, _ := io.ReadAll(r.Body)
			captured["body"] = string(bodyBytes)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != "" {
			_, _ = io.WriteString(w, body)
		}
	}
}

const summaryJSON = `{"id":"sec_abc","name":"db/pw","ownerId":"usr_1","visibility":{"kind":"private"},"version":1,"kid":"vkey_1","createdAt":"t","createdBy":"usr_1","updatedAt":"t","updatedBy":"usr_1","rotatedAt":null,"retiredAt":null,"metadata":{}}`

func TestSecretsListNoRegion(t *testing.T) {
	captured := map[string]any{}
	server := httptest.NewServer(captureVaultHandler(captured, 200, `{"data":{"secrets":[`+summaryJSON+`],"nextCursor":null},"meta":{}}`))
	defer server.Close()

	c, _ := NewClient(ClientOpts{BaseURL: server.URL, ProjectSlug: "member-portal"})
	result, _, kerr := c.Kibo.Vault.Secrets.List(context.Background(), ListSecretsInput{Prefix: "db/", Limit: 10})
	if kerr != nil {
		t.Fatalf("List: %v", kerr)
	}
	if captured["method"] != "GET" {
		t.Errorf("method: got %v", captured["method"])
	}
	uri, _ := captured["requestURI"].(string)
	if !strings.HasPrefix(uri, "/@member-portal/vault/secrets?") {
		t.Errorf("requestURI: got %q", uri)
	}
	if !strings.Contains(uri, "prefix=db%2F") {
		t.Errorf("requestURI missing escaped prefix: %q", uri)
	}
	if !strings.Contains(uri, "limit=10") {
		t.Errorf("requestURI missing limit: %q", uri)
	}
	if strings.Contains(uri, "region=") {
		t.Errorf("List must NOT send region (live-verified exception): %q", uri)
	}
	if len(result.Secrets) != 1 || result.Secrets[0].ID != "sec_abc" {
		t.Errorf("Secrets: got %+v", result.Secrets)
	}
	if result.NextCursor != nil {
		t.Errorf("NextCursor: expected nil, got %v", result.NextCursor)
	}
}

func TestSecretsGetRequiresRegion(t *testing.T) {
	captured := map[string]any{}
	server := httptest.NewServer(captureVaultHandler(captured, 200, `{"data":`+summaryJSON+`,"meta":{}}`))
	defer server.Close()

	c, _ := NewClient(ClientOpts{BaseURL: server.URL, ProjectSlug: "member-portal"})
	summary, _, kerr := c.Kibo.Vault.Secrets.Get(context.Background(), "db/pw", "us01")
	if kerr != nil {
		t.Fatalf("Get: %v", kerr)
	}
	if captured["requestURI"] != "/@member-portal/vault/secrets/db%2Fpw?region=us01" {
		t.Errorf("requestURI: got %v", captured["requestURI"])
	}
	if summary.Name != "db/pw" {
		t.Errorf("Name: got %q", summary.Name)
	}
}

func TestSecretsCreateEncodesValueBase64(t *testing.T) {
	captured := map[string]any{}
	server := httptest.NewServer(captureVaultHandler(captured, 200, `{"data":`+summaryJSON+`,"meta":{}}`))
	defer server.Close()

	c, _ := NewClient(ClientOpts{BaseURL: server.URL, ProjectSlug: "member-portal"})
	_, _, kerr := c.Kibo.Vault.Secrets.Create(context.Background(), CreateSecretInput{
		Name:       "db/pw",
		Region:     "us01",
		Value:      "hunter2",
		Visibility: &SecretVisibility{Kind: "private"},
	})
	if kerr != nil {
		t.Fatalf("Create: %v", kerr)
	}
	if captured["method"] != "POST" {
		t.Errorf("method: got %v", captured["method"])
	}
	if captured["requestURI"] != "/@member-portal/vault/secrets/db%2Fpw?region=us01" {
		t.Errorf("requestURI: got %v", captured["requestURI"])
	}
	var body map[string]any
	_ = json.Unmarshal([]byte(captured["body"].(string)), &body)
	wantB64 := base64.StdEncoding.EncodeToString([]byte("hunter2"))
	if body["value"] != wantB64 {
		t.Errorf("body.value: got %v, want base64 %q", body["value"], wantB64)
	}
	if body["value"] == "hunter2" {
		t.Errorf("body.value must not be raw plaintext on the wire")
	}
}

func TestSecretsRevealChallenge(t *testing.T) {
	captured := map[string]any{}
	server := httptest.NewServer(captureVaultHandler(captured, 200, `{"data":{"challenge":"rvc_1","expiresAt":"t"},"meta":{}}`))
	defer server.Close()

	c, _ := NewClient(ClientOpts{BaseURL: server.URL, ProjectSlug: "member-portal"})
	rc, _, kerr := c.Kibo.Vault.Secrets.RevealChallenge(context.Background(), "db/pw", "us01")
	if kerr != nil {
		t.Fatalf("RevealChallenge: %v", kerr)
	}
	if captured["method"] != "POST" {
		t.Errorf("method: got %v", captured["method"])
	}
	uri, _ := captured["requestURI"].(string)
	if !strings.Contains(uri, "op=reveal-challenge") || !strings.Contains(uri, "region=us01") {
		t.Errorf("requestURI: got %q", uri)
	}
	if rc.Challenge != "rvc_1" {
		t.Errorf("Challenge: got %q", rc.Challenge)
	}
}

func TestSecretsRevealDecodesBase64AndSendsHeader(t *testing.T) {
	captured := map[string]any{}
	wireValue := base64.StdEncoding.EncodeToString([]byte("hunter2"))
	respBody := `{"data":{"id":"sec_abc","name":"db/pw","version":1,"value":"` + wireValue +
		`","revealedAt":"t","attestationTier":{"urn":"u","operator":"platform","verified":true}},"meta":{}}`
	server := httptest.NewServer(captureVaultHandler(captured, 200, respBody))
	defer server.Close()

	c, _ := NewClient(ClientOpts{BaseURL: server.URL, ProjectSlug: "member-portal"})
	result, _, kerr := c.Kibo.Vault.Secrets.Reveal(context.Background(), "db/pw", "us01", "rvc_1")
	if kerr != nil {
		t.Fatalf("Reveal: %v", kerr)
	}
	if captured["method"] != "GET" {
		t.Errorf("method: got %v", captured["method"])
	}
	headers, _ := captured["headers"].(http.Header)
	if headers.Get("X-K9O-Reveal-Challenge") != "rvc_1" {
		t.Errorf("X-K9O-Reveal-Challenge header: got %q", headers.Get("X-K9O-Reveal-Challenge"))
	}
	if result.Value != "hunter2" {
		t.Errorf("Value: got %q, want decoded plaintext %q", result.Value, "hunter2")
	}
	if result.AttestationTier.Operator != "platform" {
		t.Errorf("AttestationTier.Operator: got %q", result.AttestationTier.Operator)
	}
}

func TestSecretsUpdateNarrowResult(t *testing.T) {
	captured := map[string]any{}
	server := httptest.NewServer(captureVaultHandler(captured, 200, `{"data":{"id":"sec_abc","name":"db/pw","version":2},"meta":{}}`))
	defer server.Close()

	c, _ := NewClient(ClientOpts{BaseURL: server.URL, ProjectSlug: "member-portal"})
	result, _, kerr := c.Kibo.Vault.Secrets.Update(context.Background(), "db/pw", UpdateSecretInput{Region: "us01", Value: "newpw"})
	if kerr != nil {
		t.Fatalf("Update: %v", kerr)
	}
	if captured["method"] != "PUT" {
		t.Errorf("method: got %v", captured["method"])
	}
	if captured["requestURI"] != "/@member-portal/vault/secrets/db%2Fpw?region=us01" {
		t.Errorf("requestURI: got %v", captured["requestURI"])
	}
	var body map[string]any
	_ = json.Unmarshal([]byte(captured["body"].(string)), &body)
	if body["value"] != base64.StdEncoding.EncodeToString([]byte("newpw")) {
		t.Errorf("body.value: got %v", body["value"])
	}
	if result.Version != 2 {
		t.Errorf("Version: got %d", result.Version)
	}
}

func TestSecretsRotateUndocumentedShape(t *testing.T) {
	captured := map[string]any{}
	server := httptest.NewServer(captureVaultHandler(captured, 200, `{"data":{"id":"sec_abc","name":"db/pw","version":2,"rotatedAt":"t","ledgerSequenceNumber":2},"meta":{}}`))
	defer server.Close()

	c, _ := NewClient(ClientOpts{BaseURL: server.URL, ProjectSlug: "member-portal"})
	result, _, kerr := c.Kibo.Vault.Secrets.Rotate(context.Background(), "db/pw", "us01")
	if kerr != nil {
		t.Fatalf("Rotate: %v", kerr)
	}
	if captured["method"] != "POST" {
		t.Errorf("method: got %v", captured["method"])
	}
	uri, _ := captured["requestURI"].(string)
	if !strings.Contains(uri, "op=rotate") || !strings.Contains(uri, "region=us01") {
		t.Errorf("requestURI: got %q", uri)
	}
	if captured["body"] != nil && captured["body"] != "" {
		t.Errorf("Rotate must send no body, got %v", captured["body"])
	}
	if result.LedgerSequenceNumber != 2 {
		t.Errorf("LedgerSequenceNumber: got %d", result.LedgerSequenceNumber)
	}
}

func TestSecretsMoveUndocumentedShape(t *testing.T) {
	captured := map[string]any{}
	server := httptest.NewServer(captureVaultHandler(captured, 200, `{"data":{"id":"sec_abc","name":"db/pw2","event":"SECRET_PATH_CHANGED"},"meta":{}}`))
	defer server.Close()

	c, _ := NewClient(ClientOpts{BaseURL: server.URL, ProjectSlug: "member-portal"})
	result, _, kerr := c.Kibo.Vault.Secrets.Move(context.Background(), "db/pw", "us01", "db/pw2")
	if kerr != nil {
		t.Fatalf("Move: %v", kerr)
	}
	if captured["method"] != "POST" {
		t.Errorf("method: got %v", captured["method"])
	}
	uri, _ := captured["requestURI"].(string)
	if !strings.Contains(uri, "op=move") || !strings.Contains(uri, "region=us01") {
		t.Errorf("requestURI: got %q", uri)
	}
	var body map[string]any
	_ = json.Unmarshal([]byte(captured["body"].(string)), &body)
	if body["newPath"] != "db/pw2" {
		t.Errorf("body.newPath: got %v", body["newPath"])
	}
	if result.Event != "SECRET_PATH_CHANGED" {
		t.Errorf("Event: got %q", result.Event)
	}
}

func TestSecretsDeleteHandles204(t *testing.T) {
	captured := map[string]any{}
	server := httptest.NewServer(captureVaultHandler(captured, 204, ""))
	defer server.Close()

	c, _ := NewClient(ClientOpts{BaseURL: server.URL, ProjectSlug: "member-portal"})
	meta, kerr := c.Kibo.Vault.Secrets.Delete(context.Background(), "db/pw", "us01")
	if kerr != nil {
		t.Fatalf("Delete: %v", kerr)
	}
	if captured["method"] != "DELETE" {
		t.Errorf("method: got %v", captured["method"])
	}
	if captured["requestURI"] != "/@member-portal/vault/secrets/db%2Fpw?region=us01" {
		t.Errorf("requestURI: got %v", captured["requestURI"])
	}
	if meta == nil {
		t.Errorf("expected non-nil Meta")
	}
}

func TestSecretsRequireProjectSlug(t *testing.T) {
	c, _ := NewClient(ClientOpts{BaseURL: "https://api.k9o.io"})
	_, _, kerr := c.Kibo.Vault.Secrets.Get(context.Background(), "db/pw", "us01")
	if kerr == nil {
		t.Fatal("expected error")
	}
	if kerr.Code != "K9O_MISSING_PROJECT_CONTEXT" {
		t.Errorf("Code: got %q", kerr.Code)
	}
}
