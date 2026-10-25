package k9o

import "context"

// Project is the canonical project shape.
type Project struct {
	ID          string  `json:"id"`
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	CustodyMode string  `json:"custodyMode"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   *string `json:"updatedAt,omitempty"`
	OrgID       string  `json:"orgId"`
}

// CreateProjectInput is the input shape for Projects.Create.
type CreateProjectInput struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	CustodyMode string `json:"custodyMode,omitempty"`
}

// UpdateProjectInput is the input shape for Projects.Update.
type UpdateProjectInput struct {
	Name *string `json:"name,omitempty"`
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
	if kerr := ValidateSlug(input.Slug, "project"); kerr != nil {
		return nil, nil, kerr
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
		Path:   ProjectPath(slug, "project"),
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
		Path:   ProjectPath(slug, "project"),
		Body:   patch,
	})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}
