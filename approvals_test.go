package k9o

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func approvalsClient(t *testing.T, data any, calls *[]streamsCall) *Client {
	t.Helper()
	return p6Client(t, roundTripFunc(func(req *http.Request) *http.Response {
		*calls = append(*calls, recordCall(req))
		return jsonResponse(200, map[string]any{"data": data, "meta": map[string]any{"cursor": map[string]any{"next": "dGFza3M"}}})
	}))
}

func wantInvalid(t *testing.T, name string, kerr *KError, calls []streamsCall) {
	t.Helper()
	if kerr == nil || kerr.Code != "K9O_INVALID_FIELD_VALUE" {
		t.Fatalf("%s: want K9O_INVALID_FIELD_VALUE, got %v", name, kerr)
	}
	if len(calls) != 0 {
		t.Fatalf("%s: a request was sent: %+v", name, calls)
	}
}

func TestApprovalsWire(t *testing.T) {
	var calls []streamsCall
	pending := map[string]any{"id": "apr_1", "status": "pending_approval", "collection": "orders", "operation": "update", "makerId": "usr_1", "proposedPayload": map[string]any{"amount": 5}, "targetRecordId": nil, "createdAt": "t"}
	c := approvalsClient(t, pending, &calls)
	ctx := context.Background()

	got, _, kerr := c.Approvals.Approve(ctx, "apr_1", ApproveInput{})
	if kerr != nil || got.Status != "pending_approval" || got.TargetRecordID != nil || got.ProposedPayload["amount"] != float64(5) {
		t.Fatalf("Approve: %+v %v", got, kerr)
	}
	if calls[0].Method != "POST" || calls[0].Path != "/@portal/pending-approvals/apr_1/approve" || calls[0].Body != "{}" {
		t.Fatalf("approve wire: %+v", calls[0])
	}

	calls = nil
	c.Approvals.List(ctx, ListApprovalsOptions{})
	if calls[0].Method != "GET" || calls[0].Path != "/@portal/pending-approvals" || len(calls[0].Query) != 0 {
		t.Fatalf("list default wire: %+v", calls[0])
	}
	calls = nil
	c.Approvals.Get(ctx, "apr_1")
	if calls[0].Method != "GET" || calls[0].Path != "/@portal/pending-approvals/apr_1" {
		t.Fatalf("get wire: %+v", calls[0])
	}
	calls = nil
	c.Approvals.Approve(ctx, "apr_1", ApproveInput{Reason: str("ok")})
	if calls[0].Body != `{"reason":"ok"}` {
		t.Fatalf("approve reason wire: %+v", calls[0])
	}
	calls = nil
	c.Approvals.Bulk(ctx, BulkApprovalsInput{Action: "approve", Criteria: map[string]any{}})
	if calls[0].Path != "/@portal/pending-approvals/bulk" || calls[0].Body != `{"action":"approve","criteria":{}}` {
		t.Fatalf("bulk approve wire: %+v", calls[0])
	}
}

func TestApprovalsBulkResultDecodes(t *testing.T) {
	var calls []streamsCall
	c := approvalsClient(t, map[string]any{"attestationId": "apr_9", "action": "reject", "affectedCount": 2, "affectedIds": []any{"apr_a", "apr_b"}, "reason": "stale"}, &calls)
	res, _, kerr := c.Approvals.Bulk(context.Background(), BulkApprovalsInput{Action: "reject", Criteria: map[string]any{"eq": []any{"change.table", "orders"}}, Reason: "stale"})
	if kerr != nil || res.AffectedCount != 2 || res.AffectedIDs[1] != "apr_b" || *res.Reason != "stale" {
		t.Fatalf("Bulk: %+v %v", res, kerr)
	}
}

func TestApprovalsValidation(t *testing.T) {
	ctx := context.Background()
	criteria := map[string]any{"eq": []any{"change.table", "orders"}}
	cases := map[string]func(c *Client) *KError{
		"list limit 0 below":   func(c *Client) *KError { _, _, k := c.Approvals.List(ctx, ListApprovalsOptions{Limit: -1}); return k },
		"list limit above 200": func(c *Client) *KError { _, _, k := c.Approvals.List(ctx, ListApprovalsOptions{Limit: 201}); return k },
		"get empty id":         func(c *Client) *KError { _, _, k := c.Approvals.Get(ctx, ""); return k },
		"approve empty id":     func(c *Client) *KError { _, _, k := c.Approvals.Approve(ctx, "", ApproveInput{}); return k },
		"reject empty reason":  func(c *Client) *KError { _, _, k := c.Approvals.Reject(ctx, "apr_1", RejectInput{}); return k },
		"reject blank reason": func(c *Client) *KError {
			_, _, k := c.Approvals.Reject(ctx, "apr_1", RejectInput{Reason: "  "})
			return k
		},
		"reject empty id": func(c *Client) *KError { _, _, k := c.Approvals.Reject(ctx, "", RejectInput{Reason: "x"}); return k },
		"bulk bad action": func(c *Client) *KError {
			_, _, k := c.Approvals.Bulk(ctx, BulkApprovalsInput{Action: "delete", Criteria: criteria})
			return k
		},
		"bulk nil criteria": func(c *Client) *KError {
			_, _, k := c.Approvals.Bulk(ctx, BulkApprovalsInput{Action: "approve"})
			return k
		},
		"bulk reject no reason": func(c *Client) *KError {
			_, _, k := c.Approvals.Bulk(ctx, BulkApprovalsInput{Action: "reject", Criteria: criteria})
			return k
		},
		"schema limit": func(c *Client) *KError {
			_, _, k := c.Schema.Collections.List(ctx, ListCollectionsOptions{Limit: 500})
			return k
		},
		"schema negative limit": func(c *Client) *KError {
			_, _, k := c.Schema.Collections.List(ctx, ListCollectionsOptions{Limit: -3})
			return k
		},
	}
	for name, run := range cases {
		var calls []streamsCall
		c := approvalsClient(t, nil, &calls)
		wantInvalid(t, name, run(c), calls)
	}
}

