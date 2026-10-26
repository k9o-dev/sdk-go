# k9o.dev/sdk-go

Client for Kilimanjaro. Read and write your data, store files, manage policies and migrations,
deploy and invoke agents or functions, and publish to and subscribe to streams and realtime events.

## Contents

- [Links](#links)
- [Install](#install)
- [Quick start](#quick-start)
- [Authentication and project context](#authentication-and-project-context)
- [Running queries](#running-queries)
- [Handling errors](#handling-errors)
- [Deploying a function](#deploying-a-function)
- [Streams](#streams)
- [Approvals](#approvals)
- [Approval policies](#approval-policies)
- [Schema collections](#schema-collections)
- [Query with SQL](#query-with-sql)
- [Search](#search)
- [Generate text](#generate-text)
- [Regional hosts](#regional-hosts)
- [Appendix](#appendix): [client options](#client-options), [functions](#functions-by-namespace),
  [error fields](#error-fields), [error codes](#error-codes), [related packages](#related-packages),
  [about Kilimanjaro](#about-kilimanjaro)

## Links

- Website: <https://kilimanjaro.dev>
- Documentation / Developer Hub: <https://docs.kilimanjaro.dev>
- Platform API: <https://api.k9o.io>
- Platform Console: <https://console.k9o.io>

## Install

```bash
go get k9o.dev/sdk-go
```

- Requires Go 1.22 or newer.
- The core module uses only the standard library.

## Quick start

The examples on this page use a `tasks` collection with a `title` and a `done` field.

```go
package main

import (
	"context"
	"fmt"
	"os"

	k9o "k9o.dev/sdk-go"
)

type Task struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

func main() {
	ctx := context.Background()

	client, kerr := k9o.NewClient(k9o.ClientOpts{
		APIKey:      os.Getenv("K9O_API_KEY"),
		OrgSlug:     "acme",
		ProjectSlug: "todo-app",
	})
	if kerr != nil {
		fmt.Println(kerr)
		os.Exit(1)
	}

	query := client.From("tasks").Select().Eq("done", false).Limit(10)
	tasks, _, kerr := k9o.SelectExecute[Task](ctx, query)
	if kerr != nil {
		fmt.Println(kerr)
		os.Exit(1)
	}

	for _, task := range tasks {
		fmt.Printf("Output: %s %s %v\n", task.ID, task.Title, task.Done)
	}
}
```

```text
Output: tsk_01H... Write the README false
```

Every call returns its result plus a `*k9o.KError`, which is `nil` on success. See [handling errors](#handling-errors).

## Authentication and project context

On the platform, set `OrgSlug`, `ProjectSlug` and `APIKey`, as in the quick start.
Leave `BaseURL` out: the client builds the organization host from `OrgSlug` on the default host.

For a self-hosted or internal gateway, set `BaseURL` to its organization host, such as
`https://acme.api.corp.example`. Any `<org>.api.<apex>` host works, and no domain is built in.
Other hosts, such as `localhost`, are used exactly as given:

```go
client, kerr := k9o.NewClient(k9o.ClientOpts{
	BaseURL:     "https://acme.api.corp.example",
	ProjectSlug: "todo-app",
	APIKey:      os.Getenv("K9O_API_KEY"),
})
```

- The SDK does not read environment variables. Pass your key in, as shown.
- `ProjectSlug` is required for data, sign-in, storage and other project calls.
- Failed idempotent requests are retried on transient errors. See [client options](#client-options).
- Every call takes a `context.Context` first, so cancellation and deadlines work as usual.

End users sign in through Gate:

```go
input := k9o.SignInInput{Email: "user@example.com", Password: "correct horse battery staple"}
_, kerr = client.Gate.SignIn(ctx, input)
if kerr != nil {
	fmt.Println(kerr)
	return
}
fmt.Println("Output: signed in")
```

```text
Output: signed in
```

## Running queries

Insert a row:

```go
newTask := map[string]any{"title": "Write the README", "done": false}
_, kerr = client.From("tasks").Insert(ctx, newTask)
if kerr != nil {
	fmt.Println(kerr)
	return
}
fmt.Println("Output: created")
```

```text
Output: created
```

Update and delete both need an `id` filter:

```go
patch := map[string]any{"done": true}
_, kerr = client.From("tasks").Update(patch).Eq("id", "tsk_01H...").Execute(ctx)
if kerr != nil {
	fmt.Println(kerr)
	return
}

_, kerr = client.From("tasks").Delete().Eq("id", "tsk_01H...").Execute(ctx)
if kerr != nil {
	fmt.Println(kerr)
	return
}
```

A select ends with the package-level helper `k9o.SelectExecute[YourRowType]`, because Go methods cannot take type parameters.
`meta.Cursor.Next` is a `*string`; it is `nil` when there is no further page:

```go
firstQuery := client.From("tasks").Select().Limit(10)
_, meta, kerr := k9o.SelectExecute[Task](ctx, firstQuery)
if kerr != nil {
	fmt.Println(kerr)
	return
}

if meta.Cursor != nil && meta.Cursor.Next != nil {
	secondQuery := client.From("tasks").Select().Limit(10).Cursor(*meta.Cursor.Next)
	secondPage, _, kerr := k9o.SelectExecute[Task](ctx, secondQuery)
	if kerr != nil {
		fmt.Println(kerr)
		return
	}
	fmt.Printf("Output: second page has %d rows\n", len(secondPage))
}
```

```text
Output: second page has 10 rows
```

- Filters: `Eq`, `Neq`, `Gt`, `Gte`, `Lt`, `Lte`, `Like`, `ILike`, `Is`, `In`, `NotIn`, `Filter`.
- Other services (storage, functions, AI and more) are listed in [functions by namespace](#functions-by-namespace).

## Deploying a function

Upload the compiled artifact first. `UploadArtifact` returns the `ArtifactKey` that `Deploy` needs.
The kind is `"tar"` (an OCI image archive of the function's container image, up to 256 MiB) and the tier is `sandbox`. The `gateway` tier is for functions owned by platform operators and is not available to most projects. Sandbox functions run in a [Firelock](https://github.com/firelock-sandbox/firelock) microVM sandbox.

```go
archive, err := os.ReadFile("hello.tar")
if err != nil {
	fmt.Println(err)
	return
}
artifact, _, kerr := client.Functions.UploadArtifact(ctx, archive, "tar")
if kerr != nil {
	fmt.Println(kerr)
	return
}

_, _, kerr = client.Functions.Deploy(ctx, k9o.DeployFunctionInput{
	Name:        "hello",
	Runtime:     "rust",
	Tier:        "sandbox",
	ArtifactKey: artifact.ArtifactKey,
	Version:     "1.0.0",
})
if kerr != nil {
	fmt.Println(kerr)
	return
}

result, _, kerr := client.Functions.Invoke(ctx, "hello", map[string]any{"name": "Ada"})
if kerr != nil {
	fmt.Println(kerr)
	return
}
fmt.Println("Output:", result)
```

```text
Output: map[greeting:Hello, Ada]
```

`Invoke` sends your payload as `{payload}`. Pass `k9o.InvokeOptions` as a last argument for `Background` and similar controls.

## Handling errors

Failures are returned as `*k9o.KError`. It implements `error`, so `errors.As` works.
Switch on `Code`:

```go
_, _, kerr = k9o.SelectExecute[Task](ctx, client.From("tasks").Select())
if kerr != nil {
	switch kerr.Code {
	case "K9O_TIER_CAP_HIT":
		fmt.Println("Output: plan limit reached:", kerr.Message)
	case "K9O_MISSING_PROJECT_CONTEXT":
		fmt.Println("Output: set ProjectSlug on the client")
	default:
		fmt.Println("Output:", kerr)
	}
}
```

Printing a `*KError` gives `[code] message`, plus `(status=N)` when the gateway returned it:

```go
noProject, _ := k9o.NewClient(k9o.ClientOpts{})
_, _, kerr = k9o.SelectExecute[Task](ctx, noProject.From("tasks").Select())
fmt.Println("Output:", kerr)
```

```text
Output: [K9O_MISSING_PROJECT_CONTEXT] data-plane operations require ProjectSlug bound on the client
```

Useful fields are `Code`, `Message`, `Recommendation` and `RequestID`.
See [error fields](#error-fields) and [error codes](#error-codes).

## Streams

Create a stream, publish an event and read it back live. `maxBytes` is the storage ceiling in bytes: 1, 2, 4, 8, 16, 32 or 64 GiB.

```go
stream, _, kerr := client.Streams.Create(ctx, "Orders", 1<<30, 0)
if kerr != nil {
	fmt.Println(kerr)
	return
}

_, _, kerr = client.Streams.Publish(ctx, stream.Slug, map[string]any{"id": 1, "total": 42}, "")
if kerr != nil {
	fmt.Println(kerr)
	return
}

subCtx, cancel := context.WithCancel(ctx)
defer cancel()
events, errs := client.Streams.Subscribe(subCtx, stream.Slug, "")
for event := range events {
	fmt.Printf("Output: %v %s\n", event.Sequence, string(event.Data))
	cancel()
}
if kerr := <-errs; kerr != nil {
	fmt.Println(kerr)
}
```

```text
Output: 1 {"id":1,"total":42}
```

- `Subscribe` delivers events on a channel and stops when the context is cancelled. Pass a sequence as the last argument to resume after it.
- `Replay` reads a bounded range in one call: `client.Streams.Replay(ctx, slug, k9o.ReplayOptions{Limit: 100})`.
- The server closes each live connection after a short time.
  `Subscribe` reconnects from the last sequence it delivered, so no event is repeated or skipped.
- Publishing needs the project admin role. Subscribing and replaying need admin or auditor.
- While streams are not available, calls fail with `K9O_STREAM_UNAVAILABLE` (HTTP 503).
  A rejected request, such as a bad `subject`, fails with `K9O_STREAM_INVALID`.

## Approvals

Writes sent with `requireApproval` can wait in a queue for a second person. `client.Approvals` reads that queue and decides its changes.
Project admins and auditors see every change; other members see only the ones they proposed.

```go
queue, _, kerr := client.Approvals.List(ctx, k9o.ListApprovalsOptions{Status: "pending_approval", Limit: 20})
if kerr != nil {
	fmt.Println(kerr)
	return
}
change := queue[0]
fmt.Printf("Output: %s %s %s\n", change.ID, change.Operation, change.Collection)

reason := "checked the amount"
decided, _, kerr := client.Approvals.Approve(ctx, change.ID, k9o.ApproveInput{Reason: &reason})
if kerr != nil {
	fmt.Println(kerr)
	return
}
fmt.Printf("Output: %s\n", decided.Status)
```

```text
Output: apr_01j9x3 update orders
Output: redeemed
```

- A change is `pending_approval`, `approved`, `rejected`, `expired` or `redeemed` (applied). An `expired` change can still read `pending_approval` until someone decides it.
- `Approve` can answer 200 with `Status` still `pending_approval`. That means your approval is recorded and more approvers are needed; the change is not applied yet.
- `Reject(ctx, id, k9o.RejectInput{Reason: ...})` needs a non-empty reason; the SDK checks it before sending.
- `List` is not paginated: it returns the newest `Limit` changes (1 to 200, default 50). `Get(ctx, id)` reads one.
- Deciding needs the queue-admin role (project admin or auditor).

Decide many at once with `Bulk`. `Criteria` is a predicate over each pending change (see [Approval policies](#approval-policies)):

```go
run, _, kerr := client.Approvals.Bulk(ctx, k9o.BulkApprovalsInput{
	Action:   "reject",
	Criteria: map[string]any{"eq": []any{"change.table", "orders"}},
	Reason:   "stale",
})
if kerr != nil {
	fmt.Println(kerr)
	return
}
fmt.Printf("Output: %d %s\n", run.AffectedCount, strings.Join(run.AffectedIDs, ","))
```

```text
Output: 2 apr_01j9x3,apr_01j9x4
```

- A row that fails (a conflict, an apply error) is skipped silently and left out of `AffectedIDs`. Compare `AffectedIDs` with the set you intended.
- `Reject` needs a `Reason`; `Criteria` must not be nil. `AttestationID` identifies the batch, even when nothing matched.
- It considers the newest 10000 pending changes. Bulk approve is not guaranteed to apply every matched row.

## Approval policies

A policy decides what happens to writes on one collection that match a predicate: `queue` them for approval, or `auto_approve` them.
For a write the first matching policy, oldest first, decides. Policies apply to writes sent with `requireApproval`.

```go
policy, _, kerr := client.ApprovalPolicies.Create(ctx, k9o.CreateApprovalPolicyInput{
	Collection: "orders",
	Operation:  "update",
	Action:     "queue",
	Predicate:  map[string]any{"gt": []any{"change.row.amount", 1000}},
})
if kerr != nil {
	fmt.Println(kerr)
	return
}
fmt.Printf("Output: %s %s\n", policy.ID, policy.Action)

removed, _, kerr := client.ApprovalPolicies.Delete(ctx, policy.ID)
if kerr != nil {
	fmt.Println(kerr)
	return
}
fmt.Printf("Output: %s\n", removed.ID)
```

```text
Output: apo_01j9x5 queue
Output: apo_01j9x5
```

- `Operation` is `insert`, `update` or `delete`; `Action` is `auto_approve` or `queue`.
- `Collection` matches `^[a-zA-Z_][a-zA-Z0-9_]{0,62}$` and `Predicate` must not be nil. The SDK checks both before sending.
- A predicate has one operator key: `and`, `or`, `not`, `eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `in`, `not_in`, `in_cidr`, `not_in_cidr` or `contains`.
  `and` and `or` take a list of predicates, `not` takes one, and a comparison takes `[left, right]`.
- Operands start with `maker.` or `change.` (for example `change.row.amount`, `change.table`, `change.operation`, `maker.id`).
- `List(ctx)` returns all policies. Identical policies are not de-duplicated. Creating and deleting need project admin or author.

## Schema collections

`client.Schema.Collections.List` lists the project's collections by name, with their size and last migration.

```go
page, meta, kerr := client.Schema.Collections.List(ctx, k9o.ListCollectionsOptions{Limit: 2})
if kerr != nil {
	fmt.Println(kerr)
	return
}
for _, c := range page {
	fmt.Printf("Output: %s %d rows, %d columns\n", c.Name, c.RowCount, c.ColumnCount)
}
fmt.Printf("Output: next %s\n", *meta.Cursor.Next)
```

```text
Output: orders 120 rows, 8 columns
Output: tasks 14 rows, 4 columns
Output: next dGFza3M
```

- Pass `meta.Cursor.Next` as `Cursor` for the next page; it is nil on the last page. `Limit` is 1 to 200 (default 50).
- A row has `Name`, `RowCount`, `RowCountIsExact`, `ColumnCount`, `IndexCount`, `LastMigrationSlug` and `LastMigrationAppliedAt`.
- `RowCount` is a live count that skips soft-deleted rows and ignores row-level security. Without an `org_id` claim on the token, `meta.Warnings` says so.
- Project admins, auditors, editors, members and viewers can read it.

## Query with SQL

`Query` runs one SQL statement and returns the rows. Positional parameters bind to `$1`, `$2` and so on:

```go
rows, meta, kerr := client.Query(ctx, "SELECT title FROM tasks WHERE done = $1 LIMIT 2", []any{false})
if kerr != nil {
	fmt.Println(kerr)
	return
}
fmt.Printf("Output: %d rows, first is %v\n", meta.RawQuery.RowCount, rows[0]["title"])
```

```text
Output: 2 rows, first is Write the README
```

`QueryNamed` takes a map and rewrites each `:name` placeholder to `$n`. A name used twice is bound once.
Quoted text, comments, dollar-quoted strings and `::` casts are left alone:

```go
rows, _, kerr := client.QueryNamed(ctx,
	"SELECT title FROM tasks WHERE data->>'owner' = :owner AND created_at > :since::timestamptz",
	map[string]any{"owner": "ada", "since": "2026-01-01"})
if kerr != nil {
	fmt.Println(kerr)
	return
}
fmt.Printf("Output: %d rows\n", len(rows))
```

```text
Output: 3 rows
```

- Queries are read-only. A write is refused with `K9O_RAW_SQL_READ_ONLY`; change data through `From(collection)`.
- One statement per call, at most 100000 characters of SQL and 1000 parameters. Row-level security applies.
  The caller needs the project-editor role. `dataset.<name>` reads an analytics dataset.
- Values are a string, a number, a bool or `nil`. Serialise anything else (for example as JSON text) first.
  A placeholder with no key in the map fails with `K9O_INVALID_FIELD_VALUE` before any request.
- `k9o.QueryInto[T]` and `k9o.QueryNamedInto[T]` decode rows into your own type.

## Search

`Search` finds the best matches for some text in one text column of a collection. `Column` and `Q` are required;
`Language` (default `english`), `Type` (`plain`, `phrase` or `websearch`), `Limit` and `Cursor` are optional.

```go
rows, meta, kerr := client.Search(ctx, "articles", k9o.SearchInput{Column: "body", Q: "solar panels", Limit: 2})
if kerr != nil {
	fmt.Println(kerr)
	return
}
fmt.Printf("Output: best match is %v\n", rows[0]["title"])
if meta.Cursor != nil && meta.Cursor.Next != nil {
	fmt.Println("Output: more results")
}
```

```text
Output: best match is Rooftop solar guide
Output: more results
```

On the select builder, `TextSearch` and `VectorSearch` combine with filters, `Limit` and `Cursor`.
`VectorSearch` returns the rows nearest to an embedding, closest first.
`Metric` is `cosine` (default), `euclidean` or `dot`; `Limit` is 1 to 1000 (default 10):

```go
embedding := []float64{0.12, -0.4, 0.33}
q := client.From("articles").Select().Eq("status", "published").
	VectorSearch("embedding", embedding, k9o.VectorSearchOptions{Metric: "cosine", Limit: 3})
rows, _, kerr := k9o.SelectExecute[map[string]any](ctx, q)
if kerr != nil {
	fmt.Println(kerr)
	return
}
fmt.Printf("Output: %d nearest\n", len(rows))
```

```text
Output: 3 nearest
```

An invalid column name, empty query, unknown option or out-of-range limit fails with `K9O_INVALID_FIELD_VALUE`
when the query runs, before any request is sent.

## Generate text

`AI.Generate` runs a prompt through an LLM. Only `prompt` is required.

```go
res, _, kerr := client.AI.Generate(ctx, "Say hello in French.", k9o.GenerateOptions{MaxTokens: 20})
if kerr != nil {
	fmt.Println(kerr)
	return
}
fmt.Printf("Output: %s\n", res.Content)
fmt.Printf("Output: %d tokens\n", res.Usage.TotalTokens)
```

```text
Output: Bonjour !
Output: 14 tokens
```

- `GenerateOptions` has `Model`, `MaxTokens`, `Temperature` and `UsageType`. Unset fields are not sent.
- `Model` is optional: the org's default LLM is used. With no default the call fails with HTTP 400.
- Models from an outside provider answer 501 `K9O_NOT_IMPLEMENTED` until they are built.

## Regional hosts

With an organization host (`<org>.api.<apex>`), the client sends one unauthenticated `HEAD` request to find your regional host.
It caches the answer and sends all later requests there. No configuration is needed.
The regional host must share the organization, family and apex of the host you configured.

## Appendix

### Client options

All fields of `k9o.ClientOpts` are optional.

| Field | What it does |
|---|---|
| `BaseURL` | Gateway origin: scheme, host, optional port. No path, query or fragment. Default `https://api.k9o.io` |
| `APIKey` | Bearer credential sent with each request |
| `OrgSlug` | Organization. With a platform host (`api.<apex>`), builds `<OrgSlug>.api.<apex>`. Also sent as `X-K9O-Org-Slug` |
| `ProjectSlug` | Project. Sent as `X-K9O-Project-Slug` and used in project paths |
| `DefaultHeaders` | Extra headers added to every request |
| `HTTPClient` | Your own `*http.Client` for timeouts or custom transports |
| `RetryConfig` | A `*k9o.RetryConfig`. Default: 3 retries, exponential backoff |

`RetryConfig` fields:

| Field | What it does |
|---|---|
| `MaxRetries` | Number of retries (default 3) |
| `Backoff` | `"exponential"`, `"linear"` or `"none"` |
| `InitialDelayMs` | First delay (default 100) |
| `MaxDelayMs` | Delay cap (default 5000) |

Retries apply to `GET`, `HEAD`, `OPTIONS` and `DELETE` only. Triggers are transport errors, 429, 502, 503 and 504.

### Functions by namespace

Each namespace is a field on `Client`. Every function takes a `context.Context` first.
Most return the result, a `*Meta` and a `*KError`. Gate functions return the result and a `*KError`.

#### Data: `client.From("collection")`

| Function | What it does |
|---|---|
| `Select()` | Start a query. Chain filters, `Limit`, `Cursor` and `Count`, then `k9o.SelectExecute[T]` |
| `Insert` | Insert one row |
| `Update(patch)` | Update one row. Chain `Eq("id", ...)`, then `Execute` |
| `Delete()` | Delete one row. Chain `Eq("id", ...)`, then `Execute` |
| `Select().TextSearch(column, text, opts)` | Full-text search on a text column |
| `Select().VectorSearch(column, embedding, opts)` | Nearest rows to an embedding, closest first |

#### SQL and search: `client.Query`, `client.Search`

| Function | What it does |
|---|---|
| `Query(sql, []any)` | Run one read-only statement with positional `$n` parameters |
| `QueryNamed(sql, map[string]any)` | Same, with `:name` placeholders |
| `Search(collection, SearchInput)` | Full-text search, best matches first |

#### Gate: `client.Gate`

| Function | What it does |
|---|---|
| `SignUp` | Create an end-user identity |
| `SignIn` | Sign in with email and password |
| `SignOut` | End a session. Needs the `RefreshToken` |
| `Whoami` | Return the current identity |
| `RefreshToken` | Exchange a refresh token |

#### Projects: `client.Projects`

| Function | What it does |
|---|---|
| `List` | List projects |
| `Create` | Create a project. `DisplayName` is required; `Slug`, `Region`, `CustodyMode` are optional |
| `Get` | Get a project by slug |
| `Update` | Update a project, for example its `DisplayName` |

#### Storage: `client.Storage`

| Function | What it does |
|---|---|
| `ListBuckets` | List buckets |
| `GetBucket` | Get one bucket |
| `CreateBucket` | Create a bucket |
| `UpdateBucket` | Change bucket settings |
| `DeleteBucket` | Delete a bucket |
| `From(bucket).Upload` | Upload an object |
| `From(bucket).CreateSignedURL` | Create a time-limited URL |
| `From(bucket).List` | List objects by prefix |
| `From(bucket).Remove` | Delete an object |

#### Functions: `client.Functions`

| Function | What it does |
|---|---|
| `UploadArtifact` | Upload `"wasm"` or `"tar"` bytes. Returns the `ArtifactKey` |
| `Deploy` | Deploy from an artifact: `Name`, `Runtime`, `Tier`, `ArtifactKey`, `Version` |
| `List` | List functions. `ListFunctionsOptions`: `Limit`, `Cursor`, `Status`, `Source`, `Tier` |
| `Get` | Get one function |
| `Delete` | Delete a function |
| `Invoke` | Call a function. Sends `{payload}`; optional `InvokeOptions` |

#### AI and compute: `client.AI`, `client.Compute`

| Function | What it does |
|---|---|
| `AI.Embed` | Embed one string or a list of strings |
| `AI.Generate` | Run a prompt with the default or a named LLM. Returns `Content` and `Usage` |
| `Compute.LLMs.List` | List available models |
| `Compute.LLMs.Read` | Get one model |
| `Compute.LLMs.Invoke` | Run a prompt. The result has `Content` and `Usage` |
| `Compute.LLMs.InvokeStream` | Run a prompt. Chunks are OpenAI-style: `Choices[0].Delta.Content` |

#### Studio: `client.Studio`

| Function | What it does |
|---|---|
| `Workflows.List` | List workflows |
| `Workflows.Create` | Create a workflow |
| `Workflows.Read` | Get a workflow |
| `Workflows.Update` | Update a workflow |
| `Workflows.Delete` | Delete a workflow |
| `Workflows.Trigger` | Start a run. Sends `{triggerContext}` |
| `Workflows.Runs.List` | List runs of a workflow |
| `Workflows.Runs.Read` | Get one run |
| `Workflows.Runs.Cancel` | Cancel a run |
| `Dashboards.List` | List dashboards |
| `Dashboards.Create` | Create a dashboard |
| `Dashboards.Read` | Get a dashboard |
| `Dashboards.Update` | Update a dashboard |
| `Dashboards.Delete` | Delete a dashboard |
| `Dashboards.Pin` | Pin a dashboard |
| `Dashboards.Unpin` | Unpin a dashboard |

#### Streams: `client.Streams`

| Function | What it does |
|---|---|
| `Create(ctx, displayName, maxBytes, retentionSeconds)` | Create a stream. A `retentionSeconds` of `0` uses the default |
| `List` | List streams |
| `Get(ctx, slug)` | Get one stream |
| `Delete(ctx, slug)` | Retire a stream |
| `Publish(ctx, slug, data, subject)` | Append an event. `subject` is an optional routing suffix; pass `""` to omit it |
| `Replay(ctx, slug, ReplayOptions)` | Read a range: `FromSequence`, `UntilSequence`, `Limit`. The result has `Events`, `NextCursor`, `HasMore` |
| `Subscribe(ctx, slug, fromSequence)` | Live events on a channel (`Sequence`, `Subject`, `Data`, `PublishedAt`). Reconnects from the last sequence |

#### Approvals: `client.Approvals`, `client.ApprovalPolicies`, `client.Schema`

| Function | What it does |
|---|---|
| `Approvals.List(ctx, ListApprovalsOptions)` | Pending changes, newest first: `Status`, `Limit` (1 to 200). Not paginated |
| `Approvals.Get(ctx, id)` | One pending change |
| `Approvals.Approve(ctx, id, ApproveInput)` | Approve. `Status` may stay `pending_approval` while more approvers are needed |
| `Approvals.Reject(ctx, id, RejectInput)` | Reject. `Reason` is required |
| `Approvals.Bulk(ctx, BulkApprovalsInput)` | Approve or reject every change matching `Criteria`. Compare `AffectedIDs` with what you intended |
| `ApprovalPolicies.List` | All approval policies, oldest first |
| `ApprovalPolicies.Create(ctx, CreateApprovalPolicyInput)` | Create a policy: `Collection`, `Operation`, `Action`, `Predicate` |
| `ApprovalPolicies.Delete(ctx, id)` | Delete a policy |
| `Schema.Collections.List(ctx, ListCollectionsOptions)` | Collections by name: `Limit`, `Cursor`. Cursor-paginated |

#### Realtime: `client.Realtime`

| Function | What it does |
|---|---|
| `Subscribe` | Subscribe to a topic. Events arrive on a channel (`Topic`, `Type`, `Sequence`, `Payload`, `OccurredAt`) |

There is no replay. A `resync` control frame arrives as an event whose `EventType` is empty.
When you see one, re-read current state and subscribe again.

#### Migrations: `client.Migrations`

| Function | What it does |
|---|---|
| `Apply` | Apply migrations now |
| `List` | List applied migrations |
| `Get` | Get one applied migration by version |
| `Queue` | Queue migrations for approval |
| `ListPending` | List queued migrations |
| `ApplyPending` | Apply a queued migration |
| `RejectPending` | Reject a queued migration |
| `DeletePending` | Remove a queued migration |

#### Policies: `client.Policies`

| Function | What it does |
|---|---|
| `Current` | Get the active policy bundle |
| `List` | List bundles, filtered by `status` only |
| `Create` | Create a bundle from a `Policy` document (plus optional `Rules`, `EditorMode`) |
| `Validate` | Check a policy document without saving |
| `Get` | Get one bundle |
| `Update` | Update a bundle |
| `Delete` | Delete a bundle |
| `Publish` | Make a bundle active |

#### Audit: `client.Audit`

| Function | What it does |
|---|---|
| `Query` | Filters: `EventType`, `ActorID`, `After`, `Before`, `Limit`, `Cursor` |

#### Organization: `client.Org`, `client.Orgs`

| Function | What it does |
|---|---|
| `Orgs.List` | List organizations |
| `Orgs.Get` | Get an organization by slug |
| `Orgs.Current` | Get the current organization |
| `Orgs.Restore` | Restore an organization |
| `Org.Get` | Get the bound organization |
| `Org.DefaultLLMGet` | Read the default model. Returns a record, not a string |
| `Org.DefaultLLMSet` | Set the default model |
| `Org.Members` | `List`, `Invite`, `Remove` |
| `Org.Groups` | `List`, `Get` |
| `Org.Gate.Roles` | Role grants: `List`, `Grant`, `Get`, `Revoke` |
| `Org.Gate.IdpProviders` | Identity providers: `List`, `Create`, `Get`, `Configure`, `Delete` |
| `Org.SMTP` | `Set` (`PasswordRef`, `FromAddress`), `Test` (`to`, `subject`), `Unset` |
| `Org.AuditLog.Query` | Query the organization audit log (`Type`, `Limit`) |

#### Current user: `client.Me`

| Function | What it does |
|---|---|
| `Profile` | `Get`, `Update` |
| `Sessions` | `List`, `Revoke`, `RevokeAll` |
| `APITokens` | `List`, `Create` (`Label`, `ScopeManifest`, `TTLSeconds`), `Delete` |

#### Vault secrets: `client.Kibo.Vault.Secrets`

| Function | What it does |
|---|---|
| `List` | List secret metadata, optionally by prefix |
| `Get` | Get one secret's metadata by exact name. The region argument may be empty |
| `Create` | Seal and store a secret |
| `Update` | Store a new value |
| `Rotate` | Rotate the wrapping key |
| `Move` | Rename a secret path |
| `Delete` | Delete a secret |
| `RevealChallenge` | Get a single-use challenge for a reveal |
| `Reveal` | Read the plaintext using that challenge |

### Error fields

| Field | Type | Meaning |
|---|---|---|
| `Code` | `string` | Stable error code, for example `K9O_TIER_CAP_HIT` |
| `Message` | `string` | Human-readable explanation |
| `Context` | `map[string]any` | Structured details for the code |
| `Recommendation` | `string` | Suggested next step, may be empty |
| `Status` | `int` | HTTP status, or `0` for errors raised in the SDK |
| `RequestID` | `string` | Gateway trace id |

### Error codes

The gateway can return other codes. They are passed through unchanged.

| Code | Raised by | When |
|---|---|---|
| `K9O_INVALID_BASE_URL` | SDK | `BaseURL` is not a valid origin |
| `K9O_INVALID_FIELD_VALUE` | SDK | Bad slug, collection name or input, or a missing `id` filter |
| `K9O_MISSING_PROJECT_CONTEXT` | SDK | A project call was made without `ProjectSlug` |
| `K9O_UPSTREAM_UNAVAILABLE` | SDK | The connection or stream to the gateway failed |
| `K9O_INTERNAL_ERROR` | SDK | The response could not be read |
| `K9O_TIER_CAP_HIT` | Gateway | A plan limit was reached |
| `K9O_PROJECT_NOT_FOUND` | Gateway | The project does not exist |
| `K9O_STREAM_UNAVAILABLE` | Gateway | Streams are not available yet; retry later |
| `K9O_STREAM_INVALID` | Gateway | A stream request was rejected, for example a bad `subject` |

### Related packages

Two optional modules live in separate Go modules, so the core SDK stays pure Go.
Both use cgo and a native library.

| Module | What it does |
|---|---|
| `k9o.dev/sdk-go/kibo` | Client-side encryption and decryption against your own KMS |
| `k9o.dev/sdk-go/barbican` | Guardrail that inspects text against a signed ruleset |

### About Kilimanjaro

Kilimanjaro is a modular developer platform for building secure enterprise applications.
You define AI agents, functions and data pipelines on the platform.
The SDKs let you deploy and invoke them, read and write your data, publish to and subscribe to streams, sign users in,
and encrypt with keys you hold.

It provides four building blocks:

- **Keep**: AI security and governance. Guardrails and policy for the agents you run.
- **Pulse**: data intelligence and real-time stream processing. Streams, analytics and live feeds.
- **Gate**: customer identity and access management. Sign-in and sessions, and automatic data scoping
  so a signed-in user reaches only their own data.
- **Kibo**: cryptographic keys and secrets management. Client-side encryption with keys you hold, and a secrets vault.

## License

Apache-2.0 © Formelo Inc. See `LICENSE` and `NOTICE`.
