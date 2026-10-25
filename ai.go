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
