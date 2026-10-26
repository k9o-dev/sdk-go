package k9o

import (
	"context"
	"strconv"
)

// AuditEvent is an audit event record.
type AuditEvent struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	ActorID    *string        `json:"actorId,omitempty"`
	TargetKind *string        `json:"targetKind,omitempty"`
	TargetID   *string        `json:"targetId,omitempty"`
	Payload    map[string]any `json:"payload"`
	OccurredAt string         `json:"occurredAt"`
	RequestID  *string        `json:"requestId,omitempty"`
	TraceID    *string        `json:"traceId,omitempty"`
}

// AuditQueryOpts are optional filters for AuditNamespace.Query; names follow
// the gateway's query parameters.
type AuditQueryOpts struct {
	// EventType filters by audit-event name (e.g. "data.write.access.insert").
	EventType string
	// ActorID filters by actor.
	ActorID string
	// After is an ISO 8601 lower bound: only events after this instant.
	After string
	// Before is an ISO 8601 upper bound: only events before this instant.
	Before string
	// Limit is the page size (the per-tier maximum is bounded server-side).
	Limit int
	// Cursor is the cursor from a prior response.
	Cursor string
}

func (o AuditQueryOpts) toQuery() map[string]string {
	q := map[string]string{}
	if o.EventType != "" {
		q["eventType"] = o.EventType
	}
	if o.ActorID != "" {
		q["actorId"] = o.ActorID
	}
	if o.After != "" {
		q["after"] = o.After
	}
	if o.Before != "" {
		q["before"] = o.Before
	}
	if o.Limit > 0 {
		q["limit"] = strconv.Itoa(o.Limit)
	}
	if o.Cursor != "" {
		q["cursor"] = o.Cursor
	}
	return q
}

// AuditNamespace exposes client.Audit.* (project-plane).
type AuditNamespace struct {
	config *httpClientConfig
}

// Query queries the audit log (reverse-chronological; filters AND-compose).
func (n *AuditNamespace) Query(ctx context.Context, opts AuditQueryOpts) ([]AuditEvent, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "audit")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[[]AuditEvent](ctx, n.config, &requestInit{Method: "GET", Path: ProjectPath(slug, "audit-log"), Query: opts.toQuery()})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}
