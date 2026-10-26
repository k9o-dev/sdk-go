package k9o

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type stCap struct {
	method, url, ctype string
	body               []byte
	calls              int
}

func stClient(t *testing.T, status int, resp any) (*Client, *stCap) {
	t.Helper()
	cp := &stCap{}
	c := p6Client(t, func(req *http.Request) *http.Response {
		cp.calls++
		cp.method, cp.url, cp.ctype = req.Method, req.URL.String(), req.Header.Get("Content-Type")
		if req.Body != nil {
			cp.body, _ = io.ReadAll(req.Body)
		}
		return jsonResponse(status, resp)
	})
	return c, cp
}

const stBase = "https://acme.api.k9o.io/@portal/storage/buckets"

var stBucketResp = map[string]any{"data": map[string]any{"name": "avatars", "public": false, "fileSizeLimit": nil, "allowedMimeTypes": []string{}}, "meta": map[string]any{}}
var stEmpty = map[string]any{"data": map[string]any{}, "meta": map[string]any{}}

func TestStorageBucketRoutes(t *testing.T) {
	ctx := context.Background()
	c, cp := stClient(t, 200, map[string]any{"data": []any{}, "meta": map[string]any{}})
	if _, _, k := c.Storage.ListBuckets(ctx); k != nil || cp.method != "GET" || cp.url != stBase {
		t.Fatalf("list: %v %s %s", k, cp.method, cp.url)
	}
	c, cp = stClient(t, 200, stBucketResp)
	b, _, k := c.Storage.GetBucket(ctx, "my bucket/x")
	if k != nil || cp.method != "GET" || cp.url != stBase+"/my%20bucket%2Fx" || b.Name != "avatars" || b.FileSizeLimit != nil {
		t.Fatalf("get: %v %s %s %+v", k, cp.method, cp.url, b)
	}
	pub, lim := true, int64(1024)
	if _, _, k = c.Storage.CreateBucket(ctx, "avatars", &CreateBucketOptions{Public: &pub, FileSizeLimit: &lim}); k != nil {
		t.Fatal(k)
	}
	if cp.method != "POST" || cp.url != stBase || cp.ctype != "application/json" || string(cp.body) != `{"fileSizeLimit":1024,"name":"avatars","public":true}` {
		t.Fatalf("create: %s %s %s %s", cp.method, cp.url, cp.ctype, cp.body)
	}
	if _, _, k = c.Storage.CreateBucket(ctx, "avatars", nil); k != nil || string(cp.body) != `{"name":"avatars"}` {
		t.Fatalf("create nil opts: %v %s", k, cp.body)
	}
	if _, _, k = c.Storage.UpdateBucket(ctx, "avatars", BucketPatch{"public": true, "fileSizeLimit": nil}); k != nil {
		t.Fatal(k)
	}
	if cp.method != "PATCH" || cp.url != stBase+"/avatars" || string(cp.body) != `{"fileSizeLimit":null,"public":true}` {
		t.Fatalf("update: %s %s %s", cp.method, cp.url, cp.body)
	}
}

func TestStorageDeleteBucketEmptyQuery(t *testing.T) {
	ctx := context.Background()
	c, cp := stClient(t, 200, stEmpty)
	if _, _, k := c.Storage.DeleteBucket(ctx, "avatars", false); k != nil || cp.method != "DELETE" || cp.url != stBase+"/avatars" {
		t.Fatalf("empty=false: %v %s %s", k, cp.method, cp.url)
	}
	if _, _, k := c.Storage.DeleteBucket(ctx, "avatars", true); k != nil || cp.url != stBase+"/avatars?empty=true" {
		t.Fatalf("empty=true: %v %s", k, cp.url)
	}
}

func TestStorageUploadMultipart(t *testing.T) {
	c, cp := stClient(t, 200, stEmpty)
	_, _, k := c.Storage.From("avatars").Upload(context.Background(), "a/b/pic.txt", "héllo", &UploadOptions{ContentType: "text/plain", Upsert: true, Metadata: map[string]string{"k": "v"}})
	if k != nil {
		t.Fatal(k)
	}
	if cp.method != "POST" || cp.url != stBase+"/avatars/objects" {
		t.Fatalf("%s %s", cp.method, cp.url)
	}
	if strings.Contains(cp.ctype, "json") {
		t.Fatalf("content-type %q", cp.ctype)
	}
	mt, params, err := mime.ParseMediaType(cp.ctype)
	if err != nil || mt != "multipart/form-data" || params["boundary"] == "" {
		t.Fatalf("content-type %q", cp.ctype)
	}
	r := multipart.NewReader(strings.NewReader(string(cp.body)), params["boundary"])
	got := map[string]string{}
	var fileName, fileCT string
	for {
		p, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(p)
		got[p.FormName()] = string(b)
		if p.FormName() == "file" {
			fileName, fileCT = p.FileName(), p.Header.Get("Content-Type")
		}
	}
	want := map[string]string{"file": "héllo", "path": "a/b/pic.txt", "contentType": "text/plain", "upsert": "true", "metadata": `{"k":"v"}`}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("part %s = %q want %q (all %v)", k, got[k], v, got)
		}
	}
	if fileName != "pic.txt" || fileCT != "text/plain" {
		t.Fatalf("file part %q %q", fileName, fileCT)
	}
	// defaults: bytes, upsert=false, no contentType / metadata parts
	if _, _, k = c.Storage.From("avatars").Upload(context.Background(), "x.bin", []byte{0, 1, 2}, nil); k != nil {
		t.Fatal(k)
	}
	s := string(cp.body)
	if !strings.Contains(s, "\r\n\r\nfalse\r\n") || strings.Contains(s, `name="contentType"`) || strings.Contains(s, `name="metadata"`) || !strings.Contains(s, "\x00\x01\x02") {
		t.Fatalf("defaults body: %q", s)
	}
	// io.Reader
	if _, _, k = c.Storage.From("avatars").Upload(context.Background(), "r.txt", strings.NewReader("stream"), nil); k != nil || !strings.Contains(string(cp.body), "stream") {
		t.Fatalf("reader: %v", k)
	}
}

