# k9o.dev/sdk-go/kibo

Client-side encryption for Go against your own KMS. The platform never sees plaintext or unwrapped keys.

## Contents

- [Links](#links)
- [Install](#install)
- [Quick start](#quick-start)
- [Handling errors](#handling-errors)
- [Building with cgo](#building-with-cgo)
- [Appendix](#appendix): [functions](#functions), [encrypt input fields](#encrypt-input-fields),

## Links

- Website: <https://kilimanjaro.dev>
- Documentation / Developer Hub: <https://docs.kilimanjaro.dev>
- Platform API: <https://api.k9o.io>
- Platform Console: <https://console.k9o.io>
  [decrypt input fields](#decrypt-input-fields), [error fields](#error-fields), [related packages](#related-packages)

## Install

```bash
go get k9o.dev/sdk-go/kibo
```

- Requires Go 1.22 or newer and cgo.
- This is a separate module from `k9o.dev/sdk-go`, so the core SDK stays pure Go.
- See [building with cgo](#building-with-cgo) for the native library.

## Quick start

Encrypt a value with a key in AWS KMS, then decrypt it again.
The cryptography runs in a native library; this package is a thin Go wrapper.

```go
package main

import (
	"fmt"
	"os"
	"time"

	"k9o.dev/sdk-go/kibo"
	ffi "k9o.dev/sdk-go/kibo/kibo_kms_rest_ffi"
)

func main() {
	region := "eu-west-1"
	expires := uint64(time.Now().Add(time.Hour).Unix())

	input := kibo.EncryptInput{
		Provider:        ffi.KmsProviderAwsKms,
		Posture:         ffi.FfiPostureByok,
		KmsCred:         kibo.Credential{Secret: os.Getenv("KMS_TOKEN"), ExpiresAtUnixSecs: expires},
		KeyRef:          kibo.KeyRef{Version: 1, ExternalRef: "arn:aws:kms:eu-west-1:111122223333:key/example"},
		ProviderConfig:  kibo.ProviderConfig{AwsRegion: &region},
		AttestationTier: ffi.FfiAttestationTierSoftware,
		Plaintext:       []byte("hello"),
	}
	encrypted, kerr := kibo.Encrypt(input)
	if kerr != nil {
		fmt.Println(kerr)
		return
	}

	decryptInput := kibo.DecryptInput{
		KmsCred:         input.KmsCred,
		ProviderConfig:  input.ProviderConfig,
		Envelope:        encrypted.Envelope,
		Strategy:        ffi.FfiStrategyLocalEnvelope,
		AttestationTier: ffi.FfiAttestationTierSoftware,
	}
	decrypted, kerr := kibo.Decrypt(decryptInput)
	if kerr != nil {
		fmt.Println(kerr)
		return
	}
	fmt.Println(string(decrypted.Plaintext))
}
```

```text
hello
```

- `Encrypt` returns an opaque envelope in `Envelope`. Store it wherever you like.
- When `Strategy` is left unset on encrypt, `LocalEnvelope` is used. Pass the same strategy to `Decrypt`.
- Decrypt needs the same strategy, attestation tier and encryption context as encrypt.

## Handling errors

Both functions return a `*kibo.Error`, which is `nil` on success. It implements `error`.
Printing it gives `[code] message`:

```go
_, kerr := kibo.Decrypt(decryptInput)
if kerr != nil {
	switch kerr.Code {
	case "UPSTREAM_KMS_ERROR":
		fmt.Println("your KMS failed:", kerr.Message)
	case "K9O_CRYPTO_POSTURE_FLOOR_VIOLATION":
		fmt.Println("posture too low:", kerr.Context["dimension"])
	default:
		fmt.Println(kerr)
	}
}
```

- `UPSTREAM_KMS_ERROR` means your own KMS call failed.
- `K9O_CRYPTO_POSTURE_FLOOR_VIOLATION` carries `dimension`, `requested` and `declared` in `Context`.
- Other codes come from the platform and are passed through. See [error fields](#error-fields).

## Building with cgo

The module links a native library, `libkibo_kms_rest_ffi`, from the `kibo_kms_rest_ffi/` directory.
Point cgo at that directory:

```sh
CGO_CFLAGS="-I$PWD/kibo_kms_rest_ffi" \
CGO_LDFLAGS="-L$PWD/kibo_kms_rest_ffi -lkibo_kms_rest_ffi" \
  go build ./...
```

## Appendix

### Functions

| Function | What it does |
|---|---|
| `kibo.Encrypt(input)` | Encrypt `Plaintext` using your KMS and return an `EncryptOutput` with the envelope |
| `kibo.Decrypt(input)` | Decrypt an envelope and return a `DecryptOutput` with the plaintext |

Provider, posture, strategy and attestation constants live in `k9o.dev/sdk-go/kibo/kibo_kms_rest_ffi`.

### Encrypt input fields

| Field | Meaning |
|---|---|
| `Provider` | KMS provider, for example `ffi.KmsProviderAwsKms` |
| `Posture` | `ffi.FfiPostureByok` or `ffi.FfiPostureHyok` |
| `Strategy` | Optional. `ffi.FfiStrategyLocalEnvelope` is used when unset |
| `OracleMode` | Optional oracle routing mode |
| `KmsCred` | Your short-lived KMS credential (`Secret`, `ExpiresAtUnixSecs`) |
| `KeyRef` | Key version and provider key identifier (`Version`, `ExternalRef`) |
| `ProviderConfig` | Provider settings, such as `AwsRegion` or `VaultAddress` |
| `AttestationTier` | `ffi.FfiAttestationTierSoftware`, `...HardwareBacked` or `...Confidential` |
| `EncryptionContext` | Optional key/value pairs bound to the ciphertext |
| `Plaintext` | The bytes to encrypt |
| `PosturePolicy` | Optional policy checked before encrypting |

### Decrypt input fields

| Field | Meaning |
|---|---|
| `KmsCred` | Your KMS credential |
| `ProviderConfig` | Provider settings |
| `Envelope` | The bytes returned by `Encrypt` |
| `Strategy` | Must match the strategy used to encrypt |
| `AttestationTier` | Must match the tier used to encrypt |
| `EncryptionContext` | Must match the context used to encrypt |

### Error fields

| Field | Type | Meaning |
|---|---|---|
| `Code` | `string` | Error code, for example `UPSTREAM_KMS_ERROR` |
| `Message` | `string` | Human-readable explanation |
| `Context` | `map[string]any` | Extra details, such as the posture dimension |

### Related packages

| Module | What it does |
|---|---|
| `k9o.dev/sdk-go` | The core platform SDK |
| `k9o.dev/sdk-go/barbican` | Guardrail that inspects text against a signed ruleset |

## License

Apache-2.0 © Formelo Inc. See `LICENSE` and `NOTICE`.
