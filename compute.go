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

// LLMUsage is the token counts the model server reported for a call.
type LLMUsage struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
}

// LLMResponse is the synchronous inference result of /invoke.
type LLMResponse struct {
	ModelID string `json:"modelId"`
	// Content is the generated text.
	Content      string   `json:"content"`
	FinishReason *string  `json:"finishReason"`
	Usage        LLMUsage `json:"usage"`
	UsageType    string   `json:"usageType"`
}

// LLMStreamChunk is one chunk of the /invoke-stream response: the model
// server's OpenAI-style chat-completion chunk, passed through unchanged. Read
// incremental text from Choices[0].Delta.Content; the final chunk may carry Usage.
type LLMStreamChunk struct {
	ID      string `json:"id,omitempty"`
	Object  string `json:"object,omitempty"`
	Model   string `json:"model,omitempty"`
	Choices []struct {
		Index int `json:"index,omitempty"`
		Delta struct {
			Role    string  `json:"role,omitempty"`
			Content *string `json:"content,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason,omitempty"`
	} `json:"choices,omitempty"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens,omitempty"`
		CompletionTokens int `json:"completion_tokens,omitempty"`
		TotalTokens      int `json:"total_tokens,omitempty"`
	} `json:"usage,omitempty"`
}

// LLMInvokeParams are the inference parameters the route reads; they are sent
// flat in the request body.
type LLMInvokeParams struct {
	// MaxTokens is 1 to 32000.
	MaxTokens int `json:"maxTokens,omitempty"`
	// Temperature is 0 to 2.
	Temperature *float64 `json:"temperature,omitempty"`
	// UsageType is a free-form usage label recorded with the call.
	UsageType string `json:"usageType,omitempty"`
}

// LLMInvokeInput is the input for Invoke / InvokeStream. The wire body is
// {prompt, maxTokens?, temperature?, usageType?}.
type LLMInvokeInput struct {
	Prompt string
	Params *LLMInvokeParams
}

func (in LLMInvokeInput) body() (map[string]any, error) {
	body := map[string]any{}
	if in.Params != nil {
		m, err := jsonMarshalToMap(in.Params)
		if err != nil {
			return nil, err
		}
		body = m
	}
	body["prompt"] = in.Prompt
	return body, nil
}

// LLMModelIDs are the curated model ids.
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
func (n *LLMsNamespace) List(ctx context.Context) ([]LLMModel, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "compute.llms")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[[]LLMModel](ctx, n.config, &requestInit{Method: "GET", Path: ProjectPath(slug, "compute/llms")})
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

// Invoke runs synchronous inference (PROJECT_AUTHOR or PROJECT_ADMIN).
func (n *LLMsNamespace) Invoke(ctx context.Context, modelID string, input LLMInvokeInput) (*LLMResponse, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "compute.llms")
	if kerr != nil {
		return nil, nil, kerr
	}
	body, err := input.body()
	if err != nil {
		return nil, nil, &KError{Code: "K9O_INTERNAL_ERROR", Message: "failed to marshal invoke params: " + err.Error()}
	}
	env, kerr := doRequest[LLMResponse](ctx, n.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "compute/llms/"+url.PathEscape(modelID)+"/invoke"), Body: body})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// InvokeStream runs streaming inference (SSE). Returns a channel of chunks and
// a channel of errors; both close when the stream terminates.
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
	body, err := input.body()
	if err != nil {
		errs := make(chan *KError, 1)
		events := make(chan LLMStreamChunk)
		errs <- &KError{Code: "K9O_INTERNAL_ERROR", Message: "failed to marshal invoke params: " + err.Error()}
		close(errs)
		close(events)
		return events, errs
	}
	return streamSSE[LLMStreamChunk](ctx, n.config, &requestInit{
		Method: "POST",
		Path:   ProjectPath(slug, "compute/llms/"+url.PathEscape(modelID)+"/invoke-stream"),
		Body:   body,
	})
}

// ComputeNamespace groups the Compute surfaces.
type ComputeNamespace struct {
	LLMs *LLMsNamespace
}

func newComputeNamespace(cfg *httpClientConfig) *ComputeNamespace {
	return &ComputeNamespace{LLMs: &LLMsNamespace{config: cfg}}
}
