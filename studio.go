package k9o

import (
	"context"
	"net/url"
)

// Workflow mirrors the _sys.workflows row.
type Workflow struct {
	ID            string           `json:"id"`
	ProjectID     string           `json:"projectId"`
	Name          string           `json:"name"`
	Description   string           `json:"description"`
	Status        string           `json:"status"`
	TriggerType   string           `json:"triggerType"`
	TriggerConfig map[string]any   `json:"triggerConfig"`
	Actions       []WorkflowAction `json:"actions"`
	RBACBinding   map[string]any   `json:"rbacBinding"`
	AuthoredBy    string           `json:"authoredBy"`
	AuthoredAt    string           `json:"authoredAt"`
	UpdatedBy     string           `json:"updatedBy"`
	UpdatedAt     string           `json:"updatedAt"`
}

// WorkflowAction is a single step in the actions chain.
type WorkflowAction struct {
	Index      int            `json:"index"`
	ActionType string         `json:"action_type"`
	Config     map[string]any `json:"config"`
	Parallel   bool           `json:"parallel,omitempty"`
}

// WorkflowRunStep mirrors a per_step_status entry.
type WorkflowRunStep struct {
	StepIndex     int            `json:"stepIndex"`
	ActionType    string         `json:"actionType"`
	Status        string         `json:"status"`
	StartedAt     string         `json:"startedAt"`
	CompletedAt   *string        `json:"completedAt,omitempty"`
	OutputSummary any            `json:"outputSummary,omitempty"`
	Error         map[string]any `json:"error,omitempty"`
	CostSubtotal  float64        `json:"costSubtotal,omitempty"`
}

// WorkflowRun mirrors the _sys.workflow_runs row.
type WorkflowRun struct {
	ID                  string            `json:"id"`
	WorkflowID          string            `json:"workflowId"`
	ProjectID           string            `json:"projectId"`
	Status              string            `json:"status"`
	TriggerContext      map[string]any    `json:"triggerContext"`
	PerStepStatus       []WorkflowRunStep `json:"perStepStatus"`
	CostAccountingTotal float64           `json:"costAccountingTotal"`
	ErrorContext        map[string]any    `json:"errorContext,omitempty"`
	StartedAt           string            `json:"startedAt"`
	CompletedAt         *string           `json:"completedAt,omitempty"`
	FiredBy             string            `json:"firedBy"`
}

// CreateWorkflowInput is the body for WorkflowsNamespace.Create.
type CreateWorkflowInput struct {
	Name          string           `json:"name"`
	Description   string           `json:"description,omitempty"`
	Status        string           `json:"status,omitempty"`
	TriggerType   string           `json:"triggerType"`
	TriggerConfig map[string]any   `json:"triggerConfig"`
	Actions       []WorkflowAction `json:"actions"`
	RBACBinding   map[string]any   `json:"rbacBinding,omitempty"`
}

// WorkflowListFilters are optional query filters for List.
type WorkflowListFilters struct {
	Status      string
	TriggerType string
	Limit       int
	Cursor      string
}

// Dashboard mirrors the _sys.dashboards row.
type Dashboard struct {
	ID                string         `json:"id"`
	ProjectID         string         `json:"projectId"`
	Name              string         `json:"name"`
	Description       string         `json:"description"`
	Status            string         `json:"status"`
	LayoutGrid        map[string]any `json:"layoutGrid"`
	Widgets           []Widget       `json:"widgets"`
	FilterConfig      map[string]any `json:"filterConfig"`
	PinnedToWorkspace bool           `json:"pinnedToWorkspace"`
	RBACBinding       map[string]any `json:"rbacBinding"`
	AuthoredBy        string         `json:"authoredBy"`
	AuthoredAt        string         `json:"authoredAt"`
	UpdatedBy         string         `json:"updatedBy"`
	UpdatedAt         string         `json:"updatedAt"`
}

// Widget is a widget config entry.
type Widget struct {
	ID         string         `json:"id"`
	WidgetType string         `json:"widgetType"`
	Title      string         `json:"title,omitempty"`
	DataSource map[string]any `json:"dataSource"`
	Config     map[string]any `json:"config,omitempty"`
}

