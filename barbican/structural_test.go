package barbican

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The wrapper does NO crypto and holds NO key: its hand-written sources must not import a crypto package or mention
// ed25519 / signature verification. (The generated binding is not scanned for the word "signature": the engine's
// own struct field is called that. It is scanned for crypto imports instead.)
func TestWrapperHasNoCryptoCode(t *testing.T) {
	forbidden := regexp.MustCompile(`(?i)ed25519|crypto/|golang\.org/x/crypto|hmac|sha256|sha512|rsa|ecdsa|jose|verifysig|verify_signature|publickey|public_key|ed25519\.Verify`)
	files, _ := filepath.Glob("*.go")
	scanned := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, _ := os.ReadFile(f)
		scanned++
		// strip comments: prose may say "crypto" (e.g. "ZERO crypto"); code may not.
		code := regexp.MustCompile(`(?m)//.*$`).ReplaceAllString(string(src), "")
		if m := forbidden.FindString(code); m != "" {
			t.Errorf("%s contains crypto-looking code %q", f, m)
		}
	}
	if scanned < 3 {
		t.Fatalf("scanned only %d wrapper files", scanned)
	}
	gen, err := os.ReadFile("k9o_keep_core_ffi_uniffi/k9o_keep_core_ffi_uniffi.go")
	if err != nil {
		t.Fatal(err)
	}
	if regexp.MustCompile(`"crypto/|x/crypto|ed25519`).Match(gen) {
		t.Error("generated binding imports crypto")
	}
}

// No verifier or keyring parameter anywhere in the public surface.
func TestNoVerifierOrKeyringParameter(t *testing.T) {
	src, _ := os.ReadFile("barbican.go")
	for _, bad := range []string{"Verifier", "Keyring", "PublicKey", "TrustedKey"} {
		if strings.Contains(string(src), bad) {
			t.Errorf("barbican.go mentions %s", bad)
		}
	}
}
