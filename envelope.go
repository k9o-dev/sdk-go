package k9o

// Envelope is the standard wrapper returned by every k9o
// surface on success. Generic over the per-method data shape.
//
// Per Go idiomatic shape: PascalCase exported fields;
// JSON tags handle wire camelCase ↔ Go PascalCase translation.
type Envelope[T any] struct {
	Data T    `json:"data"`
	Meta Meta `json:"meta"`
}

// Meta carries the structured per-namespace accessor sugar.
// Sub-namespaces are pointer types — nil when the gateway didn't send
// the corresponding sub-namespace; populated when present.
type Meta struct {
	Cursor       *MetaCursor       `json:"cursor,omitempty"`
	Subscription *MetaSubscription `json:"subscription,omitempty"`
	Idempotency  *MetaIdempotency  `json:"idempotency,omitempty"`
	Warnings     []MetaWarning     `json:"warnings,omitempty"`
	Cost         map[string]any    `json:"cost,omitempty"`
	Timing       *MetaTiming       `json:"timing,omitempty"`
	Context      *MetaContext      `json:"context,omitempty"`
}

// MetaCursor is the pagination accessor.
type MetaCursor struct {
	Next       *string `json:"next"`
	TotalCount *int    `json:"totalCount,omitempty"`
}

// MetaSubscription carries subscription delivery metadata.
type MetaSubscription struct {
	IsInitial bool `json:"isInitial"`
}

// MetaIdempotency carries the idempotency replay flag.
type MetaIdempotency struct {
	Replayed bool `json:"replayed"`
}

// MetaWarning is a non-fatal warning.
type MetaWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// MetaTiming carries wall-clock timing. Sub-millisecond timings
// are fractional, so totalMs is a float (the gateway emits e.g. 10.572).
type MetaTiming struct {
	TotalMs float64 `json:"totalMs"`
}

// MetaContext carries request-correlation metadata.
type MetaContext struct {
	RequestID string `json:"requestId"`
}