func TestStorageSignedURLExpiresIn(t *testing.T) {
	ctx := context.Background()
	c, cp := stClient(t, 200, map[string]any{"data": map[string]any{"signedUrl": "https://x", "expiresIn": 60}, "meta": map[string]any{}})
	h := c.Storage.From("avatars")
	for _, bad := range []int{0, -1, 604801} {
		cp.calls = 0
		_, _, k := h.CreateSignedURL(ctx, "a.png", SignedURLOptions{ExpiresIn: bad})
		if k == nil || k.Code != "K9O_INVALID_FIELD_VALUE" || cp.calls != 0 {
			t.Fatalf("expiresIn %d: %v calls=%d", bad, k, cp.calls)
		}
	}
	for _, ok := range []int{1, 604800} {
		if _, _, k := h.CreateSignedURL(ctx, "a.png", SignedURLOptions{ExpiresIn: ok}); k != nil {
			t.Fatalf("expiresIn %d: %v", ok, k)
		}
	}
	dl := true
	s, _, k := h.CreateSignedURL(ctx, "a b.png", SignedURLOptions{ExpiresIn: 60, Download: &dl})
	if k != nil || s.SignedURL != "https://x" {
		t.Fatal(k)
	}
	if cp.method != "POST" || cp.url != stBase+"/avatars/objects/sign" || string(cp.body) != `{"download":true,"expiresIn":60,"path":"a b.png"}` {
		t.Fatalf("%s %s %s", cp.method, cp.url, cp.body)
	}
	if _, _, k = h.CreateSignedURL(ctx, "a.png", SignedURLOptions{ExpiresIn: 5}); k != nil || strings.Contains(string(cp.body), "download") {
		t.Fatalf("download must be omitted: %s", cp.body)
	}
}

func TestStorageListRemoveEncoding(t *testing.T) {
	ctx := context.Background()
	c, cp := stClient(t, 200, map[string]any{"data": []any{}, "meta": map[string]any{}})
	h := c.Storage.From("avatars")
	if _, _, k := h.List(ctx, ""); k != nil || cp.url != stBase+"/avatars/objects" {
		t.Fatalf("list no prefix: %v %s", k, cp.url)
	}
	if _, _, k := h.List(ctx, "a b&c/"); k != nil {
		t.Fatal(k)
	}
	u, _ := url.Parse(cp.url)
	if u.Path != "/@portal/storage/buckets/avatars/objects" || u.Query().Get("prefix") != "a b&c/" || strings.Contains(u.RawQuery, " ") || strings.Count(u.RawQuery, "&") != 0 {
		t.Fatalf("list url %s", cp.url)
	}
	c, cp = stClient(t, 200, stEmpty)
	if _, _, k := c.Storage.From("avatars").Remove(ctx, "../x y&z.txt"); k != nil || cp.method != "DELETE" {
		t.Fatalf("remove: %v", k)
	}
	u, _ = url.Parse(cp.url)
	if u.Query().Get("path") != "../x y&z.txt" || strings.Count(u.RawQuery, "&") != 0 || !strings.HasPrefix(u.RawQuery, "path=") {
		t.Fatalf("remove url %s", cp.url)
	}
	// bucket name with ".." is encoded in the path, not rewritten
	if _, _, k := c.Storage.From("a/../b").Remove(ctx, "/lead"); k != nil || !strings.Contains(cp.url, "/buckets/a%2F..%2Fb/objects?path=%2Flead") {
		t.Fatalf("encoded: %v %s", k, cp.url)
	}
}

