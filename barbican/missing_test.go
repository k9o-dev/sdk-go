package barbican

import "testing"

// The engine seam must degrade to a typed error naming the missing engine, never a nil deref. Under
// CGO_ENABLED=0 the binding cannot exist, so New really does hit this path (see also README: a cgo build whose
// library is not linked fails at link/dyld time, before any Go runs).

func TestEngineMissingErrorIsTyped(t *testing.T) {
	err := error(engineMissing("test"))
	if !IsEngineMissing(err) {
		t.Fatal("not recognised")
	}
	e := err.(*Error)
	if e.Code != "K9O_BARBICAN_ENGINE_MISSING" || e.Message == "" {
		t.Fatalf("%+v", e)
	}
}
