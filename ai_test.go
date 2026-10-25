package k9o

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const aiOK = `{"data":{"embeddings":[[0.1,0.2]],"model":"nomic-embed-text-v1.5","dimensions":2},"meta":{}}`

func aiCapture(t *testing.T, status int, resp any) (*Client, *struct {
	url, method string
	body        map[string]any
}) {
	t.Helper()
	cap := &struct {
		url, method string
		body        map[string]any
	}{}
	c := p6Client(t, func(req *http.Request) *http.Response {
		cap.url, cap.method = req.URL.String(), req.Method
		b, _ := io.ReadAll(req.Body)
		cap.body = nil
		_ = json.Unmarshal(b, &cap.body)
		return jsonResponse(status, resp)
	})
	return c, cap
}

func aiOKBody() any {
	var v any
	_ = json.Unmarshal([]byte(aiOK), &v)
	return v
}

func TestAIEmbedURLVerbString(t *testing.T) {
	c, cap := aiCapture(t, 200, aiOKBody())
	r, _, kerr := c.AI.Embed(context.Background(), EmbedInput{Input: "hello"})
	if kerr != nil {
		t.Fatalf("Embed: %v", kerr)
	}
	if cap.method != "POST" || cap.url != "https://acme.api.k9o.io/@portal/ai/embeddings" {
		t.Fatalf("got %s %s", cap.method, cap.url)
	}
	if s, ok := cap.body["input"].(string); !ok || s != "hello" {
		t.Fatalf("input = %#v", cap.body["input"])
	}
	if _, has := cap.body["model"]; has {
		t.Fatalf("model must be omitted")
	}
	if len(r.Embeddings) != 1 || r.Embeddings[0][1] != 0.2 || r.Dimensions != 2 || r.Model != "nomic-embed-text-v1.5" {
		t.Fatalf("result = %+v", r)
	}
}

func TestAIEmbedListWithModel(t *testing.T) {
	c, cap := aiCapture(t, 200, aiOKBody())
	_, _, kerr := c.AI.Embed(context.Background(), EmbedInput{Model: "nomic-embed-text-v1.5", Input: []string{"a", "b"}})
	if kerr != nil {
		t.Fatalf("Embed: %v", kerr)
	}
	in, ok := cap.body["input"].([]any)
	if !ok || len(in) != 2 || in[0] != "a" || in[1] != "b" {
		t.Fatalf("input = %#v", cap.body["input"])
	}
	if cap.body["model"] != "nomic-embed-text-v1.5" {
		t.Fatalf("model = %#v", cap.body["model"])
	}
}

func TestAIEmbedModelOmittedForList(t *testing.T) {
	c, cap := aiCapture(t, 200, aiOKBody())
	if _, _, kerr := c.AI.Embed(context.Background(), EmbedInput{Input: []string{"x"}}); kerr != nil {
		t.Fatalf("Embed: %v", kerr)
	}
	if _, has := cap.body["model"]; has {
		t.Fatalf("model must be omitted")
	}
}

func TestAIEmbedRequiresProject(t *testing.T) {
	c, _ := NewClient(ClientOpts{BaseURL: "https://acme.api.k9o.io"})
	_, _, kerr := c.AI.Embed(context.Background(), EmbedInput{Input: "x"})
	if kerr == nil || kerr.Code != "K9O_MISSING_PROJECT_CONTEXT" {
		t.Fatalf("expected K9O_MISSING_PROJECT_CONTEXT, got %v", kerr)
	}
}

func TestAIEmbedServerRefusalOmitsInput(t *testing.T) {
	secret := "the-secret-document-body"
	c, _ := aiCapture(t, 400, map[string]any{
		"code": "K9O_MODEL_NOT_DECLARED", "message": "model is not a declared embedding model",
		"context": map[string]any{"declared": []string{"nomic-embed-text-v1.5"}},
	})
	_, _, kerr := c.AI.Embed(context.Background(), EmbedInput{Model: "other", Input: secret})
	if kerr == nil || kerr.Code != "K9O_MODEL_NOT_DECLARED" {
		t.Fatalf("expected K9O_MODEL_NOT_DECLARED, got %v", kerr)
	}
	if strings.Contains(kerr.Error(), secret) {
		t.Fatalf("error leaks input: %s", kerr.Error())
	}
	if j, _ := json.Marshal(kerr); strings.Contains(string(j), secret) {
		t.Fatalf("error JSON leaks input")
	}
}
