package k9o

import (
	"context"
	"encoding/base64"
	"net/url"
	"strconv"
)

// KiboNamespace exposes client.Kibo.* — the Kibo BYOK/HYOK vault SDK
// surface (mirrors @k9o/kibo's `client.kibo.*`).
//
// Vault.Secrets only. The zk-deks opaque store/fetch surface (`vault.zkDeks`
// in the TypeScript SDK) has not been ported to Go yet — VaultNamespace is
// left as the natural place to add a ZkDeks field alongside Secrets.
type KiboNamespace struct {
	Vault *VaultNamespace
}

// VaultNamespace groups the Kibo ZK vault surfaces.
type VaultNamespace struct {
	Secrets *SecretsNamespace
}

func newKiboNamespace(cfg *httpClientConfig) *KiboNamespace {
	return &KiboNamespace{
		Vault: &VaultNamespace{
			Secrets: &SecretsNamespace{config: cfg},
		},
	}
}

// SecretVisibility is the secret visibility tag — write ops
// (Create/Update) stay owner-only regardless of this tag. Principals/Roles
// are only meaningful when Kind == "shared".
type SecretVisibility struct {
	Kind       string   `json:"kind"`
	Principals []string `json:"principals,omitempty"`
	Roles      []string `json:"roles,omitempty"`
}

// SecretSummary is a secret WITHOUT its sealed ciphertext — the
// Create/List/Get metadata shape. NOT the shape returned by
// Update/Rotate/Move — see UpdateSecretResult/RotateSecretResult/
// MoveSecretResult, which are live-verified narrower/different.
type SecretSummary struct {
	// ID is "sec_<base62>".
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	OwnerID    string           `json:"ownerId"`
	Visibility SecretVisibility `json:"visibility"`
	Version    int              `json:"version"`
	// Kid is the wrapping-key id at this version.
	Kid       string            `json:"kid"`
	CreatedAt string            `json:"createdAt"`
	CreatedBy string            `json:"createdBy"`
	UpdatedAt string            `json:"updatedAt"`
	UpdatedBy string            `json:"updatedBy"`
	RotatedAt *string           `json:"rotatedAt"`
	RetiredAt *string           `json:"retiredAt"`
	Metadata  map[string]string `json:"metadata"`
}

// CreateSecretInput is the input to SecretsNamespace.Create.
type CreateSecretInput struct {
	// Name is the hierarchical path the secret is addressed under, e.g.
	// "byos/s3-prod" (also the :name URL segment).
	Name string
	// Region is OPTIONAL: the gateway sets the residency region of every vault
	// call itself and discards a caller-supplied `region` query parameter.
	// When non-empty it is still sent (older gateways required it).
	Region string
	// Value is the plaintext to seal, as a regular (non-base64) string —
	// Create base64-encodes it for the wire itself. Live-verified against
	// staging: the route's OpenAPI description says "the plaintext to seal"
	// but a raw non-base64 value on the wire is rejected with
	// K9O_INVALID_FIELD_VALUE ("Request body could not be parsed.").
	Value      string
	Visibility *SecretVisibility
	Metadata   map[string]string
}

// ListSecretsInput are optional filters for SecretsNamespace.List.
type ListSecretsInput struct {
	Prefix string
	Cursor string
	Limit  int
}

func (o ListSecretsInput) toQuery() map[string]string {
	q := map[string]string{}
	if o.Prefix != "" {
		q["prefix"] = o.Prefix
	}
	if o.Cursor != "" {
		q["cursor"] = o.Cursor
	}
	if o.Limit > 0 {
		q["limit"] = strconv.Itoa(o.Limit)
	}
	return q
}

// ListSecretsResult is the response of SecretsNamespace.List.
type ListSecretsResult struct {
	Secrets    []SecretSummary `json:"secrets"`
	NextCursor *string         `json:"nextCursor"`
}

