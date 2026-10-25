package k9o

import (
	"context"
	"net/url"
)

// LLMModel is the model metadata.
type LLMModel struct {
	ID                         string   `json:"id"`
	ProviderClass              string   `json:"providerClass"`
	Modality                   string   `json:"modality"`
	TierGate                   string   `json:"tierGate"`
	ContextWindow              int      `json:"contextWindow"`
	Capabilities               []string `json:"capabilities"`
	CostPerMillionInputTokens  float64  `json:"costPerMillionInputTokens"`
	CostPerMillionOutputTokens float64  `json:"costPerMillionOutputTokens"`
}

// LLMResponse is the synchronous inference result.
type LLMResponse struct {
	Completion string `json:"completion"`
	TokensUsed int    `json:"tokensUsed"`
	LatencyMs  int    `json:"latencyMs"`
}

// LLMStreamChunk is a streaming per-token chunk.
type LLMStreamChunk struct {
	Delta      string `json:"delta"`
	Done       bool   `json:"done,omitempty"`
	TokensUsed int    `json:"tokensUsed,omitempty"`
}

// LLMInvokeInput is the body for Invoke / InvokeStream.
type LLMInvokeInput struct {
	Prompt string         `json:"prompt"`
	Params map[string]any `json:"params,omitempty"`
}

// LLMModelIDs is the curated closed enum (extensible v2.x).
var LLMModelIDs = []string{
	"mistral-7b", "mistral-8x7b", "llama-3-8b", "llama-3-70b",
	"claude-3-5-sonnet", "gpt-4o", "gpt-4o-mini",
	"text-embedding-3-small", "text-embedding-3-large",
}

// LLMsNamespace exposes client.Compute.LLMs.*.
type LLMsNamespace struct {
	config *httpClientConfig
}

// List returns models available to the project tier.
func (n *LLMsNamespace) List(ctx context.Context, modality string) ([]LLMModel, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "compute.llms")
	if kerr != nil {
		return nil, nil, kerr
	}
	q := map[string]string{}
	if modality != "" {
		q["modality"] = modality
	}
	env, kerr := doRequest[[]LLMModel](ctx, n.config, &requestInit{Method: "GET", Path: ProjectPath(slug, "compute/llms"), Query: q})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Read returns single-model metadata + cost.
func (n *LLMsNamespace) Read(ctx context.Context, modelID string) (*LLMModel, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "compute.llms")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[LLMModel](ctx, n.config, &requestInit{Method: "GET", Path: ProjectPath(slug, "compute/llms/"+url.PathEscape(modelID))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Invoke runs synchronous inference (PROJECT_AUTHOR/ADMIN).
func (n *LLMsNamespace) Invoke(ctx context.Context, modelID string, input LLMInvokeInput) (*LLMResponse, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "compute.llms")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[LLMResponse](ctx, n.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "compute/llms/"+url.PathEscape(modelID)+"/invoke"), Body: input})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// InvokeStream runs streaming inference (SSE per-token).
// Returns a channel of chunks + a channel of errors per the Go
// async-iteration idiom; both close when the stream terminates.
func (n *LLMsNamespace) InvokeStream(ctx context.Context, modelID string, input LLMInvokeInput) (<-chan LLMStreamChunk, <-chan *KError) {
	slug, kerr := requireProjectSlug(n.config, "compute.llms")
	if kerr != nil {
		errs := make(chan *KError, 1)
		events := make(chan LLMStreamChunk)
		errs <- kerr
		close(errs)
		close(events)
		return events, errs
	}
	return streamSSE[LLMStreamChunk](ctx, n.config, &requestInit{
		Method: "POST",
		Path:   ProjectPath(slug, "compute/llms/"+url.PathEscape(modelID)+"/invoke-stream"),
		Body:   input,
	})
}

// ComputeNamespace groups the Compute surfaces.
type ComputeNamespace struct {
	LLMs *LLMsNamespace
}

func newComputeNamespace(cfg *httpClientConfig) *ComputeNamespace {
	return &ComputeNamespace{LLMs: &LLMsNamespace{config: cfg}}
}
