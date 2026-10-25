package k9o

import (
	"context"
	"encoding/json"
	"time"
)

// RealtimeEvent is a single event delivered on a subscription.
type RealtimeEvent struct {
	Topic      string          `json:"topic"`
	Type       string          `json:"type"`
	Sequence   int             `json:"sequence"`
	Payload    json.RawMessage `json:"payload"`
	OccurredAt string          `json:"occurredAt"`
}

// ReplayInput is the body for RealtimeNamespace.Replay.
type ReplayInput struct {
	Topic         string         `json:"topic"`
	FromSequence  int            `json:"fromSequence"`
	Filter        map[string]any `json:"filter,omitempty"`
	UntilSequence *int           `json:"untilSequence,omitempty"`
	Limit         *int           `json:"limit,omitempty"`
}

// SubscribeOptions configures Subscribe.
type SubscribeOptions struct {
	Filter map[string]any
	// Transport: "sse" (default) or "poll". WebSocket is omitted in the
	// net/http-based Go SDK; SSE is the richest portable transport.
	Transport    string
	PollInterval time.Duration
}

// RealtimeTopics provides typed builders for the 6 closed topic
// families.
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

// Subscribe subscribes to a topic, fanning RealtimeEvents onto the
// returned channel (per the Go async-iteration idiom). Errors
// arrive on the errs channel. Cancel via ctx; both channels close on
// termination. The SDK selects the transport.
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

	if opts.Transport == "poll" {
		return n.pollSubscribe(ctx, slug, topic, opts)
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

func (n *RealtimeNamespace) pollSubscribe(ctx context.Context, slug, topic string, opts SubscribeOptions) (<-chan RealtimeEvent, <-chan *KError) {
	events := make(chan RealtimeEvent)
	errs := make(chan *KError, 1)
	interval := opts.PollInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	go func() {
		defer close(events)
		defer close(errs)
		cursor := 0
		for {
			env, kerr := doRequest[[]RealtimeEvent](ctx, n.config, &requestInit{
				Method: "POST",
				Path:   ProjectPath(slug, "realtime/replay"),
				Body:   map[string]any{"topic": topic, "filter": opts.Filter, "fromSequence": cursor},
			})
			if kerr != nil {
				errs <- kerr
				return
			}
			for _, ev := range env.Data {
				if ev.Sequence+1 > cursor {
					cursor = ev.Sequence + 1
				}
				select {
				case events <- ev:
				case <-ctx.Done():
					return
				}
			}
			select {
			case <-time.After(interval):
			case <-ctx.Done():
				return
			}
		}
	}()
	return events, errs
}

// Replay performs a one-shot historical catch-up
// (capped at 10k events server-side).
func (n *RealtimeNamespace) Replay(ctx context.Context, input ReplayInput) ([]RealtimeEvent, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "realtime")
	if kerr != nil {
		return nil, nil, kerr
	}
	if input.FromSequence < 0 {
		return nil, nil, &KError{
			Code:           "K9O_INVALID_FILTER",
			Message:        "replay FromSequence must be a non-negative integer",
			Context:        map[string]any{"fromSequence": input.FromSequence},
			Recommendation: "Pass the highest sequence number you have already observed (or 0 to replay from the start).",
		}
	}
	env, kerr := doRequest[[]RealtimeEvent](ctx, n.config, &requestInit{
		Method: "POST",
		Path:   ProjectPath(slug, "realtime/replay"),
		Body:   input,
	})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}
