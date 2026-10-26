package k9o

import (
	"context"
	"encoding/json"
	"net/url"
	"time"
)

// Stream is a user data stream as the gateway returns it.
type Stream struct {
	ID               string `json:"id"`
	Slug             string `json:"slug"`
	ProjectID        string `json:"projectId"`
	DisplayName      string `json:"displayName"`
	SubjectPrefix    string `json:"subjectPrefix"`
	Region           string `json:"region"`
	RetentionSeconds int64  `json:"retentionSeconds"`
	RetentionClass   string `json:"retentionClass"`
	// MaxBytes is nil when the stream has no storage ceiling.
	MaxBytes  *int64  `json:"maxBytes"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"createdAt"`
	CreatedBy *string `json:"createdBy"`
}

// CreateStreamInput is the body of Streams.Create. RetentionSeconds is optional;
// zero omits it and the gateway applies its default.
type CreateStreamInput struct {
	DisplayName      string `json:"displayName"`
	MaxBytes         int64  `json:"maxBytes"`
	RetentionSeconds int64  `json:"retentionSeconds,omitempty"`
}

// StreamDeleted is the result of Streams.Delete.
type StreamDeleted struct {
	Slug   string `json:"slug"`
	Status string `json:"status"`
}

// PublishResult is the result of Streams.Publish.
type PublishResult struct {
	StreamSlug string `json:"streamSlug"`
	Subject    string `json:"subject"`
	Bytes      int64  `json:"bytes"`
	Accepted   bool   `json:"accepted"`
}

// StreamEvent is one event of a stream. Sequence is the stream sequence
// (a decimal string); Data is the published JSON payload.
type StreamEvent struct {
	Sequence    string          `json:"sequence"`
	Subject     string          `json:"subject"`
	Data        json.RawMessage `json:"data"`
	PublishedAt string          `json:"publishedAt"`
}

// ReplayOptions bounds a Streams.Replay read. Nil fields are omitted.
type ReplayOptions struct {
	FromSequence  *int64 `json:"fromSequence,omitempty"`
	UntilSequence *int64 `json:"untilSequence,omitempty"`
	Limit         int    `json:"limit,omitempty"`
}

// ReplayResult is a page of stream events. Pass NextCursor as the next
// FromSequence while HasMore is true.
type ReplayResult struct {
	StreamSlug string        `json:"streamSlug"`
	Events     []StreamEvent `json:"events"`
	NextCursor *string       `json:"nextCursor"`
	HasMore    bool          `json:"hasMore"`
}

// streamReconnectDelay is the pause before Subscribe reconnects after the
// server closes the connection (the server bounds each connection's lifetime).
var streamReconnectDelay = time.Second

// StreamsNamespace exposes client.Streams.* (user data streams).
type StreamsNamespace struct {
	config *httpClientConfig
}

func streamsPath(slug, rest string) string {
	p := "data-streams/user"
	if rest != "" {
		p += "/" + rest
	}
	return ProjectPath(slug, p)
}

// Create creates a user stream. maxBytes is the storage ceiling in bytes.
// retentionSeconds is optional: pass 0 to use the default.
func (n *StreamsNamespace) Create(ctx context.Context, displayName string, maxBytes int64, retentionSeconds int64) (*Stream, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "streams")
	if kerr != nil {
		return nil, nil, kerr
	}
	body := CreateStreamInput{DisplayName: displayName, MaxBytes: maxBytes, RetentionSeconds: retentionSeconds}
	env, kerr := doRequest[Stream](ctx, n.config, &requestInit{Method: "POST", Path: streamsPath(slug, ""), Body: body})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// List returns the project's user streams.
func (n *StreamsNamespace) List(ctx context.Context) ([]Stream, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "streams")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[[]Stream](ctx, n.config, &requestInit{Method: "GET", Path: streamsPath(slug, "")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Get returns one stream by slug.
func (n *StreamsNamespace) Get(ctx context.Context, streamSlug string) (*Stream, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "streams")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[Stream](ctx, n.config, &requestInit{Method: "GET", Path: streamsPath(slug, url.PathEscape(streamSlug))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Delete retires a stream.
func (n *StreamsNamespace) Delete(ctx context.Context, streamSlug string) (*StreamDeleted, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "streams")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[StreamDeleted](ctx, n.config, &requestInit{Method: "DELETE", Path: streamsPath(slug, url.PathEscape(streamSlug))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Publish appends an event. data is any JSON-serialisable value; subject is an
// optional dot-separated routing suffix (empty omits it). Needs project admin.
func (n *StreamsNamespace) Publish(ctx context.Context, streamSlug string, data any, subject string) (*PublishResult, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "streams")
	if kerr != nil {
		return nil, nil, kerr
	}
	body := map[string]any{"data": data}
	if subject != "" {
		body["subject"] = subject
	}
	env, kerr := doRequest[PublishResult](ctx, n.config, &requestInit{Method: "POST", Path: streamsPath(slug, url.PathEscape(streamSlug)+"/publish"), Body: body})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Replay reads a bounded range of events in one call. Needs project admin or auditor.
func (n *StreamsNamespace) Replay(ctx context.Context, streamSlug string, opts ReplayOptions) (*ReplayResult, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "streams")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[ReplayResult](ctx, n.config, &requestInit{Method: "POST", Path: streamsPath(slug, url.PathEscape(streamSlug)+"/replay"), Body: opts})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Subscribe tails a stream over SSE (GET .../subscribe), delivering events on
// the returned channel. fromSequence resumes after that sequence (empty starts
// at the beginning). The server closes each connection after a bounded time;
// Subscribe reconnects from the last delivered sequence until ctx is cancelled.
// A failure (for example K9O_STREAM_UNAVAILABLE) arrives on the errs channel
// and ends the subscription. Both channels close on termination. Needs project
// admin or auditor.
func (n *StreamsNamespace) Subscribe(ctx context.Context, streamSlug string, fromSequence string) (<-chan StreamEvent, <-chan *KError) {
	events := make(chan StreamEvent)
	errs := make(chan *KError, 1)
	slug, kerr := requireProjectSlug(n.config, "streams")
	if kerr != nil {
		errs <- kerr
		close(errs)
		close(events)
		return events, errs
	}
	path := streamsPath(slug, url.PathEscape(streamSlug)+"/subscribe")

	go func() {
		defer close(events)
		defer close(errs)
		last := fromSequence
		for ctx.Err() == nil {
			q := map[string]string{}
			if last != "" {
				q["fromSequence"] = last
			}
			in, inErrs := streamSSE[StreamEvent](ctx, n.config, &requestInit{Method: "GET", Path: path, Query: q})
			for ev := range in {
				last = ev.Sequence
				select {
				case events <- ev:
				case <-ctx.Done():
					for range in {
					}
				}
			}
			if e := <-inErrs; e != nil {
				if ctx.Err() == nil {
					errs <- e
				}
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(streamReconnectDelay):
			}
		}
	}()
	return events, errs
}
