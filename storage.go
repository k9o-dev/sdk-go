package k9o

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"net/url"
	"strings"
)

// maxSignedURLExpiresIn is the longest signed-URL lifetime in seconds (7 days).
const maxSignedURLExpiresIn = 604800

// StorageBucket is a per-project logical bucket.
type StorageBucket struct {
	Name             string   `json:"name"`
	Public           bool     `json:"public"`
	FileSizeLimit    *int64   `json:"fileSizeLimit"` // bytes; nil = no limit
	AllowedMimeTypes []string `json:"allowedMimeTypes"`
	Versioning       bool     `json:"versioning"`
	DeleteAfterDays  *int     `json:"deleteAfterDays"` // nil = never
	CreatedAt        string   `json:"createdAt"`
	UpdatedAt        string   `json:"updatedAt"`
	System           bool     `json:"system"`
	ByosBindingID    *string  `json:"byosBindingId"` // nil = platform-managed storage
}

// CreateBucketOptions are the optional fields of CreateBucket. Nil / zero
// fields are omitted from the request body.
type CreateBucketOptions struct {
	Public           *bool    `json:"public,omitempty"`
	FileSizeLimit    *int64   `json:"fileSizeLimit,omitempty"`
	AllowedMimeTypes []string `json:"allowedMimeTypes,omitempty"`
	Versioning       *bool    `json:"versioning,omitempty"`
	DeleteAfterDays  *int     `json:"deleteAfterDays,omitempty"`
	ByosBindingID    *string  `json:"byosBindingId,omitempty"`
}

// BucketPatch is the body of UpdateBucket: only the keys present are sent.
// A nil value clears a limit (fileSizeLimit, deleteAfterDays, byosBindingId,
// allowedMimeTypes). Keys: public, fileSizeLimit, allowedMimeTypes,
// versioning, deleteAfterDays, byosBindingId.
type BucketPatch map[string]any

// StorageObject is an object listing entry; Path is relative to the bucket.
type StorageObject struct {
	Path         string  `json:"path"`
	Size         int64   `json:"size"`
	LastModified *string `json:"lastModified"`
}

// UploadOptions are the optional fields of Upload.
type UploadOptions struct {
	// ContentType of the object; empty = application/octet-stream.
	ContentType string
	// Upsert false (default) refuses to overwrite an existing path (409).
	Upsert bool
	// Metadata is caller-defined string metadata (limits enforced by the gateway).
	Metadata map[string]string
}

// SignedURLOptions are the options of CreateSignedURL.
type SignedURLOptions struct {
	// ExpiresIn is REQUIRED, in seconds: 1 to 604800. There is no default.
	ExpiresIn int
	// Download makes the download an attachment.
	Download *bool
}

// SignedURL is a pre-authenticated GET URL.
type SignedURL struct {
	SignedURL string `json:"signedUrl"`
	ExpiresIn int    `json:"expiresIn"`
}

// RemovedObject is the result of removing an object.
type RemovedObject struct {
	Bucket  string `json:"bucket"`
	Path    string `json:"path"`
	Deleted bool   `json:"deleted"`
}

// DeletedBucket is the result of deleting a bucket.
type DeletedBucket struct {
	Name           string `json:"name"`
	Deleted        bool   `json:"deleted"`
	ObjectsDeleted int    `json:"objectsDeleted"`
}

// StorageNamespace exposes client.Storage.* (buckets and objects). Signed PUT
// and copy are deliberately absent: the gateway has no routes for them.
type StorageNamespace struct {
	config *httpClientConfig
}

func invalidStorageField(field, reason, msg string) *KError {
	return &KError{
		Code:    "K9O_INVALID_FIELD_VALUE",
		Message: msg,
		Context: map[string]any{"field": field, "reason": reason},
	}
}

func (n *StorageNamespace) bucketsPath(suffix string) (string, *KError) {
	slug, kerr := requireProjectSlug(n.config, "storage buckets and objects")
	if kerr != nil {
		return "", kerr
	}
	return ProjectPath(slug, "storage/buckets"+suffix), nil
}

func (n *StorageNamespace) bucketPath(name, suffix string) (string, *KError) {
	if _, kerr := requireProjectSlug(n.config, "storage buckets and objects"); kerr != nil {
		return "", kerr
	}
	if name == "" {
		return "", invalidStorageField("bucket", "empty", "bucket name must not be empty")
	}
	// "." and ".." survive percent-encoding and are collapsed as dot segments
	// by URL layers, addressing a different route. No bucket can have these names.
	if name == "." || name == ".." {
		return "", invalidStorageField("bucket", "dot_segment", "bucket name must not be \".\" or \"..\"")
	}
	return n.bucketsPath("/" + url.PathEscape(name) + suffix)
}

