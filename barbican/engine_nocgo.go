//go:build !cgo

package barbican

// Built with CGO_ENABLED=0: the uniffi binding is cgo, so there is no engine. Typed degrade, never a nil deref.

func newEngine(config) (engine, error) {
	return nil, engineMissing("this build has cgo disabled (CGO_ENABLED=0); the engine binding is cgo")
}

func usesTestTrustRoot() (bool, error) {
	return false, engineMissing("this build has cgo disabled (CGO_ENABLED=0); the engine binding is cgo")
}
