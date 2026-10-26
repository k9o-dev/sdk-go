package k9o

import (
	"context"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// PendingChange is a change waiting for (or past) a decision. Status is
// "pending_approval", "approved", "rejected", "expired" or "redeemed".
type PendingChange struct {
	// ID is "apr_<base62>".
	ID         string `json:"id"`
	Status     string `json:"status"`
	Collection string `json:"collection"`
	// Operation is "insert", "update" or "delete".
	Operation string `json:"operation"`
	MakerID   string `json:"makerId"`
	// ProposedPayload is the proposed record for a data write; for other kinds
	// of change, a description of the action.
	ProposedPayload map[string]any `json:"proposedPayload"`
	TargetRecordID  *string        `json:"targetRecordId"`
	ApproverID      *string        `json:"approverId"`
	DecidedAt       *string        `json:"decidedAt"`
	DecisionReason  *string        `json:"decisionReason"`
	CreatedAt       string         `json:"createdAt"`
	ExpiresAt       *string        `json:"expiresAt"`
}

// ListApprovalsOptions filters Approvals.List. Empty Status means all
// statuses; a zero Limit uses the gateway default (50).
type ListApprovalsOptions struct {
	// Status is one of "pending_approval", "approved", "rejected", "expired", "redeemed".
	Status string
	// Limit is 1 to 200.
	Limit int
}

// ApproveInput is the body of Approvals.Approve. Reason is an optional note
// stored with the decision; nil omits it.
type ApproveInput struct {
	Reason *string `json:"reason,omitempty"`
}

// RejectInput is the body of Approvals.Reject. Reason is required and must be
// non-empty.
type RejectInput struct {
	Reason string `json:"reason"`
}

// BulkApprovalsInput is the body of Approvals.Bulk.
type BulkApprovalsInput struct {
	// Action is "approve" or "reject".
	Action string `json:"action"`
	// Criteria is a predicate over each pending change, for example
	// {"eq": ["change.table", "orders"]}. It must not be nil.
	Criteria map[string]any `json:"criteria"`
	// Reason is required when Action is "reject".
	Reason string `json:"reason,omitempty"`
}

// BulkApprovalsResult is the result of Approvals.Bulk. Rows that fail are
// skipped silently and left out of AffectedIDs: compare it with the set you
// meant to decide.
type BulkApprovalsResult struct {
	AttestationID string   `json:"attestationId"`
	Action        string   `json:"action"`
	AffectedCount int      `json:"affectedCount"`
	AffectedIDs   []string `json:"affectedIds"`
	Reason        *string  `json:"reason"`
}

// ApprovalPolicy is a rule: writes to Collection with Operation that match
// Predicate get Action.
type ApprovalPolicy struct {
	// ID is "apo_<base62>".
	ID         string         `json:"id"`
	Collection string         `json:"collection"`
	Operation  string         `json:"operation"`
	Predicate  map[string]any `json:"predicate"`
	// Action is "auto_approve" or "queue".
	Action     string `json:"action"`
	AuthoredBy string `json:"authoredBy"`
	CreatedAt  string `json:"createdAt"`
}

// CreateApprovalPolicyInput is the body of ApprovalPolicies.Create.
type CreateApprovalPolicyInput struct {
	// Collection matches ^[a-zA-Z_][a-zA-Z0-9_]{0,62}$.
	Collection string `json:"collection"`
	// Operation is "insert", "update" or "delete".
	Operation string `json:"operation"`
	// Action is "auto_approve" or "queue".
	Action string `json:"action"`
	// Predicate is a non-nil predicate object.
	Predicate map[string]any `json:"predicate"`
}

// ApprovalPolicyDeleted is the result of ApprovalPolicies.Delete.
type ApprovalPolicyDeleted struct {
	ID string `json:"id"`
}

var approvalCollectionRE = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)

func invalidField(message string, context map[string]any) *KError {
	return &KError{Code: "K9O_INVALID_FIELD_VALUE", Message: message, Context: context}
}

func requireID(id, what string) *KError {
	if id == "" {
		return invalidField(what+" id must be a non-empty string", nil)
	}
	return nil
}

func requireReason(reason string) *KError {
	if strings.TrimSpace(reason) == "" {
		return invalidField("reason is required and must be a non-empty string", nil)
	}
	return nil
}

func checkLimit(limit int) *KError {
	if limit != 0 && (limit < 1 || limit > 200) {
		return invalidField("limit must be a whole number from 1 to 200", map[string]any{"limit": limit})
	}
	return nil
}

// ApprovalsNamespace exposes client.Approvals.* (the pending-change queue).
type ApprovalsNamespace struct {
	config *httpClientConfig
}

func approvalsPath(slug, rest string) string {
	p := "pending-approvals"
	if rest != "" {
		p += "/" + rest
	}
	return ProjectPath(slug, p)
}

