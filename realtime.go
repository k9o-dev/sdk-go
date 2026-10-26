package k9o

import (
	"context"
	"encoding/json"
)

// RealtimeEvent is a single event delivered on a subscription.
type RealtimeEvent struct {
	Topic      string          `json:"topic"`
	EventType  string          `json:"eventType"`
	Payload    json.RawMessage `json:"payload"`
	OccurredAt string          `json:"occurredAt"`
}

// SubscribeOptions configures Subscribe.
type SubscribeOptions struct {
	// Filter narrows the subscription (it composes with RLS: the filter narrows,
	// RLS gates). Sent as the JSON `filter` query parameter.
	Filter map[string]any
}

// realtimeTopics provides typed builders for the 6 closed topic families.
type realtimeTopics struct{}

// RealtimeTopic is the package-level entry to the topic builders.
var RealtimeTopic = realtimeTopics{}

func (realtimeTopics) Collection(collection string) string { return "collections." + collection + ".*" }
func (realtimeTopics) WorkflowRuns() string                { return "studio.workflows.runs.*" }
func (realtimeTopics) QueryExecuted(queryID string) string {
	return "studio.queries.executed." + queryID
}
func (realtimeTopics) DashboardRefresh(dashboardID string) string {
	return "studio.dashboards.refresh." + dashboardID
}
func (realtimeTopics) FunctionInvocations(functionID string) string {
	return "forge.functions.invocations." + functionID
}
func (realtimeTopics) AuditOrg(eventFamily string) string { return "audit.org." + eventFamily }

// RealtimeNamespace exposes client.Realtime.*.
type RealtimeNamespace struct {
	config *httpClientConfig
}

// Subscribe subscribes to a topic over SSE (GET /@<slug>/realtime/subscribe),
// fanning RealtimeEvents onto the returned channel. Errors arrive on the errs
// channel. Cancel via ctx; both channels close on termination. The gateway
// serves no WebSocket upgrade and no durable replay, so neither is offered:
// after an "event: resync" frame, re-read current state from the data plane
// and resubscribe.
func (n *RealtimeNamespace) Subscribe(ctx context.Context, topic string, opts SubscribeOptions) (<-chan RealtimeEvent, <-chan *KError) {
	slug, kerr := requireProjectSlug(n.config, "realtime")
	if kerr != nil {
		events := make(chan RealtimeEvent)
		errs := make(chan *KError, 1)
		errs <- kerr
		close(errs)
		close(events)
		return events, errs
	}

	q := map[string]string{"topic": topic}
	if opts.Filter != nil {
		if b, err := json.Marshal(opts.Filter); err == nil {
			q["filter"] = string(b)
		}
	}
	return streamSSE[RealtimeEvent](ctx, n.config, &requestInit{
		Method: "GET",
		Path:   ProjectPath(slug, "realtime/subscribe"),
		Query:  q,
	})
}
