package k9o

import (
	"math/rand"
	"strings"
)

// RetryConfig controls retry + backoff. Retries apply to
// idempotent methods (GET/HEAD/OPTIONS/DELETE) on transient failures only
// (transport error / 502 / 503 / 504 / 429); semantic 4xx and
// non-idempotent methods are never retried.
type RetryConfig struct {
	MaxRetries     int
	Backoff        string // "exponential" | "linear" | "none"
	InitialDelayMs int
	MaxDelayMs     int
}

func defaultRetryConfig() *RetryConfig {
	return &RetryConfig{MaxRetries: 3, Backoff: "exponential", InitialDelayMs: 100, MaxDelayMs: 5000}
}

var retryableMethods = map[string]bool{"GET": true, "HEAD": true, "OPTIONS": true, "DELETE": true}
var retryableStatus = map[int]bool{502: true, 503: true, 504: true}

// isRetryable reports whether a failed attempt should be retried.
func isRetryable(kerr *KError, method string) bool {
	if !retryableMethods[strings.ToUpper(method)] {
		return false
	}
	if kerr == nil {
		return false
	}
	if kerr.Code == "K9O_UPSTREAM_UNAVAILABLE" {
		return true
	}
	if retryableStatus[kerr.Status] {
		return true
	}
	if kerr.Status == 429 {
		return true
	}
	return false
}

// computeDelayMs returns the backoff delay (ms) for a 0-based attempt index,
// capped at MaxDelayMs and ±25% jittered.
func computeDelayMs(cfg *RetryConfig, attempt int) int {
	if cfg.Backoff == "none" {
		return 0
	}
	var base int
	if cfg.Backoff == "linear" {
		base = cfg.InitialDelayMs * (attempt + 1)
	} else {
		base = cfg.InitialDelayMs * (1 << uint(attempt))
	}
	if base > cfg.MaxDelayMs {
		base = cfg.MaxDelayMs
	}
	jitter := float64(base) * 0.25 * (rand.Float64()*2 - 1)
	d := int(float64(base) + jitter)
	if d < 0 {
		d = 0
	}
	return d
}
