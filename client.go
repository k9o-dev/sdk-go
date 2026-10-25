package k9o

import (
	"net/http"
)

// ClientOpts configures NewClient. All fields are optional.
type ClientOpts struct {
	// BaseURL is the gateway origin (scheme + host); rejects path/query/fragment.
	// OPTIONAL — empty defaults to the platform apex
	// "https://api.k9o.io". When BaseURL is the bare apex and OrgSlug is set,
	// the SDK derives the canonical org-subdomain root
	// "https://<OrgSlug>.api.k9o.io". Any other host is used unchanged.
	BaseURL string
	// APIKey is the default bearer token; per-call override is supported by
	// internal request helpers (not currently exposed on the public API).
	APIKey string
	// OrgSlug is the org context; sent as X-K9O-Org-Slug header.
	OrgSlug string
	// ProjectSlug is the project context; sent as X-K9O-Project-Slug header.
	ProjectSlug string
	// DefaultHeaders are merged into every request.
	DefaultHeaders map[string]string
	// HTTPClient overrides the default http.Client (useful for tests + timeouts).
	HTTPClient *http.Client
	// RetryConfig overrides retry/backoff; nil ⇒ defaults
	// (3 retries, exponential) on idempotent methods + transient failures.
	RetryConfig *RetryConfig
}

// Client is the customer-facing SDK client. Namespaces hang off the
// struct via promoted fields:
//
//	client.Projects.List(ctx)
//	client.Gate.SignIn(ctx, ...)
//
// Data DSL via the From method:
//
//	data, meta, kerr := k9o.SelectExecute[YourRowType](ctx, client.From("posts").Select())
type Client struct {
	config *httpClientConfig

	Projects   *ProjectsNamespace
	Gate       *GateNamespace
	Studio     *StudioNamespace
	Compute    *ComputeNamespace
	AI         *AINamespace
	Realtime   *RealtimeNamespace
	Org        *OrgNamespace
	Orgs       *OrgsNamespace
	Migrations *MigrationsNamespace
	Functions  *FunctionsNamespace
	Policies   *PoliciesNamespace
	Audit      *AuditNamespace
	Me         *MeNamespace
	Kibo       *KiboNamespace
	Storage    *StorageNamespace
}

// NewClient constructs a Client bound to the supplied options. Resolves
// BaseURL (optional → apex default; bare-apex + OrgSlug →
// derived org-subdomain; any other host passes through) and validates it
// at construct time; returns *KError(K9O_INVALID_BASE_URL) on
// violation.
func NewClient(opts ClientOpts) (*Client, *KError) {
	normalised, kerr := ResolveBaseURL(opts.BaseURL, opts.OrgSlug)
	if kerr != nil {
		return nil, kerr
	}
	cfg := &httpClientConfig{
		BaseURL:        normalised,
		APIKey:         opts.APIKey,
		OrgSlug:        opts.OrgSlug,
		ProjectSlug:    opts.ProjectSlug,
		DefaultHeaders: opts.DefaultHeaders,
		HTTPClient:     opts.HTTPClient,
		Retry:          opts.RetryConfig,
	}
	return &Client{
		config:     cfg,
		Projects:   &ProjectsNamespace{config: cfg},
		Gate:       &GateNamespace{config: cfg},
		Studio:     newStudioNamespace(cfg),
		Compute:    newComputeNamespace(cfg),
		AI:         &AINamespace{config: cfg},
		Realtime:   &RealtimeNamespace{config: cfg},
		Org:        newOrgNamespace(cfg),
		Orgs:       &OrgsNamespace{config: cfg},
		Migrations: &MigrationsNamespace{config: cfg},
		Functions:  &FunctionsNamespace{config: cfg},
		Policies:   &PoliciesNamespace{config: cfg},
		Audit:      &AuditNamespace{config: cfg},
		Me:         newMeNamespace(cfg),
		Kibo:       newKiboNamespace(cfg),
		Storage:    &StorageNamespace{config: cfg},
	}, nil
}

// Config returns the bound HTTP config (read-only access for advanced use).
func (c *Client) Config() ClientOpts {
	return ClientOpts{
		BaseURL:        c.config.BaseURL,
		APIKey:         c.config.APIKey,
		OrgSlug:        c.config.OrgSlug,
		ProjectSlug:    c.config.ProjectSlug,
		DefaultHeaders: c.config.DefaultHeaders,
		HTTPClient:     c.config.HTTPClient,
	}
}

// From is the data-plane DSL entry point.
//
//	rows, meta, kerr := k9o.SelectExecute[Post](ctx, client.From("posts").Select().Eq("status", "published"))
func (c *Client) From(collection string) *FromBuilder {
	return &FromBuilder{config: c.config, collection: collection}
}
