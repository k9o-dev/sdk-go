package k9o

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"testing"
)

func TestTranslateNamedParamsVectors(t *testing.T) {
	cases := []struct {
		name   string
		sql    string
		named  map[string]any
		want   string
		params []any
	}{
		{"1 operator text", `SELECT * FROM users WHERE data->>'role' = :role`, map[string]any{"role": "admin"}, `SELECT * FROM users WHERE data->>'role' = $1`, []any{"admin"}},
		{"2 reuse", `SELECT :a, :b, :a`, map[string]any{"a": 1, "b": 2}, `SELECT $1, $2, $1`, []any{1, 2}},
		{"3 quotes and casts", `SELECT ':not_a_param', ":also_not", x::int FROM t WHERE y = :y`, map[string]any{"y": true}, `SELECT ':not_a_param', ":also_not", x::int FROM t WHERE y = $1`, []any{true}},
		{"4 escaped quote and comment", "SELECT 'it''s :no' , :v -- :comment\nFROM t", map[string]any{"v": nil}, "SELECT 'it''s :no' , $1 -- :comment\nFROM t", []any{nil}},
		{"5 nested comment", `SELECT /* :c /* :d */ :e */ :f`, map[string]any{"f": 1}, `SELECT /* :c /* :d */ :e */ $1`, []any{1}},
		{"6 dollar quotes", `SELECT $$ :x $$, $q$ :y $q$, :z`, map[string]any{"z": "s"}, `SELECT $$ :x $$, $q$ :y $q$, $1`, []any{"s"}},
		{"7 slices", `SELECT arr[1:2], a:b, :n`, map[string]any{"n": 5}, `SELECT arr[1:2], a:b, $1`, []any{5}},
		{"escape string", `SELECT E'it\'s :no', :v`, map[string]any{"v": 1}, `SELECT E'it\'s :no', $1`, []any{1}},
		{"quoted identifier escape", `SELECT "a"":b", :v`, map[string]any{"v": 1}, `SELECT "a"":b", $1`, []any{1}},
		{"extra keys ignored", `SELECT :a`, map[string]any{"a": 1, "zzz": 2}, `SELECT $1`, []any{1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, params, kerr := translateNamedParams(tc.sql, tc.named)
			if kerr != nil {
				t.Fatalf("unexpected error: %v", kerr)
			}
			if got != tc.want || !reflect.DeepEqual(params, tc.params) {
				t.Fatalf("got %q %#v, want %q %#v", got, params, tc.want, tc.params)
			}
		})
	}
}

