# Changelog — github.com/k9o/sdk-go

## 1.3.0 — 2026-10-26

Every method now matches the gateway and is checked by a contract test.

### Changed

- `Projects.Create` takes `DisplayName` (required) and optional `Slug`, `Region`, `CustodyMode`. `Get` and `Update` use the projects routes.
- `Functions.Deploy` takes `Name`, `Runtime`, `Tier` (`gateway` or `sandbox`), `ArtifactKey`, `Version` and `Description`. Inline code is gone.
- `Functions.Invoke` sends `{payload}` and accepts `InvokeOptions`. `Functions.List` accepts `Limit`, `Cursor`, `Status`, `Source`, `Tier`.
- Policies `Create`, `Update` and `Validate` take a `Policy` document. `Policies.List` takes only `status`.
- `Audit.Query` filters are `EventType`, `ActorID`, `After`, `Before`, `Limit`, `Cursor`.
- Role grants use `/role-grants` and identity providers use `/identity-providers`, with typed create and update inputs.
- `Org.DefaultLLMGet` and `DefaultLLMSet` return records. `Org.SMTP.Set` takes `PasswordRef` and `FromAddress`; `Test` takes `to` and `subject`.
- `Me.APITokens.Create` takes `Label`, `ScopeManifest` and `TTLSeconds`. `Me.Sessions.Revoke` and `RevokeAll` use the session routes.
- `Compute.LLMs.Invoke` parameters are flat and the result has `Content` and `Usage`. Stream chunks are OpenAI-style.
- `Studio.Workflows.Trigger` sends `{triggerContext}`. `Gate.SignOut` requires the refresh token.
- `Kibo.Vault.Secrets.Get` looks up a secret by exact name; the region is optional.
- The API host is read from the base URL: any `<org>.api.<apex>` host works and no domain is built in.

### Removed

- `Gate.VerifySession`, `Gate.Hosted`, `Realtime.Replay` and the polling transport.
- `Me.Notifications`, `Me.AuditLog`, `Me.DataExport`.
- `Org.Groups.Create`, `Org.Groups.Delete`, `Org.Members.Get`, `Org.SMTP.Get`.
- The `modality` filter of `Compute.LLMs.List` and the `triggerType` filter of `Studio.Workflows.List`.

### Added

- `Functions.UploadArtifact(ctx, bytes, "wasm" | "tar")` returns the `ArtifactKey` for `Deploy`.

## 1.2.0 — 2026-10-25

- The README now introduces the Kilimanjaro products (Keep, Pulse, Gate, Kibo). No change to the public API.

## 1.1.1 — 2026-10-25

- Documentation links corrected: the documentation hub is https://docs.kilimanjaro.dev, with the website, platform API and console listed in a Links section. Package metadata points at the same places.

## 1.1.0 — 2026-10-25

- Documentation rewritten with runnable examples and expected output; internal cross-references removed from the published sources. No change to the public API.
- Error text for `update()` / `delete()` without an `id` filter no longer mentions a version or a roadmap.

## 1.0.0 — 2026-10-23

First public release. See the README for the full API overview.