func TestApprovalPoliciesWireAndValidation(t *testing.T) {
	ctx := context.Background()
	var calls []streamsCall
	policy := map[string]any{"id": "apo_1", "collection": "orders", "operation": "update", "action": "queue", "predicate": map[string]any{"gt": []any{"change.row.amount", 1000}}, "authoredBy": "usr_1", "createdAt": "t"}
	c := approvalsClient(t, policy, &calls)
	pol, _, kerr := c.ApprovalPolicies.Create(ctx, CreateApprovalPolicyInput{Collection: "orders", Operation: "update", Action: "queue", Predicate: map[string]any{"gt": []any{"change.row.amount", 1000}}})
	if kerr != nil || pol.ID != "apo_1" || pol.Action != "queue" {
		t.Fatalf("Create: %+v %v", pol, kerr)
	}
	if calls[0].Method != "POST" || calls[0].Path != "/@portal/approval-policies" || calls[0].Body != `{"collection":"orders","operation":"update","action":"queue","predicate":{"gt":["change.row.amount",1000]}}` {
		t.Fatalf("create wire: %+v", calls[0])
	}
	calls = nil
	c.ApprovalPolicies.List(ctx)
	if calls[0].Method != "GET" || calls[0].Path != "/@portal/approval-policies" {
		t.Fatalf("list wire: %+v", calls[0])
	}
	calls = nil
	c.ApprovalPolicies.Delete(ctx, "apo_1")
	if calls[0].Method != "DELETE" || calls[0].Path != "/@portal/approval-policies/apo_1" {
		t.Fatalf("delete wire: %+v", calls[0])
	}

	ok := CreateApprovalPolicyInput{Collection: "orders", Operation: "insert", Action: "auto_approve", Predicate: map[string]any{}}
	with := func(f func(*CreateApprovalPolicyInput)) CreateApprovalPolicyInput { in := ok; f(&in); return in }
	bad := map[string]CreateApprovalPolicyInput{
		"empty collection":   with(func(i *CreateApprovalPolicyInput) { i.Collection = "" }),
		"digit first":        with(func(i *CreateApprovalPolicyInput) { i.Collection = "1orders" }),
		"dash in collection": with(func(i *CreateApprovalPolicyInput) { i.Collection = "or-ders" }),
		"too long":           with(func(i *CreateApprovalPolicyInput) { i.Collection = strings.Repeat("a", 64) }),
		"bad operation":      with(func(i *CreateApprovalPolicyInput) { i.Operation = "upsert" }),
		"bad action":         with(func(i *CreateApprovalPolicyInput) { i.Action = "allow" }),
		"nil predicate":      with(func(i *CreateApprovalPolicyInput) { i.Predicate = nil }),
	}
	for name, in := range bad {
		var cc []streamsCall
		cl := approvalsClient(t, nil, &cc)
		_, _, k := cl.ApprovalPolicies.Create(ctx, in)
		wantInvalid(t, name, k, cc)
	}
	// 63 characters is the longest accepted name.
	var cc []streamsCall
	cl := approvalsClient(t, policy, &cc)
	long := ok
	long.Collection = strings.Repeat("a", 63)
	if len(long.Collection) != 63 {
		t.Fatalf("test setup: %d", len(long.Collection))
	}
	if _, _, k := cl.ApprovalPolicies.Create(ctx, long); k != nil || len(cc) != 1 {
		t.Fatalf("63-char collection: %v %d", k, len(cc))
	}
	var dc []streamsCall
	dl := approvalsClient(t, nil, &dc)
	_, _, k := dl.ApprovalPolicies.Delete(ctx, "")
	wantInvalid(t, "delete empty id", k, dc)
}

func TestSchemaCollectionsWire(t *testing.T) {
	var calls []streamsCall
	row := map[string]any{"name": "orders", "rowCount": 120, "rowCountIsExact": true, "columnCount": 8, "indexCount": 3, "lastMigrationSlug": nil, "lastMigrationAppliedAt": nil}
	c := approvalsClient(t, []any{row}, &calls)
	rows, meta, kerr := c.Schema.Collections.List(context.Background(), ListCollectionsOptions{Limit: 2, Cursor: "abc"})
	if kerr != nil || len(rows) != 1 || rows[0].Name != "orders" || rows[0].RowCount != 120 || rows[0].LastMigrationSlug != nil || *meta.Cursor.Next != "dGFza3M" {
		t.Fatalf("List: %+v %v", rows, kerr)
	}
	if calls[0].Method != "GET" || calls[0].Path != "/@portal/schema/collections" || calls[0].Query["limit"] != "2" || calls[0].Query["cursor"] != "abc" {
		t.Fatalf("wire: %+v", calls[0])
	}
	calls = nil
	c.Schema.Collections.List(context.Background(), ListCollectionsOptions{})
	if len(calls[0].Query) != 0 {
		t.Fatalf("default query: %+v", calls[0])
	}
}

func TestApprovalsNeedProjectSlug(t *testing.T) {
	c, kerr := NewClient(ClientOpts{BaseURL: "https://acme.api.k9o.io"})
	if kerr != nil {
		t.Fatal(kerr)
	}
	if _, _, k := c.Approvals.List(context.Background(), ListApprovalsOptions{}); k == nil || k.Code != "K9O_MISSING_PROJECT_CONTEXT" {
		t.Fatalf("got %v", k)
	}
}
