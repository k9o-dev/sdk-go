# k9o.dev/sdk-go

Go SDK for the Kilimanjaro (k9o) platform: a typed client for data, sign-in, storage, functions, AI and more.

Kilimanjaro is an AI-native platform on a Governed Database. Its products are Keep (AI Security and Governance), Pulse (Data Intelligence), Gate (Customer Identity and Access Management) and Kibo (Keys and Secrets Management).

## Contents

- [Links](#links)
- [Install](#install)
- [Quick start](#quick-start)
- [Authentication and project context](#authentication-and-project-context)
- [Running queries](#running-queries)
- [Handling errors](#handling-errors)
- [Regional hosts](#regional-hosts)
- [Appendix](#appendix): [client options](#client-options), [functions](#functions-by-namespace),

## Links

- Website: <https://kilimanjaro.dev>
- Documentation / Developer Hub: <https://docs.kilimanjaro.dev>
- Platform API: <https://api.k9o.io>
- Platform Console: <https://console.k9o.io>
  [error fields](#error-fields), [error codes](#error-codes), [related packages](#related-packages)

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
		fmt.Println(task.ID, task.Title, task.Done)
	}
}
```

```text
tsk_01H... Write the README false
```

Every call returns its result plus a `*k9o.KError`, which is `nil` on success. See [handling errors](#handling-errors).

## Authentication and project context

On the platform, set `OrgSlug`, `ProjectSlug` and `APIKey`, as in the quick start.
Leave `BaseURL` out: the client derives `https://<OrgSlug>.api.k9o.io` for you.

For a custom or self-hosted host, set `BaseURL` instead of `OrgSlug`:

```go
client, kerr := k9o.NewClient(k9o.ClientOpts{
	BaseURL:     "https://k9o.internal.example.com",
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
session, kerr := client.Gate.SignIn(ctx, input)
if kerr != nil {
	fmt.Println(kerr)
	return
}
fmt.Println(session.Data)
```

## Running queries

Insert a row:

```go
newTask := map[string]any{"title": "Write the README", "done": false}
created, kerr := client.From("tasks").Insert(ctx, newTask)
if kerr != nil {
	fmt.Println(kerr)
	return
}
fmt.Println(created.Data)
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
	fmt.Println("second page has", len(secondPage), "rows")
}
```

```text
second page has 10 rows
```

- Filters: `Eq`, `Neq`, `Gt`, `Gte`, `Lt`, `Lte`, `Like`, `ILike`, `Is`, `In`, `NotIn`, `Filter`.
- Other services (storage, functions, AI and more) are listed in [functions by namespace](#functions-by-namespace).

## Handling errors

Failures are returned as `*k9o.KError`. It implements `error`, so `errors.As` works.
Switch on `Code`:

```go
_, _, kerr = k9o.SelectExecute[Task](ctx, client.From("tasks").Select())
if kerr != nil {
	switch kerr.Code {
	case "K9O_TIER_CAP_HIT":
		fmt.Println("plan limit reached:", kerr.Message)
	case "K9O_MISSING_PROJECT_CONTEXT":
		fmt.Println("set ProjectSlug on the client")
	default:
		fmt.Println(kerr)
	}
}
```

Printing a `*KError` gives `[code] message`, plus `(status=N)` when the gateway returned it:

```go
noProject, _ := k9o.NewClient(k9o.ClientOpts{})
_, _, kerr = k9o.SelectExecute[Task](ctx, noProject.From("tasks").Select())
fmt.Println(kerr)
```

```text
[K9O_MISSING_PROJECT_CONTEXT] data-plane operations require ProjectSlug bound on the client
```

Useful fields are `Code`, `Message`, `Recommendation` and `RequestID`.
See [error fields](#error-fields) and [error codes](#error-codes).

## Regional hosts

With `OrgSlug` and the default host, the client sends one unauthenticated `HEAD` request to find your regional host.
It caches the answer and sends all later requests there. No configuration is needed.

## Appendix

### Client options

All fields of `k9o.ClientOpts` are optional.

| Field | What it does |
|---|---|
| `BaseURL` | Gateway origin: scheme, host, optional port. No path, query or fragment. Default `https://api.k9o.io` |
| `APIKey` | Bearer credential sent with each request |
| `OrgSlug` | Organization. With the default host, selects `https://<OrgSlug>.api.k9o.io`. Also sent as `X-K9O-Org-Slug` |
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
Most return the result, a `*Meta` and a `*KError`.

#### Data: `client.From("collection")`

| Function | What it does |
|---|---|
| `Select()` | Start a query. Chain filters, `Limit`, `Cursor` and `Count`, then `k9o.SelectExecute[T]` |
| `Insert` | Insert one row |
| `Update(patch)` | Update one row. Chain `Eq("id", ...)`, then `Execute` |
| `Delete()` | Delete one row. Chain `Eq("id", ...)`, then `Execute` |

#### Gate: `client.Gate`

| Function | What it does |
|---|---|
| `SignUp` | Create an end-user identity |
| `SignIn` | Sign in with email and password |
| `SignOut` | End a session |
| `Whoami` | Return the current identity |
| `RefreshToken` | Exchange a refresh token |
| `VerifySession` | Check the current session |
| `Hosted` | Hosted sign-in URL and relying-party id |

#### Projects: `client.Projects`

| Function | What it does |
|---|---|
| `List` | List projects |
| `Create` | Create a project |
| `Get` | Get a project by slug |
| `Update` | Update a project |

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
| `Deploy` | Deploy a function |
| `List` | List functions |
| `Get` | Get one function |
| `Delete` | Delete a function |
| `Invoke` | Call a function with a JSON payload |

#### AI and compute: `client.AI`, `client.Compute`

| Function | What it does |
|---|---|
| `AI.Embed` | Embed one string or a list of strings |
| `Compute.LLMs.List` | List available models |
| `Compute.LLMs.Read` | Get one model |
| `Compute.LLMs.Invoke` | Run a prompt and get the completion |
| `Compute.LLMs.InvokeStream` | Run a prompt and read tokens from a channel |

#### Studio: `client.Studio`

| Function | What it does |
|---|---|
| `Workflows.List` | List workflows |
| `Workflows.Create` | Create a workflow |
| `Workflows.Read` | Get a workflow |
| `Workflows.Update` | Update a workflow |
| `Workflows.Delete` | Delete a workflow |
| `Workflows.Trigger` | Start a run |
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

#### Realtime: `client.Realtime`

| Function | What it does |
|---|---|
| `Subscribe` | Subscribe to a topic. Events arrive on a channel |
| `Replay` | Fetch past events from a sequence number |

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
| `List` | List bundles |
| `Create` | Create a bundle |
| `Validate` | Check rules without saving |
| `Get` | Get one bundle |
| `Update` | Update a bundle |
| `Delete` | Delete a bundle |
| `Publish` | Make a bundle active |

#### Audit: `client.Audit`

| Function | What it does |
|---|---|
| `Query` | Query the project audit log with `AuditQueryOpts` filters |

#### Organization: `client.Org`, `client.Orgs`

| Function | What it does |
|---|---|
| `Orgs.List` | List organizations |
| `Orgs.Get` | Get an organization by slug |
| `Orgs.Current` | Get the current organization |
| `Orgs.Restore` | Restore an organization |
| `Org.Get` | Get the bound organization |
| `Org.DefaultLLMGet` | Read the default model |
| `Org.DefaultLLMSet` | Set the default model |
| `Org.Members` | `List`, `Invite`, `Get`, `Remove` |
| `Org.Groups` | `List`, `Create`, `Get`, `Delete` |
| `Org.Gate.Roles` | `List`, `Grant`, `Get`, `Revoke` |
| `Org.Gate.IdpProviders` | `List`, `Create`, `Get`, `Configure`, `Delete` |
| `Org.SMTP` | `Get`, `Set`, `Test`, `Unset` |
| `Org.AuditLog.Query` | Query the organization audit log |

#### Current user: `client.Me`

| Function | What it does |
|---|---|
| `Profile` | `Get`, `Update` |
| `Sessions` | `List`, `Revoke`, `RevokeAll` |
| `APITokens` | `List`, `Create`, `Delete` |
| `Notifications` | `List`, `MarkRead`, `MarkAllRead` |
| `AuditLog.Query` | Query your audit log |
| `DataExport` | `Request`, `Status` |

#### Vault secrets: `client.Kibo.Vault.Secrets`

| Function | What it does |
|---|---|
| `List` | List secret metadata, optionally by prefix |
| `Get` | Get one secret's metadata |
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
| `K9O_INVALID_FILTER` | SDK | `Replay` was given a negative sequence |
| `K9O_UPSTREAM_UNAVAILABLE` | SDK | The connection or stream to the gateway failed |
| `K9O_INTERNAL_ERROR` | SDK | The response could not be read |
| `K9O_TIER_CAP_HIT` | Gateway | A plan limit was reached |
| `K9O_PROJECT_NOT_FOUND` | Gateway | The project does not exist |

### Related packages

Two optional modules live in separate Go modules, so the core SDK stays pure Go.
Both use cgo and a native library.

| Module | What it does |
|---|---|
| `k9o.dev/sdk-go/kibo` | Client-side encryption and decryption against your own KMS |
| `k9o.dev/sdk-go/barbican` | Guardrail that inspects text against a signed ruleset |

## License

Apache-2.0 © Formelo Inc. See `LICENSE` and `NOTICE`.