// AttestationTier is the releasing-node observed attestation tier
// (K4), present on every Reveal.
type AttestationTier struct {
	// URN is the "attestation:destination:region" trust-tier URN.
	URN string `json:"urn"`
	// Operator is the node provenance: "platform" (Kibo-operated) or
	// "client" (Private Perimeter).
	Operator string `json:"operator"`
	// Verified reports whether node attestation was verified against the
	// vendor PKI on the URN's attestation segment.
	Verified bool `json:"verified"`
}

// RevealChallenge is the result of SecretsNamespace.RevealChallenge — a
// single-use challenge to present on the immediately-following Reveal call.
type RevealChallenge struct {
	Challenge string `json:"challenge"`
	ExpiresAt string `json:"expiresAt"`
}

// RevealSecretResult is the result of SecretsNamespace.Reveal. Value is the
// decoded plaintext (the SDK base64-decodes the wire value, the inverse of
// Create's encode) — never the raw base64.
type RevealSecretResult struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Version         int             `json:"version"`
	Value           string          `json:"value"`
	RevealedAt      string          `json:"revealedAt"`
	AttestationTier AttestationTier `json:"attestationTier"`
}

// UpdateSecretInput is the input to SecretsNamespace.Update.
type UpdateSecretInput struct {
	// Region is optional, as on CreateSecretInput.
	Region string
	// Value is the new plaintext — bumps Version. Update base64-encodes it
	// for the wire, same as Create.
	Value    string
	Metadata map[string]string
}

// UpdateSecretResult is the result of SecretsNamespace.Update.
//
// 🔴 Live-verified NARROWER than the documented SecretSummary: a real PUT
// against staging returns only {id, name, version}, not the full summary
// the OpenAPI schema claims. Typed to what's actually observed rather than
// the aspirational doc shape — call Get afterward if the full summary is
// needed.
type UpdateSecretResult struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version int    `json:"version"`
}

// RotateSecretResult is the result of SecretsNamespace.Rotate.
//
// 🔴 Live-verified shape, not documented anywhere in the OpenAPI schema
// (the route only declares a union of SecretSummary/RevealChallengeResponse,
// which this matches neither). LedgerSequenceNumber is undocumented; kept
// as-observed.
type RotateSecretResult struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	Version              int    `json:"version"`
	RotatedAt            string `json:"rotatedAt"`
	LedgerSequenceNumber int    `json:"ledgerSequenceNumber"`
}

// MoveSecretResult is the result of SecretsNamespace.Move.
//
// 🔴 Live-verified shape, also not the documented SecretSummary: no
// Version, and an undocumented Event field ("SECRET_PATH_CHANGED" observed;
// typed as a plain string rather than a closed enum since only one sample
// was seen and a closed enum can't be asserted from one observation).
type MoveSecretResult struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Event string `json:"event"`
}

// encodeSecretValueBase64 / decodeSecretValueBase64 handle the
// vault-secrets wire `value` field, which is base64 in BOTH directions
// (live-verified — this contradicts the OpenAPI's plain-language
// description "the plaintext to seal", which reads like it should be raw).
// Create/Update take plaintext on the public Go API and encode here before
// putting it on the wire; Reveal decodes the wire value back to plaintext
// before returning it — the caller-facing API always deals in plaintext,
// never base64, symmetric with Create/Update.
func encodeSecretValueBase64(plaintext string) string {
	return base64.StdEncoding.EncodeToString([]byte(plaintext))
}

func decodeSecretValueBase64(wireValue string) (string, *KError) {
	decoded, err := base64.StdEncoding.DecodeString(wireValue)
	if err != nil {
		return "", &KError{
			Code:    "K9O_INTERNAL_ERROR",
			Message: "failed to base64-decode revealed secret value: " + err.Error(),
		}
	}
	return string(decoded), nil
}

