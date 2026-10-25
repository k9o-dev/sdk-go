//go:build !cgo

package barbican

import "testing"

func TestNewWithoutEngineReturnsTypedError(t *testing.T) {
	b, err := New()
	if b != nil || !IsEngineMissing(err) {
		t.Fatalf("b=%v err=%v", b, err)
	}
	if _, err := UsesTestTrustRoot(); !IsEngineMissing(err) {
		t.Fatalf("err=%v", err)
	}
}
