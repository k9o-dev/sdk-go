package kibo

import (
	"testing"

	ffi "k9o.dev/sdk-go/kibo/kibo_kms_rest_ffi"
)

// The crypto round-trip needs real customer-KMS creds and is exercised by the
// staging conformance ZK step; here we pin the boundary the wrapper owns — the
// posture-floor rejection (enforced by the core pre-KMS), which round-trips
// through the SYNC ZkEncryptBlocking export and maps to a typed *Error. This
// test needs the CI-injected cdylib (the whole module is cgo); the identical
// behaviour is proven against the real core in the Ruby + Python suites.
func TestPostureFloorViolationMapsToError(t *testing.T) {
	in := EncryptInput{
		Provider:          ffi.KmsProviderAwsKms,
		Posture:           ffi.FfiPostureByok,
		KmsCred:           ffi.FfiCredential{Secret: "t", ExpiresAtUnixSecs: 0},
		KeyRef:            ffi.FfiKeyRef{Version: 1, ExternalRef: "arn:aws:kms:us-east-1:0:key/abc"},
		ProviderConfig:    ffi.FfiProviderConfig{AwsRegion: ptr("us-east-1")},
		AttestationTier:   ffi.FfiAttestationTierSoftware,
		EncryptionContext: []ffi.FfiKvPair{},
		Plaintext:         []byte("hi"),
		// BYOK requested, HYOK floor declared -> violation (rejected pre-KMS).
		PosturePolicy: &ffi.FfiPosturePolicy{
			MinCustodyPosture:  ptrMin(ffi.FfiMinPostureHyok),
			AllowedOracleModes: &ffi.FfiAllowedOracleModes{Brokered: true, Direct: true},
		},
	}

	_, err := Encrypt(in)
	if err == nil {
		t.Fatal("expected a posture-floor *Error, got nil")
	}
	if err.Code != "K9O_CRYPTO_POSTURE_FLOOR_VIOLATION" {
		t.Fatalf("code = %q, want K9O_CRYPTO_POSTURE_FLOOR_VIOLATION", err.Code)
	}
	if err.Context["dimension"] != "custody_posture" {
		t.Fatalf("dimension = %v, want custody_posture", err.Context["dimension"])
	}
	if err.Context["requested"] != "byok" {
		t.Fatalf("requested = %v, want byok", err.Context["requested"])
	}
}

func ptr(s string) *string { return &s }

func ptrMin(m ffi.FfiMinPosture) *ffi.FfiMinPosture { return &m }
