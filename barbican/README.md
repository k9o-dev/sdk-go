# k9o.dev/sdk-go/barbican

Barbican for Go: the free, embeddable L2 guardrail (input normalization, deny-list matching over a signed ruleset,
local verdict, streaming output redactor). Advisory / defense-in-depth, never the sandbox boundary.

All logic and all crypto live in the audited Rust engine (`k9o-keep-core`, uniffi crate `ffi-uniffi`). This package
is a thin wrapper over the `uniffi-bindgen-go` cgo binding. It holds no key and takes no verifier argument:
`LoadRulesetSigned` hands the blob to the engine, which verifies it under a key from a root-signed key manifest.
Refusals are the engine's closed reason tokens in `Error.Reason`, never invented text.

```go
b, err := barbican.New() // no network; *Error{Code: CodeEngineMissing} if the engine is not available
if err != nil { /* barbican.IsEngineMissing(err) */ }
defer b.Close()
b.LoadKeyManifest(jws, uint64(time.Now().Unix()))
_, err = b.LoadRulesetSigned(barbican.SignedRuleset{...}) // *Error{Code: CodeRulesetInvalid, Reason: "unknown_kid", Stage: "ruleset"}
v := b.Inspect(text, nil)
```

## Nested module + native engine

- `k9o_keep_core_ffi_uniffi/*.go|.h` is the COMMITTED `uniffi-bindgen-go` output (pinned `0.4.0+v0.28.3`, uniffi
  0.28). CI regenerates from the release cdylib and fails on any diff.
- The native library is CI-injected (`gen-cgo-shims.sh` writes per-platform static-link shims) and NEVER committed.
  A build made with `--features test-trust-root` embeds the published TEST root public key; only the shared KAT
  test may link it. `gen-cgo-shims.sh <mod> <artifacts> <trust-kat.json>` refuses any archive that embeds it.
- Built with `CGO_ENABLED=0` the package compiles and `New` returns `CodeEngineMissing`. A cgo build whose library
  is not linked fails at link/load time, before Go runs.

Local test (engine built from keep-core `ffi-uniffi`):

```sh
# test-root engine: full shared-KAT replay
CGO_LDFLAGS="-L$LIB_TEST -lk9o_keep_core_ffi_uniffi -Wl,-rpath,$LIB_TEST" go test -count=1 ./...
# release engine: every signed ruleset / key manifest must be refused
K9O_EXPECT_PRODUCTION=1 CGO_LDFLAGS="-L$LIB_REL -lk9o_keep_core_ffi_uniffi -Wl,-rpath,$LIB_REL" go test -count=1 ./...
CGO_ENABLED=0 go test -count=1 ./...   # typed engine-missing degrade
```
(On macOS set the dylib install name to `@rpath/...` first: `install_name_tool -id`.)
