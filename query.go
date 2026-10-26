package k9o

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// Query runs one read-only SQL statement against the project's data:
// POST /@<slug>/query. params is a positional list bound to $1, $2, ...; the
// SQL is sent as written. Use QueryNamed for :name placeholders.
//
// The gateway refuses writes (K9O_RAW_SQL_READ_ONLY), accepts one statement
// per call, applies row-level security, allows at most 100000 characters of
// SQL and 1000 parameters, and needs the project-editor role. A reference to
// dataset.<name> reads an analytics dataset. Values must be string, number,
// bool or nil; serialise anything else (for example as JSON text) first.
func (c *Client) Query(ctx context.Context, sql string, params []any) ([]map[string]any, *Meta, *KError) {
	return QueryInto[map[string]any](ctx, c, sql, params)
}

// QueryNamed is Query with named placeholders: each :name in the SQL is
// rewritten to $n and the map values become the positional list. A
// placeholder with no key fails with K9O_INVALID_FIELD_VALUE before any
// request; extra keys are ignored.
func (c *Client) QueryNamed(ctx context.Context, sql string, params map[string]any) ([]map[string]any, *Meta, *KError) {
	return QueryNamedInto[map[string]any](ctx, c, sql, params)
}

// QueryInto is Query with a caller-chosen row type (Go methods cannot take
// type parameters).
//
//	rows, meta, kerr := k9o.QueryInto[Task](ctx, client, "SELECT * FROM tasks WHERE id = $1", []any{"tsk_1"})
func QueryInto[T any](ctx context.Context, c *Client, sql string, params []any) ([]T, *Meta, *KError) {
	values := make([]any, len(params))
	for i, v := range params {
		checked, kerr := checkQueryValue(v, fmt.Sprintf("params[%d]", i))
		if kerr != nil {
			return nil, nil, kerr
		}
		values[i] = checked
	}
	return runQuery[T](ctx, c, sql, values)
}

// QueryNamedInto is QueryNamed with a caller-chosen row type.
func QueryNamedInto[T any](ctx context.Context, c *Client, sql string, params map[string]any) ([]T, *Meta, *KError) {
	text, values, kerr := translateNamedParams(sql, params)
	if kerr != nil {
		return nil, nil, kerr
	}
	return runQuery[T](ctx, c, text, values)
}

func runQuery[T any](ctx context.Context, c *Client, sql string, values []any) ([]T, *Meta, *KError) {
	slug, kerr := requireProjectSlug(c.config, "query")
	if kerr != nil {
		return nil, nil, kerr
	}
	body := map[string]any{"sql": sql}
	if len(values) > 0 {
		body["params"] = values
	}
	env, kerr := doRequest[[]T](ctx, c.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "query"), Body: body})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

func invalidFieldValue(message string, context map[string]any) *KError {
	return &KError{Code: "K9O_INVALID_FIELD_VALUE", Message: message, Context: context}
}

// checkQueryValue accepts string, bool, nil and the Go numeric types.
func checkQueryValue(v any, where string) (any, *KError) {
	switch x := v.(type) {
	case nil, string, bool:
		return v, nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return v, nil
	case float32:
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			break
		}
		return v, nil
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			break
		}
		return v, nil
	case json.Number:
		return v, nil
	}
	return nil, invalidFieldValue(where+" must be a string, number, boolean or null; serialise anything else (for example as JSON text) first", nil)
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func isIdentChar(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

// translateNamedParams rewrites :name placeholders to $n. Quoted text,
// comments, dollar-quoted strings and :: casts are copied unchanged.
func translateNamedParams(sql string, named map[string]any) (string, []any, *KError) {
	var order []string
	index := map[string]int{}
	var out strings.Builder
	n := len(sql)
	at := func(i int) byte {
		if i >= 0 && i < n {
			return sql[i]
		}
		return 0
	}
	i := 0
	for i < n {
		c := sql[i]
		next := at(i + 1)
		switch {
		case c == '\'' || ((c == 'E' || c == 'e') && next == '\'' && !(i > 0 && isIdentChar(sql[i-1]))):
			escapes := c != '\''
			j := i + 1
			if escapes {
				j = i + 2
			}
			for ; j < n; j++ {
				if escapes && sql[j] == '\\' {
					j++
					continue
				}
				if sql[j] == '\'' {
					if at(j+1) == '\'' {
						j++
						continue
					}
					break
				}
			}
			end := min(j+1, n)
			out.WriteString(sql[i:end])
			i = j + 1
		case c == '"':
			j := i + 1
			for ; j < n; j++ {
				if sql[j] == '"' {
					if at(j+1) == '"' {
						j++
						continue
					}
					break
				}
			}
			end := min(j+1, n)
			out.WriteString(sql[i:end])
			i = j + 1
		case c == '-' && next == '-':
			j := strings.IndexByte(sql[i:], '\n')
			if j == -1 {
				j = n
			} else {
				j += i
			}
			out.WriteString(sql[i:j])
			i = j
		case c == '/' && next == '*':
			depth := 1
			j := i + 2
			for j < n && depth > 0 {
				if sql[j] == '/' && at(j+1) == '*' {
					depth++
					j += 2
				} else if sql[j] == '*' && at(j+1) == '/' {
					depth--
					j += 2
				} else {
					j++
				}
			}
			out.WriteString(sql[i:j])
			i = j
		case c == '$':
			j := i + 1
			if isIdentStart(at(j)) {
				for j < n && isIdentChar(sql[j]) {
					j++
				}
			}
			if at(j) == '$' {
				tag := sql[i : j+1]
				end := n
				if close := strings.Index(sql[j+1:], tag); close != -1 {
					end = j + 1 + close + len(tag)
				}
				out.WriteString(sql[i:end])
				i = end
			} else {
				out.WriteByte(c)
				i++
			}
		case c == ':' && next == ':':
			out.WriteString("::")
			i += 2
		case c == ':' && isIdentStart(next) && !(i > 0 && isIdentChar(sql[i-1])):
			j := i + 2
			for j < n && isIdentChar(sql[j]) {
				j++
			}
			name := sql[i+1 : j]
			idx, seen := index[name]
			if !seen {
				if _, ok := named[name]; !ok {
					return "", nil, invalidFieldValue("no value for :"+name, map[string]any{"name": name})
				}
				order = append(order, name)
				idx = len(order) - 1
				index[name] = idx
			}
			fmt.Fprintf(&out, "$%d", idx+1)
			i = j
		default:
			out.WriteByte(c)
			i++
		}
	}
	values := make([]any, len(order))
	for k, name := range order {
		checked, kerr := checkQueryValue(named[name], "the value for :"+name)
		if kerr != nil {
			return "", nil, kerr
		}
		values[k] = checked
	}
	return out.String(), values, nil
}
