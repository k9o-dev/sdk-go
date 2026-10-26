package k9o

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var collectionNameRegex = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,62}$`)

// FromBuilder is the entry point for the data-plane DSL.
// Returned by Client.From(collection).
type FromBuilder struct {
	config     *httpClientConfig
	collection string
}

// Select initiates a SELECT query. Returns a SelectBuilder for chaining
// filters; terminal via SelectExecute (package-level generic helper).
func (f *FromBuilder) Select() *SelectBuilder {
	return &SelectBuilder{config: f.config, collection: f.collection}
}

// Insert posts a record and returns the inserted record envelope.
// Generic over the per-collection row shape via the typed wrapper
// InsertExecute. The non-generic Insert method here returns any-typed
// data for callers who don't want to specialize.
func (f *FromBuilder) Insert(ctx context.Context, record any) (*Envelope[any], *KError) {
	path, kerr := collectionPath(f.config, f.collection)
	if kerr != nil {
		return nil, kerr
	}
	return doRequest[any](ctx, f.config, &requestInit{
		Method: "POST",
		Path:   path,
		Body:   record,
	})
}

// Update initiates an UPDATE. Returns an UpdateBuilder; the chain MUST
// include .Eq("id", "rec_...") before .Execute().
func (f *FromBuilder) Update(patch any) *UpdateBuilder {
	return &UpdateBuilder{config: f.config, collection: f.collection, patch: patch}
}

// Delete initiates a DELETE. Returns a DeleteBuilder; the chain MUST
// include .Eq("id", "rec_...") before .Execute().
func (f *FromBuilder) Delete() *DeleteBuilder {
	return &DeleteBuilder{config: f.config, collection: f.collection}
}

// filter is one filter clause attached to a query builder.
type filter struct {
	column string
	op     string
	value  any
}

// queryBuilderBase carries shared filter/page state.
type queryBuilderBase struct {
	filters   []filter
	limitVal  *int
	cursorVal *string
	countMode *string
}

func (q *queryBuilderBase) addFilter(column, op string, value any) {
	q.filters = append(q.filters, filter{column: column, op: op, value: value})
}

func (q *queryBuilderBase) toQuery() map[string]string {
	out := map[string]string{}
	for _, f := range q.filters {
		switch v := f.value.(type) {
		case []any:
			parts := make([]string, len(v))
			for i, x := range v {
				parts[i] = fmt.Sprintf("%v", x)
			}
			out[f.column] = fmt.Sprintf("%s.(%s)", f.op, strings.Join(parts, ","))
		case []string:
			out[f.column] = fmt.Sprintf("%s.(%s)", f.op, strings.Join(v, ","))
		default:
			out[f.column] = fmt.Sprintf("%s.%v", f.op, v)
		}
	}
	if q.limitVal != nil {
		out["limit"] = fmt.Sprintf("%d", *q.limitVal)
	}
	if q.cursorVal != nil {
		out["cursor"] = *q.cursorVal
	}
	if q.countMode != nil {
		out["count"] = *q.countMode
	}
	return out
}

// SelectBuilder is the chainable SELECT query builder.
type SelectBuilder struct {
	queryBuilderBase
	config      *httpClientConfig
	collection  string
	searchQuery map[string]string // textSearch / vectorSearch wire params
	searchErr   *KError           // first invalid search argument, reported at execute
}

func (s *SelectBuilder) Eq(column string, value any) *SelectBuilder {
	s.addFilter(column, "eq", value)
	return s
}
func (s *SelectBuilder) Neq(column string, value any) *SelectBuilder {
	s.addFilter(column, "neq", value)
	return s
}
func (s *SelectBuilder) Gt(column string, value any) *SelectBuilder {
	s.addFilter(column, "gt", value)
	return s
}
func (s *SelectBuilder) Gte(column string, value any) *SelectBuilder {
	s.addFilter(column, "gte", value)
	return s
}
func (s *SelectBuilder) Lt(column string, value any) *SelectBuilder {
	s.addFilter(column, "lt", value)
	return s
}
func (s *SelectBuilder) Lte(column string, value any) *SelectBuilder {
	s.addFilter(column, "lte", value)
	return s
}
func (s *SelectBuilder) In(column string, values []any) *SelectBuilder {
	s.addFilter(column, "in", values)
	return s
}
func (s *SelectBuilder) NotIn(column string, values []any) *SelectBuilder {
	s.addFilter(column, "notIn", values)
	return s
}
func (s *SelectBuilder) Like(column string, pattern string) *SelectBuilder {
	s.addFilter(column, "like", pattern)
	return s
}
func (s *SelectBuilder) ILike(column string, pattern string) *SelectBuilder {
	s.addFilter(column, "ilike", pattern)
	return s
}
func (s *SelectBuilder) Is(column string, value string) *SelectBuilder {
	s.addFilter(column, "is", value)
	return s
}
func (s *SelectBuilder) Filter(column, op string, value any) *SelectBuilder {
	s.addFilter(column, op, value)
	return s
}
func (s *SelectBuilder) Limit(n int) *SelectBuilder {
	s.limitVal = &n
	return s
}
func (s *SelectBuilder) Cursor(opaque string) *SelectBuilder {
	s.cursorVal = &opaque
	return s
}
func (s *SelectBuilder) Count(mode string) *SelectBuilder {
	s.countMode = &mode
	return s
}

// SelectExecute is the generic terminal-verb function for SelectBuilder.
// Go's method receivers cannot have additional type parameters; we use
// a package-level function so callers can specify the row type:
//
//	rows, meta, kerr := k9o.SelectExecute[Post](ctx, client.From("posts").Select())
func SelectExecute[T any](ctx context.Context, s *SelectBuilder) ([]T, *Meta, *KError) {
	if s.searchErr != nil {
		return nil, nil, s.searchErr
	}
	path, kerr := collectionPath(s.config, s.collection)
	if kerr != nil {
		return nil, nil, kerr
	}
	query := s.toQuery()
	for k, v := range s.searchQuery {
		query[k] = v
	}
	env, kerr := doRequest[[]T](ctx, s.config, &requestInit{
		Method: "GET",
		Path:   path,
		Query:  query,
	})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// UpdateBuilder is the chainable UPDATE query builder.
type UpdateBuilder struct {
	queryBuilderBase
	config     *httpClientConfig
	collection string
	patch      any
}

func (u *UpdateBuilder) Eq(column string, value any) *UpdateBuilder {
	u.addFilter(column, "eq", value)
	return u
}

func (u *UpdateBuilder) Execute(ctx context.Context) (*Envelope[any], *KError) {
	var idVal string
	for _, f := range u.filters {
		if f.column == "id" && f.op == "eq" {
			idVal = fmt.Sprintf("%v", f.value)
			break
		}
	}
	if idVal == "" {
		return nil, &KError{
			Code:    "K9O_INVALID_FIELD_VALUE",
			Message: "Update() requires .Eq(\"id\", ...) filter; changing many rows in one call is not supported",
			Context: map[string]any{"collection": u.collection},
		}
	}
	path, kerr := collectionPath(u.config, u.collection)
	if kerr != nil {
		return nil, kerr
	}
	return doRequest[any](ctx, u.config, &requestInit{
		Method: "PATCH",
		Path:   path + "/" + url.PathEscape(idVal),
		Body:   u.patch,
	})
}

// DeleteBuilder is the chainable DELETE query builder.
type DeleteBuilder struct {
	queryBuilderBase
	config     *httpClientConfig
	collection string
}

func (d *DeleteBuilder) Eq(column string, value any) *DeleteBuilder {
	d.addFilter(column, "eq", value)
	return d
}

func (d *DeleteBuilder) Execute(ctx context.Context) (*Envelope[any], *KError) {
	var idVal string
	for _, f := range d.filters {
		if f.column == "id" && f.op == "eq" {
			idVal = fmt.Sprintf("%v", f.value)
			break
		}
	}
	if idVal == "" {
		return nil, &KError{
			Code:    "K9O_INVALID_FIELD_VALUE",
			Message: "Delete() requires .Eq(\"id\", ...) filter; deleting many rows in one call is not supported",
			Context: map[string]any{"collection": d.collection},
		}
	}
	path, kerr := collectionPath(d.config, d.collection)
	if kerr != nil {
		return nil, kerr
	}
	return doRequest[any](ctx, d.config, &requestInit{
		Method: "DELETE",
		Path:   path + "/" + url.PathEscape(idVal),
	})
}

func collectionPath(cfg *httpClientConfig, collection string) (string, *KError) {
	if cfg.ProjectSlug == "" {
		return "", &KError{
			Code:    "K9O_MISSING_PROJECT_CONTEXT",
			Message: "data-plane operations require ProjectSlug bound on the client",
			Context: map[string]any{"collection": collection},
		}
	}
	if !collectionNameRegex.MatchString(collection) {
		return "", &KError{
			Code:    "K9O_INVALID_FIELD_VALUE",
			Message: fmt.Sprintf("collection name %q does not match required format", collection),
			Context: map[string]any{"collection": collection},
		}
	}
	return ProjectPath(cfg.ProjectSlug, "data/"+collection), nil
}
