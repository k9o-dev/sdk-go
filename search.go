package k9o

import (
	"context"
	"encoding/json"
	"math"
	"regexp"
	"strconv"
)

var (
	searchColumnRegex   = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)
	searchLanguageRegex = regexp.MustCompile(`^[a-zA-Z_]+$`)
)

// TextSearchOptions tune SelectBuilder.TextSearch. Zero values mean the
// defaults: Language "english" and Type "plain".
type TextSearchOptions struct {
	// Language is a Postgres text-search configuration name.
	Language string
	// Type is "plain" (default), "phrase" or "websearch".
	Type string
}

// VectorSearchOptions tune SelectBuilder.VectorSearch. Zero values mean the
// defaults: Metric "cosine" and the gateway's default limit (10).
type VectorSearchOptions struct {
	// Metric is "cosine" (default), "euclidean" or "dot".
	Metric string
	// Limit is the number of rows to return, 1 to 1000.
	Limit int
}

// TextSearch adds full-text search over one text column. It composes with
// filters, Limit and Cursor. An invalid argument is reported by
// SelectExecute as K9O_INVALID_FIELD_VALUE, before any request.
func (s *SelectBuilder) TextSearch(column, query string, opts TextSearchOptions) *SelectBuilder {
	if !searchColumnRegex.MatchString(column) {
		return s.failSearch("textSearch column '" + column + "' is not a valid identifier")
	}
	if query == "" {
		return s.failSearch("textSearch query must be non-empty")
	}
	language := opts.Language
	if language == "" {
		language = "english"
	}
	if !searchLanguageRegex.MatchString(language) {
		return s.failSearch("textSearch language must be a text-search configuration name")
	}
	typ := opts.Type
	if typ == "" {
		typ = "plain"
	}
	if typ != "plain" && typ != "phrase" && typ != "websearch" {
		return s.failSearch("textSearch type must be plain, phrase or websearch")
	}
	s.ensureSearch()
	s.searchQuery["textSearch"] = column + ":" + query
	if language != "english" {
		s.searchQuery["textSearchLanguage"] = language
	} else {
		delete(s.searchQuery, "textSearchLanguage")
	}
	if typ != "plain" {
		s.searchQuery["textSearchType"] = typ
	} else {
		delete(s.searchQuery, "textSearchType")
	}
	return s
}

// VectorSearch adds semantic search: the rows nearest to an embedding in one
// vector column, closest first. An invalid argument is reported by
// SelectExecute as K9O_INVALID_FIELD_VALUE, before any request.
func (s *SelectBuilder) VectorSearch(column string, embedding []float64, opts VectorSearchOptions) *SelectBuilder {
	if !searchColumnRegex.MatchString(column) {
		return s.failSearch("vectorSearch column '" + column + "' is not a valid identifier")
	}
	if len(embedding) == 0 {
		return s.failSearch("vectorSearch embedding must be a non-empty array of numbers")
	}
	for _, n := range embedding {
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return s.failSearch("vectorSearch embedding must be a non-empty array of numbers")
		}
	}
	metric := opts.Metric
	if metric == "" {
		metric = "cosine"
	}
	if metric != "cosine" && metric != "euclidean" && metric != "dot" {
		return s.failSearch("vectorSearch metric must be cosine, euclidean or dot")
	}
	if opts.Limit != 0 && (opts.Limit < 1 || opts.Limit > 1000) {
		return s.failSearch("vectorSearch limit must be an integer from 1 to 1000")
	}
	encoded, _ := json.Marshal(embedding)
	s.ensureSearch()
	s.searchQuery["vectorSearch"] = column + ":" + string(encoded)
	if metric != "cosine" {
		s.searchQuery["vectorMetric"] = metric
	} else {
		delete(s.searchQuery, "vectorMetric")
	}
	if opts.Limit != 0 {
		s.searchQuery["vectorLimit"] = strconv.Itoa(opts.Limit)
	} else {
		delete(s.searchQuery, "vectorLimit")
	}
	return s
}

func (s *SelectBuilder) ensureSearch() {
	if s.searchQuery == nil {
		s.searchQuery = map[string]string{}
	}
}

func (s *SelectBuilder) failSearch(message string) *SelectBuilder {
	if s.searchErr == nil {
		s.searchErr = invalidFieldValue(message, nil)
	}
	return s
}

// SearchInput is the argument of Client.Search. Column and Q are required.
type SearchInput struct {
	// Column is the text column to search.
	Column string
	// Q is the search text.
	Q string
	// Language is a text-search configuration name (default "english").
	Language string
	// Type is "plain" (default), "phrase" or "websearch".
	Type string
	// Limit is the page size; 0 uses the gateway default.
	Limit int
	// Cursor is the opaque cursor from a previous response's Meta.Cursor.Next.
	Cursor string
}

// Search runs a full-text search over one text column of a collection, best
// matches first. It is a shortcut for
// From(collection).Select().TextSearch(...) with an optional Limit and Cursor.
func (c *Client) Search(ctx context.Context, collection string, input SearchInput) ([]map[string]any, *Meta, *KError) {
	return SearchInto[map[string]any](ctx, c, collection, input)
}

// SearchInto is Search with a caller-chosen row type.
func SearchInto[T any](ctx context.Context, c *Client, collection string, input SearchInput) ([]T, *Meta, *KError) {
	b := c.From(collection).Select().TextSearch(input.Column, input.Q, TextSearchOptions{Language: input.Language, Type: input.Type})
	if input.Limit != 0 {
		b = b.Limit(input.Limit)
	}
	if input.Cursor != "" {
		b = b.Cursor(input.Cursor)
	}
	return SelectExecute[T](ctx, b)
}