// CreateDashboardInput is the body for DashboardsNamespace.Create.
type CreateDashboardInput struct {
	Name         string         `json:"name"`
	Description  string         `json:"description,omitempty"`
	Status       string         `json:"status,omitempty"`
	LayoutGrid   map[string]any `json:"layoutGrid,omitempty"`
	Widgets      []Widget       `json:"widgets,omitempty"`
	FilterConfig map[string]any `json:"filterConfig,omitempty"`
	RBACBinding  map[string]any `json:"rbacBinding,omitempty"`
}

// WorkflowRunsNamespace exposes client.Studio.Workflows.Runs.*.
type WorkflowRunsNamespace struct {
	config *httpClientConfig
}

// List returns runs of a workflow (cursor-paginated).
func (n *WorkflowRunsNamespace) List(ctx context.Context, workflowID string, status string) ([]WorkflowRun, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "studio.workflows")
	if kerr != nil {
		return nil, nil, kerr
	}
	q := map[string]string{}
	if status != "" {
		q["status"] = status
	}
	env, kerr := doRequest[[]WorkflowRun](ctx, n.config, &requestInit{
		Method: "GET",
		Path:   ProjectPath(slug, "workflows/"+url.PathEscape(workflowID)+"/runs"),
		Query:  q,
	})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Read returns a single run's detail (per-step status + cost).
func (n *WorkflowRunsNamespace) Read(ctx context.Context, workflowID, runID string) (*WorkflowRun, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "studio.workflows")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[WorkflowRun](ctx, n.config, &requestInit{
		Method: "GET",
		Path:   ProjectPath(slug, "workflows/"+url.PathEscape(workflowID)+"/runs/"+url.PathEscape(runID)),
	})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Cancel cancels a pending or running run.
func (n *WorkflowRunsNamespace) Cancel(ctx context.Context, workflowID, runID string) (*WorkflowRun, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "studio.workflows")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[WorkflowRun](ctx, n.config, &requestInit{
		Method: "POST",
		Path:   ProjectPath(slug, "workflows/"+url.PathEscape(workflowID)+"/runs/"+url.PathEscape(runID)+"/cancel"),
	})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// WorkflowsNamespace exposes client.Studio.Workflows.*.
type WorkflowsNamespace struct {
	config *httpClientConfig
	Runs   *WorkflowRunsNamespace
}

// List returns workflows (cursor-paginated).
func (n *WorkflowsNamespace) List(ctx context.Context, filters WorkflowListFilters) ([]Workflow, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "studio.workflows")
	if kerr != nil {
		return nil, nil, kerr
	}
	q := map[string]string{}
	if filters.Status != "" {
		q["status"] = filters.Status
	}
	if filters.TriggerType != "" {
		q["triggerType"] = filters.TriggerType
	}
	if filters.Cursor != "" {
		q["cursor"] = filters.Cursor
	}
	env, kerr := doRequest[[]Workflow](ctx, n.config, &requestInit{Method: "GET", Path: ProjectPath(slug, "workflows"), Query: q})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Create creates a workflow (requires PROJECT_AUTHOR).
func (n *WorkflowsNamespace) Create(ctx context.Context, input CreateWorkflowInput) (*Workflow, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "studio.workflows")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[Workflow](ctx, n.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "workflows"), Body: input})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Read returns a workflow by id.
func (n *WorkflowsNamespace) Read(ctx context.Context, workflowID string) (*Workflow, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "studio.workflows")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[Workflow](ctx, n.config, &requestInit{Method: "GET", Path: ProjectPath(slug, "workflows/"+url.PathEscape(workflowID))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Update mutates a workflow (requires PROJECT_AUTHOR).
func (n *WorkflowsNamespace) Update(ctx context.Context, workflowID string, updates map[string]any) (*Workflow, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "studio.workflows")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[Workflow](ctx, n.config, &requestInit{Method: "PATCH", Path: ProjectPath(slug, "workflows/"+url.PathEscape(workflowID)), Body: updates})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Delete deletes a workflow (requires PROJECT_AUTHOR).
func (n *WorkflowsNamespace) Delete(ctx context.Context, workflowID string) (*Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "studio.workflows")
	if kerr != nil {
		return nil, kerr
	}
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "DELETE", Path: ProjectPath(slug, "workflows/"+url.PathEscape(workflowID))})
	if kerr != nil {
		return nil, kerr
	}
	return &env.Meta, nil
}

