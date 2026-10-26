package k9o

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type streamsCall struct {
	Method string
	Path   string
	Query  map[string]string
	Body   string
}

func recordCall(req *http.Request) streamsCall {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	q := map[string]string{}
	for k, v := range req.URL.Query() {
		q[k] = v[0]
	}
	return streamsCall{Method: req.Method, Path: req.URL.EscapedPath(), Query: q, Body: string(body)}
}

func envelope(data any) *http.Response {
	return jsonResponse(200, map[string]any{"data": data, "meta": map[string]any{}})
}

func TestStreamsControlPlane(t *testing.T) {
	var calls []streamsCall
	rt := roundTripFunc(func(req *http.Request) *http.Response {
		calls = append(calls, recordCall(req))
		switch {
		case req.Method == "DELETE":
			return envelope(map[string]any{"slug": "orders", "status": "deleted"})
		case req.Method == "GET" && strings.HasSuffix(req.URL.Path, "/user"):
			return envelope([]any{map[string]any{"slug": "orders", "maxBytes": 1073741824}})
		case strings.HasSuffix(req.URL.Path, "/publish"):
			return envelope(map[string]any{"streamSlug": "orders", "subject": "k9o.x.user.pstr_1.eu", "bytes": 8, "accepted": true})
		case strings.HasSuffix(req.URL.Path, "/replay"):
			return envelope(map[string]any{"streamSlug": "orders", "events": []any{map[string]any{"sequence": "11", "subject": "s", "data": map[string]any{"id": 1}, "publishedAt": "t"}}, "nextCursor": "11", "hasMore": false})
		}
		return envelope(map[string]any{"slug": "orders", "displayName": "Orders", "maxBytes": nil, "retentionSeconds": 86400})
	})
	c := p6Client(t, rt)
	ctx := context.Background()

	st, _, kerr := c.Streams.Create(ctx, "Orders", 1073741824, 0)
	if kerr != nil || st.Slug != "orders" || st.MaxBytes != nil {
		t.Fatalf("Create: %+v %v", st, kerr)
	}
	list, _, kerr := c.Streams.List(ctx)
	if kerr != nil || len(list) != 1 || *list[0].MaxBytes != 1073741824 {
		t.Fatalf("List: %+v %v", list, kerr)
	}
	if _, _, kerr = c.Streams.Get(ctx, "orders"); kerr != nil {
		t.Fatal(kerr)
	}
	del, _, kerr := c.Streams.Delete(ctx, "orders")
	if kerr != nil || del.Status != "deleted" {
		t.Fatalf("Delete: %+v %v", del, kerr)
	}
	pub, _, kerr := c.Streams.Publish(ctx, "orders", map[string]any{"id": 1}, "")
	if kerr != nil || !pub.Accepted || pub.Bytes != 8 {
		t.Fatalf("Publish: %+v %v", pub, kerr)
	}
	from := int64(10)
	rep, _, kerr := c.Streams.Replay(ctx, "orders", ReplayOptions{FromSequence: &from})
	if kerr != nil || len(rep.Events) != 1 || rep.Events[0].Sequence != "11" || string(rep.Events[0].Data) != `{"id":1}` || *rep.NextCursor != "11" {
		t.Fatalf("Replay: %+v %v", rep, kerr)
	}

	want := []streamsCall{
		{"POST", "/@portal/data-streams/user", nil, `{"displayName":"Orders","maxBytes":1073741824}`},
		{"GET", "/@portal/data-streams/user", nil, ""},
		{"GET", "/@portal/data-streams/user/orders", nil, ""},
		{"DELETE", "/@portal/data-streams/user/orders", nil, ""},
		{"POST", "/@portal/data-streams/user/orders/publish", nil, `{"data":{"id":1}}`},
		{"POST", "/@portal/data-streams/user/orders/replay", nil, `{"fromSequence":10}`},
	}
	if len(calls) != len(want) {
		t.Fatalf("calls = %+v", calls)
	}
	for i, w := range want {
		g := calls[i]
		if g.Method != w.Method || g.Path != w.Path || g.Body != w.Body || len(g.Query) != 0 {
			t.Fatalf("call %d = %+v, want %+v", i, g, w)
		}
	}
}