// ListBuckets lists the project's buckets: GET /@<slug>/storage/buckets.
func (n *StorageNamespace) ListBuckets(ctx context.Context) ([]StorageBucket, *Meta, *KError) {
	p, kerr := n.bucketsPath("")
	if kerr != nil {
		return nil, nil, kerr
	}
	return storageCall[[]StorageBucket](ctx, n.config, &requestInit{Method: "GET", Path: p})
}

// GetBucket reads one bucket: GET .../buckets/<name>.
func (n *StorageNamespace) GetBucket(ctx context.Context, name string) (*StorageBucket, *Meta, *KError) {
	p, kerr := n.bucketPath(name, "")
	if kerr != nil {
		return nil, nil, kerr
	}
	return storageCallPtr[StorageBucket](ctx, n.config, &requestInit{Method: "GET", Path: p})
}

// CreateBucket creates a bucket: POST .../buckets with {name, ...opts}.
// Pass nil opts for defaults.
func (n *StorageNamespace) CreateBucket(ctx context.Context, name string, opts *CreateBucketOptions) (*StorageBucket, *Meta, *KError) {
	p, kerr := n.bucketsPath("")
	if kerr != nil {
		return nil, nil, kerr
	}
	body := map[string]any{}
	if opts != nil {
		raw, err := jsonMarshalToMap(opts)
		if err != nil {
			return nil, nil, &KError{Code: "K9O_INTERNAL_ERROR", Message: fmt.Sprintf("failed to marshal bucket options: %v", err)}
		}
		body = raw
	}
	body["name"] = name
	return storageCallPtr[StorageBucket](ctx, n.config, &requestInit{Method: "POST", Path: p, Body: body})
}

// UpdateBucket changes a bucket: PATCH .../buckets/<name>.
func (n *StorageNamespace) UpdateBucket(ctx context.Context, name string, patch BucketPatch) (*StorageBucket, *Meta, *KError) {
	p, kerr := n.bucketPath(name, "")
	if kerr != nil {
		return nil, nil, kerr
	}
	body := map[string]any(patch)
	if body == nil {
		body = map[string]any{}
	}
	return storageCallPtr[StorageBucket](ctx, n.config, &requestInit{Method: "PATCH", Path: p, Body: body})
}

// DeleteBucket deletes a bucket: DELETE .../buckets/<name>. A non-empty bucket
// is refused (409 bucket_not_empty) unless empty is true, which sends
// ?empty=true and purges its objects first.
func (n *StorageNamespace) DeleteBucket(ctx context.Context, name string, empty bool) (*DeletedBucket, *Meta, *KError) {
	p, kerr := n.bucketPath(name, "")
	if kerr != nil {
		return nil, nil, kerr
	}
	init := &requestInit{Method: "DELETE", Path: p}
	if empty {
		init.Query = map[string]string{"empty": "true"}
	}
	return storageCallPtr[DeletedBucket](ctx, n.config, init)
}

// From returns a handle for object operations on one bucket.
func (n *StorageNamespace) From(bucket string) *StorageBucketHandle {
	return &StorageBucketHandle{ns: n, bucket: bucket}
}

// StorageBucketHandle performs object operations on one bucket.
type StorageBucketHandle struct {
	ns     *StorageNamespace
	bucket string
}

func requireObjectPath(path string) *KError {
	if path == "" {
		return invalidStorageField("path", "empty", "object path must not be empty")
	}
	return nil
}