// Trigger manual-triggers a run (trigger_type=manual only).
func (n *WorkflowsNamespace) Trigger(ctx context.Context, workflowID string, triggerContext map[string]any) (*WorkflowRun, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "studio.workflows")
	if kerr != nil {
		return nil, nil, kerr
	}
	if triggerContext == nil {
		triggerContext = map[string]any{}
	}
	env, kerr := doRequest[WorkflowRun](ctx, n.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "workflows/"+url.PathEscape(workflowID)+"/runs"), Body: triggerContext})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// DashboardsNamespace exposes client.Studio.Dashboards.*.
type DashboardsNamespace struct {
	config *httpClientConfig
}

// List returns dashboards (cursor-paginated).
func (n *DashboardsNamespace) List(ctx context.Context, status string) ([]Dashboard, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "studio.dashboards")
	if kerr != nil {
		return nil, nil, kerr
	}
	q := map[string]string{}
	if status != "" {
		q["status"] = status
	}
	env, kerr := doRequest[[]Dashboard](ctx, n.config, &requestInit{Method: "GET", Path: ProjectPath(slug, "dashboards"), Query: q})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Create creates a dashboard (requires PROJECT_AUTHOR).
func (n *DashboardsNamespace) Create(ctx context.Context, input CreateDashboardInput) (*Dashboard, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "studio.dashboards")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[Dashboard](ctx, n.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "dashboards"), Body: input})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Read returns a dashboard by id.
func (n *DashboardsNamespace) Read(ctx context.Context, dashboardID string) (*Dashboard, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "studio.dashboards")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[Dashboard](ctx, n.config, &requestInit{Method: "GET", Path: ProjectPath(slug, "dashboards/"+url.PathEscape(dashboardID))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Update mutates a dashboard (requires PROJECT_AUTHOR).
func (n *DashboardsNamespace) Update(ctx context.Context, dashboardID string, updates map[string]any) (*Dashboard, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "studio.dashboards")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[Dashboard](ctx, n.config, &requestInit{Method: "PATCH", Path: ProjectPath(slug, "dashboards/"+url.PathEscape(dashboardID)), Body: updates})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Delete deletes a dashboard (requires PROJECT_AUTHOR).
func (n *DashboardsNamespace) Delete(ctx context.Context, dashboardID string) (*Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "studio.dashboards")
	if kerr != nil {
		return nil, kerr
	}
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "DELETE", Path: ProjectPath(slug, "dashboards/"+url.PathEscape(dashboardID))})
	if kerr != nil {
		return nil, kerr
	}
	return &env.Meta, nil
}

// Pin pins a dashboard to the Workspace sidebar (requires PROJECT_AUTHOR).
func (n *DashboardsNamespace) Pin(ctx context.Context, dashboardID string) (*Dashboard, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "studio.dashboards")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[Dashboard](ctx, n.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "dashboards/"+url.PathEscape(dashboardID)+"/pin")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Unpin unpins a dashboard (requires PROJECT_AUTHOR).
func (n *DashboardsNamespace) Unpin(ctx context.Context, dashboardID string) (*Dashboard, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "studio.dashboards")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[Dashboard](ctx, n.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "dashboards/"+url.PathEscape(dashboardID)+"/unpin")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// StudioNamespace groups the Pathmaker-authored Studio surfaces.
type StudioNamespace struct {
	Workflows  *WorkflowsNamespace
	Dashboards *DashboardsNamespace
}

func newStudioNamespace(cfg *httpClientConfig) *StudioNamespace {
	return &StudioNamespace{
		Workflows:  &WorkflowsNamespace{config: cfg, Runs: &WorkflowRunsNamespace{config: cfg}},
		Dashboards: &DashboardsNamespace{config: cfg},
	}
}