// List returns pending changes, newest first: GET /@<slug>/pending-approvals.
// It is not paginated: only Limit (1 to 200, default 50) caps the result.
// Project admins and auditors see the whole queue; other members see only the
// changes they proposed.
func (n *ApprovalsNamespace) List(ctx context.Context, opts ListApprovalsOptions) ([]PendingChange, *Meta, *KError) {
	if kerr := checkLimit(opts.Limit); kerr != nil {
		return nil, nil, kerr
	}
	slug, kerr := requireProjectSlug(n.config, "approvals")
	if kerr != nil {
		return nil, nil, kerr
	}
	q := map[string]string{}
	if opts.Status != "" {
		q["status"] = opts.Status
	}
	if opts.Limit != 0 {
		q["limit"] = strconv.Itoa(opts.Limit)
	}
	env, kerr := doRequest[[]PendingChange](ctx, n.config, &requestInit{Method: "GET", Path: approvalsPath(slug, ""), Query: q})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Get returns one pending change. Members who are not queue admins can read
// only their own.
func (n *ApprovalsNamespace) Get(ctx context.Context, id string) (*PendingChange, *Meta, *KError) {
	if kerr := requireID(id, "approval"); kerr != nil {
		return nil, nil, kerr
	}
	slug, kerr := requireProjectSlug(n.config, "approvals")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[PendingChange](ctx, n.config, &requestInit{Method: "GET", Path: approvalsPath(slug, url.PathEscape(id))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Approve approves a pending change. Read Status on the result:
// "pending_approval" means your approval was recorded but more approvers are
// needed and the change is not applied yet.
func (n *ApprovalsNamespace) Approve(ctx context.Context, id string, input ApproveInput) (*PendingChange, *Meta, *KError) {
	if kerr := requireID(id, "approval"); kerr != nil {
		return nil, nil, kerr
	}
	slug, kerr := requireProjectSlug(n.config, "approvals")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[PendingChange](ctx, n.config, &requestInit{Method: "POST", Path: approvalsPath(slug, url.PathEscape(id)+"/approve"), Body: input})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Reject rejects a pending change. Reason is required.
func (n *ApprovalsNamespace) Reject(ctx context.Context, id string, input RejectInput) (*PendingChange, *Meta, *KError) {
	if kerr := requireReason(input.Reason); kerr != nil {
		return nil, nil, kerr
	}
	if kerr := requireID(id, "approval"); kerr != nil {
		return nil, nil, kerr
	}
	slug, kerr := requireProjectSlug(n.config, "approvals")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[PendingChange](ctx, n.config, &requestInit{Method: "POST", Path: approvalsPath(slug, url.PathEscape(id)+"/reject"), Body: input})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Bulk approves or rejects every pending change matching Criteria. Rows that
// fail are skipped without a report, so compare AffectedIDs with what you
// intended. It considers the newest 10000 pending changes.
func (n *ApprovalsNamespace) Bulk(ctx context.Context, input BulkApprovalsInput) (*BulkApprovalsResult, *Meta, *KError) {
	if input.Action != "approve" && input.Action != "reject" {
		return nil, nil, invalidField(`action must be "approve" or "reject"`, nil)
	}
	if input.Criteria == nil {
		return nil, nil, invalidField("criteria must be a predicate object", nil)
	}
	if input.Action == "reject" {
		if kerr := requireReason(input.Reason); kerr != nil {
			return nil, nil, kerr
		}
	}
	slug, kerr := requireProjectSlug(n.config, "approvals")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[BulkApprovalsResult](ctx, n.config, &requestInit{Method: "POST", Path: approvalsPath(slug, "bulk"), Body: input})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// ApprovalPoliciesNamespace exposes client.ApprovalPolicies.* (the rules that
// decide which writes are queued).
type ApprovalPoliciesNamespace struct {
	config *httpClientConfig
}

func approvalPoliciesPath(slug, rest string) string {
	p := "approval-policies"
	if rest != "" {
		p += "/" + rest
	}
	return ProjectPath(slug, p)
}

// List returns all policies, oldest first. The first matching policy for a
// write decides.
func (n *ApprovalPoliciesNamespace) List(ctx context.Context) ([]ApprovalPolicy, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "approvalPolicies")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[[]ApprovalPolicy](ctx, n.config, &requestInit{Method: "GET", Path: approvalPoliciesPath(slug, "")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Create creates a policy. Identical policies are not de-duplicated.
func (n *ApprovalPoliciesNamespace) Create(ctx context.Context, input CreateApprovalPolicyInput) (*ApprovalPolicy, *Meta, *KError) {
	if !approvalCollectionRE.MatchString(input.Collection) {
		return nil, nil, invalidField("collection must match ^[a-zA-Z_][a-zA-Z0-9_]{0,62}$", map[string]any{"collection": input.Collection})
	}
	if input.Operation != "insert" && input.Operation != "update" && input.Operation != "delete" {
		return nil, nil, invalidField(`operation must be "insert", "update" or "delete"`, nil)
	}
	if input.Action != "auto_approve" && input.Action != "queue" {
		return nil, nil, invalidField(`action must be "auto_approve" or "queue"`, nil)
	}
	if input.Predicate == nil {
		return nil, nil, invalidField("predicate must be a predicate object", nil)
	}
	slug, kerr := requireProjectSlug(n.config, "approvalPolicies")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[ApprovalPolicy](ctx, n.config, &requestInit{Method: "POST", Path: approvalPoliciesPath(slug, ""), Body: input})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Delete deletes a policy.
func (n *ApprovalPoliciesNamespace) Delete(ctx context.Context, id string) (*ApprovalPolicyDeleted, *Meta, *KError) {
	if kerr := requireID(id, "policy"); kerr != nil {
		return nil, nil, kerr
	}
	slug, kerr := requireProjectSlug(n.config, "approvalPolicies")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[ApprovalPolicyDeleted](ctx, n.config, &requestInit{Method: "DELETE", Path: approvalPoliciesPath(slug, url.PathEscape(id))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}