// Upload uploads an object as multipart/form-data: POST .../objects with parts
// file, path, contentType (if set), upsert and metadata (if set). data may be
// a string (sent as UTF-8), []byte, or an io.Reader (read to the end). The
// path is sent as given; the server decides what is acceptable.
func (h *StorageBucketHandle) Upload(ctx context.Context, path string, data any, opts *UploadOptions) (*StorageObject, *Meta, *KError) {
	p, kerr := h.ns.bucketPath(h.bucket, "/objects")
	if kerr != nil {
		return nil, nil, kerr
	}
	if kerr := requireObjectPath(path); kerr != nil {
		return nil, nil, kerr
	}
	var payload []byte
	switch d := data.(type) {
	case string:
		payload = []byte(d)
	case []byte:
		payload = d
	case io.Reader:
		b, err := io.ReadAll(d)
		if err != nil {
			return nil, nil, &KError{Code: "K9O_INVALID_FIELD_VALUE", Message: fmt.Sprintf("failed to read upload data: %v", err), Context: map[string]any{"field": "data", "reason": "unreadable"}}
		}
		payload = b
	default:
		return nil, nil, invalidStorageField("data", "type", "upload data must be a string, []byte or io.Reader")
	}
	if opts == nil {
		opts = &UploadOptions{}
	}
	filename := "file"
	if i := strings.LastIndex(path, "/"); i >= 0 {
		path2 := path[i+1:]
		if path2 != "" {
			filename = path2
		}
	} else {
		filename = path
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\r", "", "\n", "").Replace(filename)))
	ct := opts.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	hdr.Set("Content-Type", ct)
	fw, err := w.CreatePart(hdr)
	if err == nil {
		_, err = fw.Write(payload)
	}
	fields := [][2]string{{"path", path}}
	if opts.ContentType != "" {
		fields = append(fields, [2]string{"contentType", opts.ContentType})
	}
	fields = append(fields, [2]string{"upsert", fmt.Sprintf("%t", opts.Upsert)})
	if opts.Metadata != nil {
		mb, merr := jsonMarshalBytes(opts.Metadata)
		if merr != nil && err == nil {
			err = merr
		}
		fields = append(fields, [2]string{"metadata", string(mb)})
	}
	for _, f := range fields {
		if err == nil {
			err = w.WriteField(f[0], f[1])
		}
	}
	if err == nil {
		err = w.Close()
	}
	if err != nil {
		return nil, nil, &KError{Code: "K9O_INTERNAL_ERROR", Message: fmt.Sprintf("failed to build multipart body: %v", err)}
	}
	return storageCallPtr[StorageObject](ctx, h.ns.config, &requestInit{Method: "POST", Path: p, RawBody: buf.Bytes(), RawContentType: w.FormDataContentType()})
}

// CreateSignedURL mints a pre-authenticated GET URL: POST .../objects/sign.
// opts.ExpiresIn is required (integer 1..604800) and is checked before any
// request. There is no default and no transform option.
func (h *StorageBucketHandle) CreateSignedURL(ctx context.Context, path string, opts SignedURLOptions) (*SignedURL, *Meta, *KError) {
	p, kerr := h.ns.bucketPath(h.bucket, "/objects/sign")
	if kerr != nil {
		return nil, nil, kerr
	}
	if kerr := requireObjectPath(path); kerr != nil {
		return nil, nil, kerr
	}
	if opts.ExpiresIn < 1 || opts.ExpiresIn > maxSignedURLExpiresIn {
		return nil, nil, &KError{
			Code:           "K9O_INVALID_FIELD_VALUE",
			Message:        fmt.Sprintf("ExpiresIn is required and must be an integer from 1 to %d seconds", maxSignedURLExpiresIn),
			Context:        map[string]any{"field": "expiresIn", "reason": "out_of_range"},
			Recommendation: "Pass an explicit ExpiresIn; signed URLs have no default lifetime.",
		}
	}
	body := map[string]any{"path": path, "expiresIn": opts.ExpiresIn}
	if opts.Download != nil {
		body["download"] = *opts.Download
	}
	return storageCallPtr[SignedURL](ctx, h.ns.config, &requestInit{Method: "POST", Path: p, Body: body})
}

// List lists objects, optionally under prefix (empty = no prefix filter):
// GET .../objects?prefix=.
func (h *StorageBucketHandle) List(ctx context.Context, prefix string) ([]StorageObject, *Meta, *KError) {
	p, kerr := h.ns.bucketPath(h.bucket, "/objects")
	if kerr != nil {
		return nil, nil, kerr
	}
	init := &requestInit{Method: "GET", Path: p}
	if prefix != "" {
		init.Query = map[string]string{"prefix": prefix}
	}
	return storageCall[[]StorageObject](ctx, h.ns.config, init)
}

// Remove deletes one object: DELETE .../objects?path=.
func (h *StorageBucketHandle) Remove(ctx context.Context, path string) (*RemovedObject, *Meta, *KError) {
	p, kerr := h.ns.bucketPath(h.bucket, "/objects")
	if kerr != nil {
		return nil, nil, kerr
	}
	if kerr := requireObjectPath(path); kerr != nil {
		return nil, nil, kerr
	}
	return storageCallPtr[RemovedObject](ctx, h.ns.config, &requestInit{Method: "DELETE", Path: p, Query: map[string]string{"path": path}})
}

func storageCall[T any](ctx context.Context, cfg *httpClientConfig, init *requestInit) (T, *Meta, *KError) {
	env, kerr := doRequest[T](ctx, cfg, init)
	if kerr != nil {
		var zero T
		return zero, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func storageCallPtr[T any](ctx context.Context, cfg *httpClientConfig, init *requestInit) (*T, *Meta, *KError) {
	env, kerr := doRequest[T](ctx, cfg, init)
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

func jsonMarshalBytes(v any) ([]byte, error) { return json.Marshal(v) }

func jsonMarshalToMap(v any) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	m := map[string]any{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}
