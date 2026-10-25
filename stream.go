package k9o

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// requireProjectSlug returns the bound project slug or a *KError when
// none is set. Shared by the project-scoped P6 namespaces.
func requireProjectSlug(cfg *httpClientConfig, surface string) (string, *KError) {
	if cfg.ProjectSlug == "" {
		return "", &KError{
			Code:    "K9O_MISSING_PROJECT_CONTEXT",
			Message: surface + " operations require ProjectSlug bound on the client",
		}
	}
	return cfg.ProjectSlug, nil
}

// streamSSE opens a Server-Sent-Events stream against the gateway and
// fans decoded payloads of type T onto the returned events channel.
// Errors (transport, non-2xx, decode) arrive on the errs channel. Both
// channels are closed when the stream terminates (on the [DONE]
// sentinel, EOF, or ctx cancellation).
//
// Per the Go idiom for async iteration is a channel; the SDK
// abstracts the transport
// (feedback_sdk_abstracted_primitive_default_to_richer_interaction.md).
// Used by Compute LLMs InvokeStream + Realtime Subscribe
// SSE transport.
func streamSSE[T any](ctx context.Context, cfg *httpClientConfig, init *requestInit) (<-chan T, <-chan *KError) {
	events := make(chan T)
	errs := make(chan *KError, 1)

	go func() {
		defer close(events)
		defer close(errs)

		urlStr := strings.TrimRight(cfg.resolveOrigin(ctx), "/") + init.Path
		if len(init.Query) > 0 {
			params := []string{}
			for k, v := range init.Query {
				params = append(params, fmt.Sprintf("%s=%s", k, v))
			}
			urlStr = urlStr + "?" + strings.Join(params, "&")
		}

		var bodyReader io.Reader
		headers := cfg.buildHeaders(init)
		headers.Set("Accept", "text/event-stream")
		if init.Body != nil {
			b, err := json.Marshal(init.Body)
			if err != nil {
				errs <- &KError{Code: "K9O_INTERNAL_ERROR", Message: fmt.Sprintf("failed to marshal request body: %v", err)}
				return
			}
			bodyReader = strings.NewReader(string(b))
			headers.Set("Content-Type", "application/json")
		}

		req, err := http.NewRequestWithContext(ctx, init.Method, urlStr, bodyReader)
		if err != nil {
			errs <- &KError{Code: "K9O_INTERNAL_ERROR", Message: fmt.Sprintf("failed to build request: %v", err)}
			return
		}
		req.Header = headers

		httpClient := cfg.HTTPClient
		if httpClient == nil {
			httpClient = http.DefaultClient
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			errs <- &KError{
				Code:    "K9O_UPSTREAM_UNAVAILABLE",
				Message: fmt.Sprintf("network error reaching %s: %v", urlStr, err),
				Context: map[string]any{"url": urlStr},
			}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			respBytes, _ := io.ReadAll(resp.Body)
			var wire KErrorWire
			if json.Unmarshal(respBytes, &wire) == nil && wire.Code != "" && wire.Message != "" {
				errs <- kerrorFromWire(&wire, resp.StatusCode)
				return
			}
			errs <- &KError{
				Code:    "K9O_INTERNAL_ERROR",
				Message: fmt.Sprintf("upstream returned %d opening stream", resp.StatusCode),
				Context: map[string]any{"url": urlStr, "status": resp.StatusCode},
				Status:  resp.StatusCode,
			}
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		var dataLines []string
		flush := func() bool {
			if len(dataLines) == 0 {
				return true
			}
			payload := strings.Join(dataLines, "\n")
			dataLines = dataLines[:0]
			if payload == "[DONE]" {
				return false
			}
			var item T
			if err := json.Unmarshal([]byte(payload), &item); err != nil {
				errs <- &KError{Code: "K9O_INTERNAL_ERROR", Message: fmt.Sprintf("failed to decode stream chunk: %v", err)}
				return false
			}
			select {
			case events <- item:
				return true
			case <-ctx.Done():
				return false
			}
		}

		for scanner.Scan() {
			line := strings.TrimRight(scanner.Text(), "\r")
			if line == "" {
				if !flush() {
					return
				}
				continue
			}
			if strings.HasPrefix(line, ":") {
				continue
			}
			if strings.HasPrefix(line, "data:") {
				dataLines = append(dataLines, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
			}
		}
		flush()
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			errs <- &KError{Code: "K9O_UPSTREAM_UNAVAILABLE", Message: fmt.Sprintf("stream read error: %v", err)}
		}
	}()

	return events, errs
}
