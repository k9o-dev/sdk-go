package k9o

import "context"

// EmbedInput is the body for AI.Embed.
//
// Input is either a single string or a []string; it is sent exactly as given
// (a single string stays a string on the wire). Model is optional and omitted
// from the body when empty (the server defaults to the one declared embedding
// model). The SDK does no content validation: bounds are enforced by the
// gateway, and the input text is never placed in an SDK-constructed error.
type EmbedInput struct {
	Model string
	Input any
}

// EmbedResult is the embedding result: one vector per input string, in order.
type EmbedResult struct {
	Embeddings [][]float64 `json:"embeddings"`
	Model      string      `json:"model"`
	Dimensions int         `json:"dimensions"`
}

// AINamespace exposes client.AI.*. Platform AI primitives; callers are an
// extension (manifest grant `(ai, embed, "*")`) or a project member's backend.
type AINamespace struct {
	config *httpClientConfig
}

// Embed embeds one or more strings: POST /@<slug>/ai/embeddings.
func (n *AINamespace) Embed(ctx context.Context, input EmbedInput) (*EmbedResult, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "ai")
	if kerr != nil {
		return nil, nil, kerr
	}
	body := map[string]any{"input": input.Input}
	if input.Model != "" {
		body["model"] = input.Model
	}
	env, kerr := doRequest[EmbedResult](ctx, n.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "ai/embeddings"), Body: body})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// GenerateOptions are the optional fields of AI.Generate; zero values are
// omitted from the body. Model is the model id; when empty the gateway uses
// the org's default LLM. MaxTokens is 1 to 32000, Temperature is 0 to 2
// (a pointer so 0 can be sent) and UsageType is a free-form usage label.
type GenerateOptions struct {
	Model       string
	MaxTokens   int
	Temperature *float64
	UsageType   string
}

// Generate runs a prompt through an LLM: POST /@<slug>/ai/generate. It is a
// verb over Compute.LLMs.Invoke and returns the same result. The gateway
// answers 400 when no Model is given and the org has no default LLM, and 501
// K9O_NOT_IMPLEMENTED for models served by an outside provider.
func (n *AINamespace) Generate(ctx context.Context, prompt string, opts GenerateOptions) (*LLMResponse, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "ai")
	if kerr != nil {
		return nil, nil, kerr
	}
	body := generateBody{Prompt: prompt, Model: opts.Model, MaxTokens: opts.MaxTokens, Temperature: opts.Temperature, UsageType: opts.UsageType}
	env, kerr := doRequest[LLMResponse](ctx, n.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "ai/generate"), Body: body})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// generateBody is the wire body; the struct keeps prompt first and omits unset fields.
type generateBody struct {
	Prompt      string   `json:"prompt"`
	Model       string   `json:"model,omitempty"`
	MaxTokens   int      `json:"maxTokens,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
	UsageType   string   `json:"usageType,omitempty"`
}
