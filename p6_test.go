package k9o

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// roundTripFunc lets us stub http.Client for capture + canned responses.
type roundTripFunc func(*http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req), nil }

func jsonResponse(status int, body any) *http.Response {
	b, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(b))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func sseResponse(sse string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(sse)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
}

func p6Client(t *testing.T, rt roundTripFunc) *Client {
	t.Helper()
	c, kerr := NewClient(ClientOpts{
		BaseURL:     "https://acme.api.k9o.io",
		ProjectSlug: "portal",
		HTTPClient:  &http.Client{Transport: noProbe(rt)},
	})
	if kerr != nil {
		t.Fatalf("NewClient: %v", kerr)
	}
	return c
}

func TestStudioWorkflows(t *testing.T) {
	var gotURL, gotMethod string
	rt := roundTripFunc(func(req *http.Request) *http.Response {
		gotURL = req.URL.String()
		gotMethod = req.Method
		if req.Method == "GET" && strings.HasSuffix(req.URL.Path, "/workflows") {
			return jsonResponse(200, map[string]any{"data": []any{}, "meta": map[string]any{}})
		}
		return jsonResponse(200, map[string]any{"data": map[string]any{"id": "wfl_x"}, "meta": map[string]any{}})
	})
	c := p6Client(t, rt)
	ctx := context.Background()

	if _, _, kerr := c.Studio.Workflows.List(ctx, WorkflowListFilters{Status: "active"}); kerr != nil {
		t.Fatalf("List: %v", kerr)
	}
	if !strings.Contains(gotURL, "/@portal/workflows") || !strings.Contains(gotURL, "status=active") {
		t.Fatalf("List URL = %s", gotURL)
	}

	if _, _, kerr := c.Studio.Workflows.Create(ctx, CreateWorkflowInput{Name: "onboard", TriggerType: "manual", TriggerConfig: map[string]any{}}); kerr != nil {
		t.Fatalf("Create: %v", kerr)
	}
	if gotMethod != "POST" || gotURL != "https://acme.api.k9o.io/@portal/workflows" {
		t.Fatalf("Create %s %s", gotMethod, gotURL)
	}

	if _, _, kerr := c.Studio.Workflows.Trigger(ctx, "wfl_x", map[string]any{"foo": "bar"}); kerr != nil {
		t.Fatalf("Trigger: %v", kerr)
	}
	if gotURL != "https://acme.api.k9o.io/@portal/workflows/wfl_x/runs" {
		t.Fatalf("Trigger URL = %s", gotURL)
	}

	if _, _, kerr := c.Studio.Workflows.Runs.Cancel(ctx, "wfl_x", "wfr_y"); kerr != nil {
		t.Fatalf("Runs.Cancel: %v", kerr)
	}
	if gotURL != "https://acme.api.k9o.io/@portal/workflows/wfl_x/runs/wfr_y/cancel" {
		t.Fatalf("Cancel URL = %s", gotURL)
	}
}

func TestStudioDashboards(t *testing.T) {
	var gotURL, gotMethod string
	rt := roundTripFunc(func(req *http.Request) *http.Response {
		gotURL = req.URL.String()
		gotMethod = req.Method
		return jsonResponse(200, map[string]any{"data": map[string]any{"id": "dsh_x"}, "meta": map[string]any{}})
	})
	c := p6Client(t, rt)
	ctx := context.Background()

	if _, _, kerr := c.Studio.Dashboards.Pin(ctx, "dsh_x"); kerr != nil {
		t.Fatalf("Pin: %v", kerr)
	}
	if gotMethod != "POST" || gotURL != "https://acme.api.k9o.io/@portal/dashboards/dsh_x/pin" {
		t.Fatalf("Pin %s %s", gotMethod, gotURL)
	}
}

func TestComputeLLMs(t *testing.T) {
	var gotURL string
	rt := roundTripFunc(func(req *http.Request) *http.Response {
		gotURL = req.URL.String()
		return jsonResponse(200, map[string]any{"data": map[string]any{"modelId": "mistral-7b", "content": "hi", "finishReason": "stop", "usage": map[string]any{"promptTokens": 1, "completionTokens": 1, "totalTokens": 2}, "usageType": "sdk.direct"}, "meta": map[string]any{}})
	})
	c := p6Client(t, rt)
	ctx := context.Background()

	resp, _, kerr := c.Compute.LLMs.Invoke(ctx, "mistral-7b", LLMInvokeInput{Prompt: "hello"})
	if kerr != nil {
		t.Fatalf("Invoke: %v", kerr)
	}
	if gotURL != "https://acme.api.k9o.io/@portal/compute/llms/mistral-7b/invoke" {
		t.Fatalf("Invoke URL = %s", gotURL)
	}
	if resp.Content != "hi" || resp.Usage.TotalTokens != 2 {
		t.Fatalf("response = %+v", resp)
	}
}

