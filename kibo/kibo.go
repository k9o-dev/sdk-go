// Package kibo is the Zero-Knowledge BYOK/HYOK vault crypto surface for Go —
// kibo.Encrypt / kibo.Decrypt run entirely client-side against your own KMS.
// The platform never sees plaintext or unwrapped keys.
//
// ZERO crypto here: every wrap/unwrap, envelope framing, posture-floor
// enforcement, and KMS round-trip lives in the audited Rust core
// (kibo-kms-rest-ffi, piece C). Go has no async runtime in the FFI calling
// convention, so this calls the SYNC ZkEncryptBlocking / ZkDecryptBlocking
// exports (crypto-identical to the async exports the mobile SDKs use).
//
// This is a nested module (k9o.dev/sdk-go/kibo) so the core k9o.dev/sdk-go SDK
// stays pure-Go: the uniffi-bindgen-go binding here is cgo and links the native
// cdylib. The generated binding (kibo_kms_rest_ffi/) is committed + drift-checked
// and the per-platform cdylib is CI-injected.
package kibo

import (
	"errors"

	ffi "k9o.dev/sdk-go/kibo/kibo_kms_rest_ffi"
)

// Re-exported binding types so callers import only this package.
type (
	EncryptInput      = ffi.EncryptInput
	EncryptOutput     = ffi.EncryptOutput
	DecryptInput      = ffi.DecryptInput
	DecryptOutput     = ffi.DecryptOutput
	Credential        = ffi.FfiCredential
	KeyRef            = ffi.FfiKeyRef
	ProviderConfig    = ffi.FfiProviderConfig
	KvPair            = ffi.FfiKvPair
	PosturePolicy     = ffi.FfiPosturePolicy
	AllowedOracleModes = ffi.FfiAllowedOracleModes
)

// Error mirrors the platform KError shape (Code + Message + Context). The Go ZK
// surface is a separate module, so it carries its own error type rather than
// importing the core SDK's KError (same dedicated-error pattern as the mobile
// SDKs). Codes are passed through from the bound core (never hardcoded):
//   - a K9O_* wire code for platform errors,
//   - "UPSTREAM_KMS_ERROR" for the customer's own KMS failing (error-code
//     namespace boundary: upstream != K9O_*),
//   - "K9O_CRYPTO_POSTURE_FLOOR_VIOLATION" for a posture-floor breach, with the
//     dimension / requested / declared carried in Context.
type Error struct {
	Code    string
	Message string
	Context map[string]any
}

func (e *Error) Error() string {
	return "[" + e.Code + "] " + e.Message
}

// Encrypt encrypts input.Plaintext via the customer KMS, client-side, returning
// the opaque ZKFE envelope. Returns a non-nil *Error on failure.
func Encrypt(input EncryptInput) (EncryptOutput, *Error) {
	out, err := ffi.ZkEncryptBlocking(input)
	if err != nil {
		return out, toError(err)
	}
	return out, nil
}

// Decrypt decrypts a ZKFE input.Envelope. Strategy, AttestationTier, and
// EncryptionContext MUST match the encrypt-time values (AAD-bound).
func Decrypt(input DecryptInput) (DecryptOutput, *Error) {
	out, err := ffi.ZkDecryptBlocking(input)
	if err != nil {
		return out, toError(err)
	}
	return out, nil
}

// toError maps the bound core's uniffi error onto *Error, codes passed through.
func toError(err error) *Error {
	var pv *ffi.ZkFfiUniffiErrorPostureViolation
	if errors.As(err, &pv) {
		return &Error{
			Code:    pv.Code,
			Message: pv.Detail,
			Context: map[string]any{
				"dimension": pv.Dimension,
				"requested": pv.Requested,
				"declared":  pv.Declared,
			},
		}
	}
	var pl *ffi.ZkFfiUniffiErrorPlatform
	if errors.As(err, &pl) {
		return &Error{Code: pl.Code, Message: pl.Detail}
	}
	var uk *ffi.ZkFfiUniffiErrorUpstreamKms
	if errors.As(err, &uk) {
		return &Error{Code: "UPSTREAM_KMS_ERROR", Message: uk.Detail}
	}
	return &Error{Code: "KIBO_FFI_ERROR", Message: err.Error()}
}