// SecretsNamespace exposes client.Kibo.Vault.Secrets.* — the typed secrets
// surface (services/gateway/src/routes/vault-secrets.ts): the full family
// List/Get/Create/RevealChallenge/Reveal/Update/Rotate/Move/Delete.
//
// The region argument of every operation on a NAMED secret is optional: the
// gateway sets the region itself, and a non-empty region is still sent as
// ?region= for older gateways.
type SecretsNamespace struct {
	config *httpClientConfig
}

// secretPath builds /@<projectSlug>/vault/secrets/<name>, percent-encoding
// name so a hierarchical path like "db/pw" addresses a single :name segment
// on the wire (mirrors the TS reference's encodeURIComponent(name)).
// regionQuery builds the query of a named-secret call: op (if any) plus region
// (only when non-empty).
func regionQuery(op, region string) map[string]string {
	q := map[string]string{}
	if op != "" {
		q["op"] = op
	}
	if region != "" {
		q["region"] = region
	}
	return q
}

func secretPath(projectSlug, name string) string {
	return ProjectPath(projectSlug, "vault/secrets/"+url.PathEscape(name))
}

// List returns secrets (metadata only — never lists ciphertext).
// GET /@<projectSlug>/vault/secrets. The one secrets operation that does
// NOT need region (live-verified).
func (n *SecretsNamespace) List(ctx context.Context, input ListSecretsInput) (*ListSecretsResult, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "kibo.vault.secrets")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[ListSecretsResult](ctx, n.config, &requestInit{
		Method: "GET",
		Path:   ProjectPath(slug, "vault/secrets"),
		Query:  input.toQuery(),
	})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// maxGetPages bounds the pages Get walks while looking for an exact name.
const maxGetPages = 20

// Get reads a secret's metadata (never its value — use RevealChallenge +
// Reveal for that). The vault answers GET .../vault/secrets/<name> only as a
// reveal (401 K9O_REVEAL_CHALLENGE_REQUIRED without a challenge), so Get lists
// by the exact name as prefix and picks the exact match: GET .../vault/secrets
// ?prefix=<name>&limit=100. It returns K9O_NOT_FOUND (404) when no secret has
// that name. region is accepted for symmetry and not sent.
func (n *SecretsNamespace) Get(ctx context.Context, name string, region string) (*SecretSummary, *Meta, *KError) {
	_ = region
	cursor := ""
	for page := 0; page < maxGetPages; page++ {
		res, meta, kerr := n.List(ctx, ListSecretsInput{Prefix: name, Cursor: cursor, Limit: 100})
		if kerr != nil {
			return nil, nil, kerr
		}
		for i := range res.Secrets {
			if res.Secrets[i].Name == name {
				return &res.Secrets[i], meta, nil
			}
		}
		if res.NextCursor == nil || *res.NextCursor == "" {
			break
		}
		cursor = *res.NextCursor
	}
	return nil, nil, &KError{
		Code:    "K9O_NOT_FOUND",
		Message: "No secret named '" + name + "' in this project.",
		Context: map[string]any{"name": name},
		Status:  404,
	}
}

// Create creates a secret under the caller's own auth.
// POST /@<projectSlug>/vault/secrets/<name>?region=<region> → the
// sec_<base62> summary.
func (n *SecretsNamespace) Create(ctx context.Context, input CreateSecretInput) (*SecretSummary, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "kibo.vault.secrets")
	if kerr != nil {
		return nil, nil, kerr
	}
	body := map[string]any{"value": encodeSecretValueBase64(input.Value)}
	if input.Visibility != nil {
		body["visibility"] = input.Visibility
	}
	if input.Metadata != nil {
		body["metadata"] = input.Metadata
	}
	env, kerr := doRequest[SecretSummary](ctx, n.config, &requestInit{
		Method: "POST",
		Path:   secretPath(slug, input.Name),
		Query:  regionQuery("", input.Region),
		Body:   body,
	})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// RevealChallenge mints a single-use reveal challenge — present it as