func TestStorageEmptyNamesAndMissingSlug(t *testing.T) {
	ctx := context.Background()
	c, cp := stClient(t, 200, stEmpty)
	if _, _, k := c.Storage.GetBucket(ctx, ""); k == nil || k.Code != "K9O_INVALID_FIELD_VALUE" {
		t.Fatalf("empty bucket: %v", k)
	}
	h := c.Storage.From("avatars")
	if _, _, k := h.Remove(ctx, ""); k == nil || k.Code != "K9O_INVALID_FIELD_VALUE" {
		t.Fatal(k)
	}
	if _, _, k := h.Upload(ctx, "", "x", nil); k == nil || k.Code != "K9O_INVALID_FIELD_VALUE" {
		t.Fatal(k)
	}
	if _, _, k := h.Upload(ctx, "p", 42, nil); k == nil || k.Code != "K9O_INVALID_FIELD_VALUE" {
		t.Fatal(k)
	}
	if cp.calls != 0 {
		t.Fatalf("requests sent: %d", cp.calls)
	}
	calls := 0
	nc, kerr := NewClient(ClientOpts{BaseURL: "https://acme.api.k9o.io", HTTPClient: &http.Client{Transport: noProbe(roundTripFunc(func(*http.Request) *http.Response { calls++; return jsonResponse(200, stEmpty) }))}})
	if kerr != nil {
		t.Fatal(kerr)
	}
	checks := []*KError{}
	_, _, k := nc.Storage.ListBuckets(ctx)
	checks = append(checks, k)
	_, _, k = nc.Storage.CreateBucket(ctx, "abc", nil)
	checks = append(checks, k)
	_, _, k = nc.Storage.From("b").Upload(ctx, "p", "x", nil)
	checks = append(checks, k)
	_, _, k = nc.Storage.From("b").CreateSignedURL(ctx, "p", SignedURLOptions{ExpiresIn: 5})
	checks = append(checks, k)
	for i, k := range checks {
		if k == nil || k.Code != "K9O_MISSING_PROJECT_CONTEXT" {
			t.Fatalf("check %d: %v", i, k)
		}
	}
	if calls != 0 {
		t.Fatalf("requests sent: %d", calls)
	}
}

func TestStoragePassthroughErrors(t *testing.T) {
	ctx := context.Background()
	c, _ := stClient(t, 409, map[string]any{"code": "K9O_ALREADY_EXISTS", "message": "bucket_not_empty", "context": map[string]any{"reason": "bucket_not_empty"}})
	_, _, k := c.Storage.DeleteBucket(ctx, "avatars", false)
	if k == nil || k.Status != 409 || k.Code != "K9O_ALREADY_EXISTS" || k.Message != "bucket_not_empty" {
		t.Fatalf("409: %+v", k)
	}
	c, _ = stClient(t, 400, map[string]any{"code": "K9O_INVALID_BUCKET_NAME", "message": "bad name"})
	_, _, k = c.Storage.CreateBucket(ctx, "BAD", nil)
	if k == nil || k.Code != "K9O_INVALID_BUCKET_NAME" || k.Status != 400 {
		t.Fatalf("400: %+v", k)
	}
	c, _ = stClient(t, 409, map[string]any{"code": "K9O_ALREADY_EXISTS", "message": "exists"})
	_, _, k = c.Storage.From("avatars").Upload(ctx, "p.txt", "SECRET-OBJECT-BYTES", nil)
	if k == nil || k.Status != 409 || strings.Contains(k.Error(), "SECRET-OBJECT-BYTES") || strings.Contains(k.Message, "SECRET") {
		t.Fatalf("upload 409: %+v", k)
	}
}

func TestStorageDotBucketNamesRefusedBeforeRequest(t *testing.T) {
	ctx := context.Background()
	c, cp := stClient(t, 200, stEmpty)
	check := func(label string, k *KError) {
		t.Helper()
		if k == nil || k.Code != "K9O_INVALID_FIELD_VALUE" || k.Context["reason"] != "dot_segment" || k.Context["field"] != "bucket" {
			t.Fatalf("%s: %+v", label, k)
		}
	}
	for _, name := range []string{".", ".."} {
		_, _, k := c.Storage.GetBucket(ctx, name)
		check("GetBucket "+name, k)
		_, _, k = c.Storage.UpdateBucket(ctx, name, BucketPatch{})
		check("UpdateBucket "+name, k)
		_, _, k = c.Storage.DeleteBucket(ctx, name, false)
		check("DeleteBucket "+name, k)
		h := c.Storage.From(name)
		_, _, k = h.Upload(ctx, "p", "x", nil)
		check("Upload "+name, k)
		_, _, k = h.CreateSignedURL(ctx, "p", SignedURLOptions{ExpiresIn: 5})
		check("CreateSignedURL "+name, k)
		_, _, k = h.List(ctx, "")
		check("List "+name, k)
		_, _, k = h.Remove(ctx, "p")
		check("Remove "+name, k)
	}
	if cp.calls != 0 {
		t.Fatalf("requests sent: %d", cp.calls)
	}
	if _, _, k := c.Storage.GetBucket(ctx, "avatars"); k != nil || cp.calls != 1 {
		t.Fatalf("positive control: %v calls=%d", k, cp.calls)
	}
}
