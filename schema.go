package k9o

import (
	"context"
	"strconv"
)

// CollectionSummary is one collection (table) of the project's data.
type CollectionSummary struct {
	Name string `json:"name"`
	// RowCount is a live count excluding soft-deleted rows. It ignores row-level security.
	RowCount               int64   `json:"rowCount"`
	RowCountIsExact        bool    `json:"rowCountIsExact"`
	ColumnCount            int     `json:"columnCount"`
	IndexCount             int     `json:"indexCount"`
	LastMigrationSlug      *string `json:"lastMigrationSlug"`
	LastMigrationAppliedAt *string `json:"lastMigrationAppliedAt"`
}

// ListCollectionsOptions pages Schema.Collections.List. A zero Limit uses the
// gateway default (50).
type ListCollectionsOptions struct {
	// Limit is 1 to 200.
	Limit int
	// Cursor is Meta.Cursor.Next of the previous page.
	Cursor string
}

// SchemaNamespace exposes client.Schema.*.
type SchemaNamespace struct {
	Collections *SchemaCollectionsNamespace
}

// SchemaCollectionsNamespace exposes client.Schema.Collections.*.
type SchemaCollectionsNamespace struct {
	config *httpClientConfig
}

func newSchemaNamespace(cfg *httpClientConfig) *SchemaNamespace {
	return &SchemaNamespace{Collections: &SchemaCollectionsNamespace{config: cfg}}
}

// List lists the project's collections by name, one page at a time:
// GET /@<slug>/schema/collections. Pass Meta.Cursor.Next of one page as Cursor
// of the next, until it is nil.
func (n *SchemaCollectionsNamespace) List(ctx context.Context, opts ListCollectionsOptions) ([]CollectionSummary, *Meta, *KError) {
	if kerr := checkLimit(opts.Limit); kerr != nil {
		return nil, nil, kerr
	}
	slug, kerr := requireProjectSlug(n.config, "schema")
	if kerr != nil {
		return nil, nil, kerr
	}
	q := map[string]string{}
	if opts.Limit != 0 {
		q["limit"] = strconv.Itoa(opts.Limit)
	}
	if opts.Cursor != "" {
		q["cursor"] = opts.Cursor
	}
	env, kerr := doRequest[[]CollectionSummary](ctx, n.config, &requestInit{Method: "GET", Path: ProjectPath(slug, "schema/collections"), Query: q})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}