// X-K9O-Reveal-Challenge on the immediately-following Reveal call.
// POST .../<name>?op=reveal-challenge&region=<region>, no body.
func (n *SecretsNamespace) RevealChallenge(ctx context.Context, name string, region string) (*RevealChallenge, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "kibo.vault.secrets")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[RevealChallenge](ctx, n.config, &requestInit{
		Method: "POST",
		Path:   secretPath(slug, name),
		Query:  regionQuery("reveal-challenge", region),
	})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Reveal reveals a secret's plaintext value using a single-use challenge
// from RevealChallenge. GET .../<name>?region=<region> with the
// X-K9O-Reveal-Challenge header. Decodes the wire's base64 value back to
// plaintext (live-verified: the reveal response's value is base64,
// symmetric with Create's encode).
func (n *SecretsNamespace) Reveal(ctx context.Context, name string, region string, challenge string) (*RevealSecretResult, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "kibo.vault.secrets")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[RevealSecretResult](ctx, n.config, &requestInit{
		Method:  "GET",
		Path:    secretPath(slug, name),
		Query:   regionQuery("", region),
		Headers: map[string]string{"X-K9O-Reveal-Challenge": challenge},
	})
	if kerr != nil {
		return nil, nil, kerr
	}
	plaintext, kerr := decodeSecretValueBase64(env.Data.Value)
	if kerr != nil {
		return nil, nil, kerr
	}
	env.Data.Value = plaintext
	return &env.Data, &env.Meta, nil
}

// Update updates a secret's value (bumps version) — owner-only.
// PUT .../<name>?region=<region>.
//
// 🔴 Returns only {id, name, version} live (see UpdateSecretResult), not
// the full summary the OpenAPI schema documents — call Get afterward if
// you need the rest.
func (n *SecretsNamespace) Update(ctx context.Context, name string, input UpdateSecretInput) (*UpdateSecretResult, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "kibo.vault.secrets")
	if kerr != nil {
		return nil, nil, kerr
	}
	body := map[string]any{"value": encodeSecretValueBase64(input.Value)}
	if input.Metadata != nil {
		body["metadata"] = input.Metadata
	}
	env, kerr := doRequest[UpdateSecretResult](ctx, n.config, &requestInit{
		Method: "PUT",
		Path:   secretPath(slug, name),
		Query:  regionQuery("", input.Region),
		Body:   body,
	})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Rotate rotates a secret's wrapping key (re-wraps under a fresh kid;
// content unchanged) — owner-only. POST .../<name>?op=rotate&region=<region>,
// no body.
func (n *SecretsNamespace) Rotate(ctx context.Context, name string, region string) (*RotateSecretResult, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "kibo.vault.secrets")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[RotateSecretResult](ctx, n.config, &requestInit{
		Method: "POST",
		Path:   secretPath(slug, name),
		Query:  regionQuery("rotate", region),
	})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Move moves (renames) a secret's hierarchical path — owner-only.
// POST .../<name>?op=move&region=<region>, body {newPath}.
func (n *SecretsNamespace) Move(ctx context.Context, name string, region string, newPath string) (*MoveSecretResult, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "kibo.vault.secrets")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[MoveSecretResult](ctx, n.config, &requestInit{
		Method: "POST",
		Path:   secretPath(slug, name),
		Query:  regionQuery("move", region),
		Body:   map[string]string{"newPath": newPath},
	})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Delete retires a secret — owner-only. DELETE .../<name>?region=<region> →
// 204 No Content (empty body).
func (n *SecretsNamespace) Delete(ctx context.Context, name string, region string) (*Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "kibo.vault.secrets")
	if kerr != nil {
		return nil, kerr
	}
	env, kerr := doRequest[any](ctx, n.config, &requestInit{
		Method: "DELETE",
		Path:   secretPath(slug, name),
		Query:  regionQuery("", region),
	})
	if kerr != nil {
		return nil, kerr
	}
	return &env.Meta, nil
}
