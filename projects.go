package k9o

import "context"

// Project is the project record the gateway returns. Fields follow the
// gateway's wire names.
type Project struct {
	ID                      string   `json:"id"`
	Slug                    string   `json:"slug"`
	OrgID                   string   `json:"orgId"`
	DisplayName             string   `json:"displayName"`
	Description             *string  `json:"description"`
	ProductScope            string   `json:"productScope"`
	Region                  string   `json:"region"`
	CustodyMode             string   `json:"custodyMode"`
	Status                  string   `json:"status"`
	Role                    *string  `json:"role"`
	DeletedAt               *string  `json:"deletedAt"`
	CreatedAt               string   `json:"createdAt"`
	SoftDeleteRetentionDays *int     `json:"softDeleteRetentionDays"`
	ProcessingContexts      []string `json:"processingContexts"`
}

// ShowroomLocale is the optional showroom locale of CreateProjectInput.
type ShowroomLocale struct {
	Code       string `json:"code"`
	Currency   string `json:"currency"`
	MoneyScale int    `json:"moneyScale"`
}

// CreateProjectInput is the body of Projects.Create; property names are the
// gateway's. Only DisplayName is required.
type CreateProjectInput struct {
	// DisplayName is the display name (1 to 128 characters). Required.
	DisplayName string `json:"displayName"`
	// Slug is the URL slug (lowercase); the gateway derives one from
	// DisplayName when empty.
	Slug string `json:"slug,omitempty"`
	// ProductScope is "core" or "kibo".
	ProductScope string `json:"productScope,omitempty"`
	// Region is the region code of the project's data.
	Region               string `json:"region,omitempty"`
	DefaultProjectRegion string `json:"defaultProjectRegion,omitempty"`
	// CustodyMode is "PLATFORM_MANAGED", "SKR", "BYOK" or "HYOK".
	CustodyMode        string          `json:"custodyMode,omitempty"`
	ProcessingContexts []string        `json:"processingContexts,omitempty"`
	ExecSeamConfined   *bool           `json:"execSeamConfined,omitempty"`
	ShowroomVertical   string          `json:"showroomVertical,omitempty"`
	Persona            string          `json:"persona,omitempty"`
	ShowroomLocale     *ShowroomLocale `json:"showroomLocale,omitempty"`
	// IdempotencyKey makes a retried create return the first result instead
	// of a second project.
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
}

// UpdateProjectInput is the body of Projects.Update: only the fields set are
// sent.
type UpdateProjectInput struct {
	Slug                    *string  `json:"slug,omitempty"`
	DisplayName             *string  `json:"displayName,omitempty"`
	Description             *string  `json:"description,omitempty"`
	Region                  *string  `json:"region,omitempty"`
	ProductScope            *string  `json:"productScope,omitempty"`
	SoftDeleteRetentionDays *int     `json:"softDeleteRetentionDays,omitempty"`
	ProcessingContexts      []string `json:"processingContexts,omitempty"`
}

// ProjectsNamespace exposes client.Projects.*.
type ProjectsNamespace struct {
	config *httpClientConfig
}

// List returns projects accessible to the caller.
func (p *ProjectsNamespace) List(ctx context.Context) ([]Project, *Meta, *KError) {
	env, kerr := doRequest[[]Project](ctx, p.config, &requestInit{
		Method: "GET",
		Path:   OrgPath("projects"),
	})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Create creates a new project under the caller's active org.
func (p *ProjectsNamespace) Create(ctx context.Context, input CreateProjectInput) (*Project, *Meta, *KError) {
	if input.Slug != "" {
		if kerr := ValidateSlug(input.Slug, "project"); kerr != nil {
			return nil, nil, kerr
		}
	}
	env, kerr := doRequest[Project](ctx, p.config, &requestInit{
		Method: "POST",
		Path:   OrgPath("projects"),
		Body:   input,
	})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Get returns a project by slug.
func (p *ProjectsNamespace) Get(ctx context.Context, slug string) (*Project, *Meta, *KError) {
	if kerr := ValidateSlug(slug, "project"); kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[Project](ctx, p.config, &requestInit{
		Method: "GET",
		Path:   OrgPath("projects/" + slug),
	})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Update mutates a project's mutable fields.
func (p *ProjectsNamespace) Update(ctx context.Context, slug string, patch UpdateProjectInput) (*Project, *Meta, *KError) {
	if kerr := ValidateSlug(slug, "project"); kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[Project](ctx, p.config, &requestInit{
		Method: "PATCH",
		Path:   OrgPath("projects/" + slug),
		Body:   patch,
	})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}
