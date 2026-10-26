// Package k9o is the Kilimanjaro platform SDK for Go.
//
// Idiomatic shape: PascalCase exported methods +
// tuple-return (data, meta, err *KError) error delivery. KError is
// a concrete pointer type (nil on success) — caller pattern:
//
//	data, meta, kerr := client.From("posts").Select().Execute(ctx)
//	if kerr != nil {
//	    if kerr.Code == "K9O_TIER_CAP_HIT" { ... }
//	    return kerr
//	}
package k9o

import "fmt"

// KError is the closed-taxonomy error type returned by every k9o
// surface on non-2xx gateway responses or SDK-side validation failures.
//
// Implements the standard library `error` interface; safe to wrap
// with errors.Is / errors.As. Caller code narrows on `.Code` field
// per the closed taxonomy.
type KError struct {
	// Code is the closed-taxonomy code (e.g. "K9O_TIER_CAP_HIT").
	Code string
	// Message is the human-readable explanation.
	Message string
	// Context carries structured per-code payload.
	Context map[string]any
	// Recommendation is the actionable next step (may be empty string).
	Recommendation string
	// Status is the HTTP status the gateway returned (0 for SDK-side errors).
	Status int
	// RequestID is the gateway-emitted trace id.
	RequestID string
}

// Error satisfies the error interface.
func (e *KError) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("[%s] %s (status=%d)", e.Code, e.Message, e.Status)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// KErrorWire is the wire-shape sentinel matching the gateway's
// KError JSON payload.
type KErrorWire struct {
	Code           string         `json:"code"`
	Message        string         `json:"message"`
	Context        map[string]any `json:"context,omitempty"`
	Recommendation string         `json:"recommendation,omitempty"`
	RequestID      string         `json:"requestId,omitempty"`
}

// kerrorFromWire rehydrates a wire KError payload into a typed *KError.
// No allow-list — gateway code-space is open-ended; preserves whatever
// code the gateway sends. JSON tag handles wire camelCase → Go
// PascalCase translation at the boundary.
func kerrorFromWire(wire *KErrorWire, status int) *KError {
	return &KError{
		Code:           wire.Code,
		Message:        wire.Message,
		Context:        wire.Context,
		Recommendation: wire.Recommendation,
		Status:         status,
		RequestID:      wire.RequestID,
	}
}
