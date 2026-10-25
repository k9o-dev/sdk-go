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

// AuditQueryOpts are optional filters for AuditNamespace.Query.
type AuditQueryOpts struct {
	Since      string
	Until      string
	Type       string
	ActorID    string
	TargetKind string
	TargetID   string
	Limit      int
	Cursor     string
}

func (o AuditQueryOpts) toQuery() map[string]string {
	q := map[string]string{}
	if o.Since != "" {
		q["since"] = o.Since
	}
	if o.Until != "" {
		q["until"] = o.Until
	}
	if o.Type != "" {
		q["type"] = o.Type
	}
	if o.ActorID != "" {
		q["actorId"] = o.ActorID
	}
	if o.TargetKind != "" {
		q["targetKind"] = o.TargetKind
	}
	if o.TargetID != "" {
		q["targetId"] = o.TargetID
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