func TestComputeLLMsInvokeStream(t *testing.T) {
	var gotURL string
	sse := `data: {"choices":[{"delta":{"content":"Hel"}}]}` + "\n\n" + `data: {"choices":[{"delta":{"content":"lo"},"finish_reason":"stop"}],"usage":{"total_tokens":1}}` + "\n\n" + "data: [DONE]\n\n"
	rt := roundTripFunc(func(req *http.Request) *http.Response {
		gotURL = req.URL.String()
		return sseResponse(sse)
	})
	c := p6Client(t, rt)
	events, errs := c.Compute.LLMs.InvokeStream(context.Background(), "gpt-4o", LLMInvokeInput{Prompt: "hi"})
	var sb strings.Builder
	for ev := range events {
		if len(ev.Choices) > 0 && ev.Choices[0].Delta.Content != nil {
			sb.WriteString(*ev.Choices[0].Delta.Content)
		}
	}
	if kerr := <-errs; kerr != nil {
		t.Fatalf("stream err: %v", kerr)
	}
	if gotURL != "https://acme.api.k9o.io/@portal/compute/llms/gpt-4o/invoke-stream" {
		t.Fatalf("InvokeStream URL = %s", gotURL)
	}
	if sb.String() != "Hello" {
		t.Fatalf("stream delta = %q", sb.String())
	}
}

func TestOrgDefaultLLM(t *testing.T) {
	var gotURL, gotMethod string
	rt := roundTripFunc(func(req *http.Request) *http.Response {
		gotURL = req.URL.String()
		gotMethod = req.Method
		return jsonResponse(200, map[string]any{"data": map[string]any{"orgId": "org_x", "defaultModelId": "mistral-7b"}, "meta": map[string]any{}})
	})
	c, _ := NewClient(ClientOpts{BaseURL: "https://acme.api.k9o.io", HTTPClient: &http.Client{Transport: noProbe(rt)}})
	if _, _, kerr := c.Org.DefaultLLMGet(context.Background()); kerr != nil {
		t.Fatalf("DefaultLLMGet: %v", kerr)
	}
	if gotURL != "https://acme.api.k9o.io/default-llm" {
		t.Fatalf("Get URL = %s", gotURL)
	}
	if _, _, kerr := c.Org.DefaultLLMSet(context.Background(), "claude-3-5-sonnet"); kerr != nil {
		t.Fatalf("DefaultLLMSet: %v", kerr)
	}
	if gotMethod != "PATCH" {
		t.Fatalf("Set method = %s", gotMethod)
	}
}

func TestRealtimeTopicBuilders(t *testing.T) {
	if got := RealtimeTopic.Collection("orders"); got != "collections.orders.*" {
		t.Fatalf("Collection = %s", got)
	}
	if got := RealtimeTopic.WorkflowRuns(); got != "studio.workflows.runs.*" {
		t.Fatalf("WorkflowRuns = %s", got)
	}
	if got := RealtimeTopic.DashboardRefresh("dsh_x"); got != "studio.dashboards.refresh.dsh_x" {
		t.Fatalf("DashboardRefresh = %s", got)
	}
}

func TestRealtimeSubscribeSSE(t *testing.T) {
	var gotURL string
	ev1, _ := json.Marshal(map[string]any{"topic": "collections.orders.*", "eventType": "row_inserted", "payload": map[string]any{}, "occurredAt": "t1"})
	ev2, _ := json.Marshal(map[string]any{"topic": "collections.orders.*", "eventType": "row_updated", "payload": map[string]any{}, "occurredAt": "t2"})
	sse := "data: " + string(ev1) + "\n\n" + "data: " + string(ev2) + "\n\n"
	rt := roundTripFunc(func(req *http.Request) *http.Response {
		gotURL = req.URL.String()
		return sseResponse(sse)
	})
	c := p6Client(t, rt)
	events, errs := c.Realtime.Subscribe(context.Background(), RealtimeTopic.Collection("orders"), SubscribeOptions{})
	var types []string
	for ev := range events {
		types = append(types, ev.EventType)
	}
	if kerr := <-errs; kerr != nil {
		t.Fatalf("subscribe err: %v", kerr)
	}
	if !strings.Contains(gotURL, "realtime/subscribe") || !strings.Contains(gotURL, "topic=collections.orders") {
		t.Fatalf("Subscribe URL = %s", gotURL)
	}
	if len(types) != 2 || types[0] != "row_inserted" || types[1] != "row_updated" {
		t.Fatalf("types = %v", types)
	}
}