func TestTranslateNamedParamsErrors(t *testing.T) {
	_, _, kerr := translateNamedParams(`SELECT :missing`, map[string]any{})
	if kerr == nil || kerr.Code != "K9O_INVALID_FIELD_VALUE" || !contains(kerr.Message, ":missing") {
		t.Fatalf("vector 8: %v", kerr)
	}
	_, _, kerr = translateNamedParams(`SELECT :a`, map[string]any{"a": map[string]any{}})
	if kerr == nil || kerr.Code != "K9O_INVALID_FIELD_VALUE" {
		t.Fatalf("vector 9: %v", kerr)
	}
	_, _, kerr = translateNamedParams(`SELECT :a`, map[string]any{"a": []string{"x"}})
	if kerr == nil || kerr.Code != "K9O_INVALID_FIELD_VALUE" {
		t.Fatalf("array value: %v", kerr)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

type seenReq struct {
	method, path string
	query        map[string]string
	body         map[string]any
}

func queryClient(t *testing.T, seen *[]seenReq, resp string) *Client {
	t.Helper()
	return p6Client(t, func(req *http.Request) *http.Response {
		var body map[string]any
		if req.Body != nil {
			b, _ := io.ReadAll(req.Body)
			_ = json.Unmarshal(b, &body)
		}
		q := map[string]string{}
		for k, v := range req.URL.Query() {
			q[k] = v[0]
		}
		*seen = append(*seen, seenReq{req.Method, req.URL.Path, q, body})
		var v any
		_ = json.Unmarshal([]byte(resp), &v)
		return jsonResponse(200, v)
	})
}

func TestQueryWire(t *testing.T) {
	var seen []seenReq
	c := queryClient(t, &seen, `{"data":[{"id":"a"}],"meta":{"rawQuery":{"intent":"select","rowCount":1}}}`)
	ctx := context.Background()

	rows, meta, kerr := c.Query(ctx, "SELECT $1", []any{"x"})
	if kerr != nil {
		t.Fatal(kerr)
	}
	if len(rows) != 1 || rows[0]["id"] != "a" || meta.RawQuery == nil || meta.RawQuery.RowCount != 1 || meta.RawQuery.Intent != "select" {
		t.Fatalf("rows=%v meta=%+v", rows, meta)
	}
	if seen[0].method != "POST" || seen[0].path != "/@portal/query" || seen[0].body["sql"] != "SELECT $1" || !reflect.DeepEqual(seen[0].body["params"], []any{"x"}) {
		t.Fatalf("positional: %+v", seen[0])
	}

	// vector 11: no params, no key
	if _, _, kerr := c.Query(ctx, "SELECT 1", nil); kerr != nil {
		t.Fatal(kerr)
	}
	if _, has := seen[1].body["params"]; has {
		t.Fatalf("params key must be absent: %+v", seen[1].body)
	}

	if _, _, kerr := c.QueryNamed(ctx, "SELECT :a, :b, :a", map[string]any{"a": 1, "b": "two"}); kerr != nil {
		t.Fatal(kerr)
	}
	if seen[2].body["sql"] != "SELECT $1, $2, $1" || !reflect.DeepEqual(seen[2].body["params"], []any{float64(1), "two"}) {
		t.Fatalf("named: %+v", seen[2])
	}
}

func TestQueryValidationSendsNothing(t *testing.T) {
	var seen []seenReq
	c := queryClient(t, &seen, `{"data":[],"meta":{}}`)
	ctx := context.Background()
	if _, _, kerr := c.QueryNamed(ctx, "SELECT :missing", map[string]any{}); kerr == nil || kerr.Code != "K9O_INVALID_FIELD_VALUE" {
		t.Fatalf("missing: %v", kerr)
	}
	if _, _, kerr := c.Query(ctx, "SELECT $1", []any{map[string]any{"a": 1}}); kerr == nil || kerr.Code != "K9O_INVALID_FIELD_VALUE" {
		t.Fatalf("object: %v", kerr)
	}
	if len(seen) != 0 {
		t.Fatalf("requests sent: %d", len(seen))
	}
}

func TestSearchWire(t *testing.T) {
	var seen []seenReq
	c := queryClient(t, &seen, `{"data":[],"meta":{}}`)
	ctx := context.Background()

	if _, _, kerr := c.Search(ctx, "posts", SearchInput{Column: "body", Q: "red shoes", Language: "french", Type: "websearch", Limit: 3, Cursor: "cur_1"}); kerr != nil {
		t.Fatal(kerr)
	}
	want := map[string]string{"textSearch": "body:red shoes", "textSearchLanguage": "french", "textSearchType": "websearch", "limit": "3", "cursor": "cur_1"}
	if seen[0].method != "GET" || seen[0].path != "/@portal/data/posts" || !reflect.DeepEqual(seen[0].query, want) {
		t.Fatalf("search: %+v", seen[0])
	}

	// defaults are omitted
	if _, _, kerr := c.Search(ctx, "posts", SearchInput{Column: "body", Q: "x", Language: "english", Type: "plain"}); kerr != nil {
		t.Fatal(kerr)
	}
	if !reflect.DeepEqual(seen[1].query, map[string]string{"textSearch": "body:x"}) {
		t.Fatalf("defaults: %+v", seen[1].query)
	}

	// vector search
	b := c.From("posts").Select().VectorSearch("embedding", []float64{0.5, -1, 2}, VectorSearchOptions{Metric: "dot", Limit: 7})
	if _, _, kerr := SelectExecute[map[string]any](ctx, b); kerr != nil {
		t.Fatal(kerr)
	}
	want = map[string]string{"vectorSearch": "embedding:[0.5,-1,2]", "vectorMetric": "dot", "vectorLimit": "7"}
	if !reflect.DeepEqual(seen[2].query, want) {
		t.Fatalf("vector: %+v", seen[2].query)
	}
	b = c.From("posts").Select().VectorSearch("embedding", []float64{1}, VectorSearchOptions{})
	if _, _, kerr := SelectExecute[map[string]any](ctx, b); kerr != nil {
		t.Fatal(kerr)
	}
	if !reflect.DeepEqual(seen[3].query, map[string]string{"vectorSearch": "embedding:[1]"}) {
		t.Fatalf("vector defaults: %+v", seen[3].query)
	}
}

func TestSearchValidation(t *testing.T) {
	var seen []seenReq
	c := queryClient(t, &seen, `{"data":[],"meta":{}}`)
	ctx := context.Background()
	bad := []*SelectBuilder{
		c.From("posts").Select().TextSearch("bad col", "x", TextSearchOptions{}),
		c.From("posts").Select().TextSearch("body", "", TextSearchOptions{}),
		c.From("posts").Select().TextSearch("body", "x", TextSearchOptions{Language: "en glish"}),
		c.From("posts").Select().TextSearch("body", "x", TextSearchOptions{Type: "regex"}),
		c.From("posts").Select().VectorSearch("1bad", []float64{1}, VectorSearchOptions{}),
		c.From("posts").Select().VectorSearch("e", nil, VectorSearchOptions{}),
		c.From("posts").Select().VectorSearch("e", []float64{1}, VectorSearchOptions{Metric: "manhattan"}),
		c.From("posts").Select().VectorSearch("e", []float64{1}, VectorSearchOptions{Limit: 1001}),
		c.From("posts").Select().VectorSearch("e", []float64{1}, VectorSearchOptions{Limit: -1}),
	}
	for i, b := range bad {
		if _, _, kerr := SelectExecute[map[string]any](ctx, b); kerr == nil || kerr.Code != "K9O_INVALID_FIELD_VALUE" {
			t.Fatalf("case %d: %v", i, kerr)
		}
	}
	if _, _, kerr := c.Search(ctx, "posts", SearchInput{Q: "x"}); kerr == nil || kerr.Code != "K9O_INVALID_FIELD_VALUE" {
		t.Fatalf("missing column: %v", kerr)
	}
	if len(seen) != 0 {
		t.Fatalf("requests sent: %d", len(seen))
	}
}