func TestStreamsRequireProject(t *testing.T) {
	c, _ := NewClient(ClientOpts{BaseURL: "https://acme.api.k9o.io"})
	_, _, kerr := c.Streams.List(context.Background())
	if kerr == nil || kerr.Code != "K9O_MISSING_PROJECT_CONTEXT" {
		t.Fatalf("kerr = %v", kerr)
	}
	ev, errs := c.Streams.Subscribe(context.Background(), "orders", "")
	for range ev {
	}
	if e := <-errs; e == nil || e.Code != "K9O_MISSING_PROJECT_CONTEXT" {
		t.Fatalf("subscribe err = %v", e)
	}
}

func TestStreamsUnavailableMapsToKError(t *testing.T) {
	rt := roundTripFunc(func(req *http.Request) *http.Response {
		return jsonResponse(503, map[string]any{"code": "K9O_STREAM_UNAVAILABLE", "message": "The Pulse Stream data plane is not yet available in this deployment.", "context": map[string]any{}})
	})
	c := p6Client(t, rt)
	c.config.Retry = &RetryConfig{MaxRetries: 0, Backoff: "none"}
	_, _, kerr := c.Streams.Publish(context.Background(), "orders", 1, "")
	if kerr == nil || kerr.Code != "K9O_STREAM_UNAVAILABLE" || kerr.Status != 503 {
		t.Fatalf("Publish kerr = %+v", kerr)
	}
	ev, errs := c.Streams.Subscribe(context.Background(), "orders", "")
	for range ev {
	}
	e := <-errs
	if e == nil || e.Code != "K9O_STREAM_UNAVAILABLE" || e.Status != 503 {
		t.Fatalf("Subscribe err = %+v", e)
	}
}

func TestStreamsSubscribeResumesFromLastSequence(t *testing.T) {
	old := streamReconnectDelay
	streamReconnectDelay = time.Millisecond
	defer func() { streamReconnectDelay = old }()

	var mu sync.Mutex
	var calls []streamsCall
	frame := func(seq string) string {
		b, _ := json.Marshal(map[string]any{"sequence": seq, "subject": "s", "data": map[string]any{"n": seq}, "publishedAt": "t"})
		return "id: " + seq + "\ndata: " + string(b) + "\n\n"
	}
	rt := roundTripFunc(func(req *http.Request) *http.Response {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, recordCall(req))
		switch len(calls) {
		case 1:
			return sseResponse("retry: 1000\n:ok\n\n" + frame("1") + frame("2"))
		case 2:
			return sseResponse("retry: 1000\n:ok\n\n" + frame("3"))
		}
		return sseResponse("retry: 1000\n:ok\n\n")
	})
	c := p6Client(t, rt)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ev, errs := c.Streams.Subscribe(ctx, "orders", "")
	var got []string
	for e := range ev {
		got = append(got, e.Sequence)
		if len(got) == 3 {
			cancel()
		}
	}
	if kerr := <-errs; kerr != nil {
		t.Fatalf("err = %v", kerr)
	}
	if strings.Join(got, ",") != "1,2,3" {
		t.Fatalf("sequences = %v", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(calls) < 2 {
		t.Fatalf("calls = %+v", calls)
	}
	if _, has := calls[0].Query["fromSequence"]; has || calls[0].Method != "GET" || calls[0].Path != "/@portal/data-streams/user/orders/subscribe" {
		t.Fatalf("first call = %+v", calls[0])
	}
	if calls[1].Query["fromSequence"] != "2" {
		t.Fatalf("second call = %+v, want fromSequence=2", calls[1])
	}
	if len(calls) > 2 && calls[2].Query["fromSequence"] != "3" {
		t.Fatalf("third call = %+v, want fromSequence=3", calls[2])
	}
}
