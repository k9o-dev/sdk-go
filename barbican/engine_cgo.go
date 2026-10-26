//go:build cgo

package barbican

import (
	"errors"

	ffi "k9o.dev/sdk-go/barbican/k9o_keep_core_ffi_uniffi"
)

type cgoEngine struct{ c *ffi.KeepCore }

func newEngine(cfg config) (engine, error) {
	return &cgoEngine{c: ffi.NewKeepCore(ffi.KeepConfig{FailOpen: cfg.failOpen, Leet: cfg.leet, DecodeDepth: cfg.decodeDepth})}, nil
}

func usesTestTrustRoot() (bool, error) { return ffi.UsesTestTrustRoot(), nil }

// toRefusal maps the binding's error to a refusal: the engine's closed reason token rides through untouched.
func toRefusal(err error) *refusal {
	if err == nil {
		return nil
	}
	var ri *ffi.KeepErrorRulesetInvalid
	if errors.As(err, &ri) {
		return &refusal{reason: ri.Reason}
	}
	return &refusal{other: err.Error()}
}

func lowerRules(rs []Rule) []ffi.KeepRule {
	out := make([]ffi.KeepRule, len(rs))
	for i, r := range rs {
		out[i] = ffi.KeepRule{Id: r.ID, Pattern: r.Pattern, Severity: r.Severity}
	}
	return out
}

func lowerSigned(r SignedRuleset) ffi.KeepSignedRulesetBlob {
	return ffi.KeepSignedRulesetBlob{
		RulesetVersion: r.RulesetVersion,
		Rules:          lowerRules(r.Rules),
		SigningKeyKid:  r.SigningKeyKid,
		Signature:      r.Signature,
		CreatedAt:      r.CreatedAt,
	}
}

func (e *cgoEngine) loadKeyManifest(jws string, now uint64) (uint64, *refusal) {
	v, err := e.c.LoadKeyManifest(jws, now)
	return v, toRefusal(err)
}
func (e *cgoEngine) loadRulesetUnsigned(version uint64, rules []Rule) (uint64, *refusal) {
	v, err := e.c.LoadRulesetUnsigned(ffi.KeepRulesetBlob{RulesetVersion: version, Rules: lowerRules(rules)})
	return v, toRefusal(err)
}
func (e *cgoEngine) loadRulesetSigned(r SignedRuleset) (uint64, *refusal) {
	v, err := e.c.LoadRulesetSigned(lowerSigned(r))
	return v, toRefusal(err)
}
func (e *cgoEngine) rulesetDigest(r SignedRuleset) (string, *refusal) {
	v, err := e.c.RulesetDigest(lowerSigned(r))
	return v, toRefusal(err)
}
func (e *cgoEngine) verifyManifest(jws string) (ManifestInfo, *refusal) {
	m, err := e.c.VerifyManifest(jws)
	if err != nil {
		return ManifestInfo{}, toRefusal(err)
	}
	return ManifestInfo{RulesetVersion: m.RulesetVersion, SigningKeyKid: m.SigningKeyKid, ArtifactHash: m.ArtifactHash, ArtifactURL: m.ArtifactUrl, ValidUntil: m.ValidUntil}, nil
}
func (e *cgoEngine) keyManifestSequence() uint64 { return e.c.KeyManifestSequence() }
func (e *cgoEngine) signedFloor() uint64         { return e.c.SignedFloor() }
func (e *cgoEngine) activeRulesetIsSigned() bool { return e.c.ActiveRulesetIsSigned() }
func (e *cgoEngine) trustedKids() []string       { return e.c.TrustedKids() }
func (e *cgoEngine) deprecatedKids() []string    { return e.c.DeprecatedKids() }
func (e *cgoEngine) classify(t string) string    { return e.c.ClassifyLocal(t) }
func (e *cgoEngine) coreVersion() string         { return e.c.CoreVersion() }
func (e *cgoEngine) close()                      { e.c.Destroy() }

func (e *cgoEngine) inspect(text string, wc *string) Verdict {
	v := e.c.InspectInput(text, wc)
	m := make([]Matched, len(v.Matched))
	for i, x := range v.Matched {
		m[i] = Matched{RuleID: x.RuleId, SpanStart: x.SpanStart, SpanEnd: x.SpanEnd, Severity: x.Severity, MatchedForm: x.MatchedForm}
	}
	return Verdict{Decision: v.Decision, Reason: v.Reason, Matched: m, SchemaValid: v.SchemaValid, WaypointClassHint: v.WaypointClassHint, FailMode: v.FailMode, RulesetVersion: v.RulesetVersion, CoreVersion: v.CoreVersion}
}

func (e *cgoEngine) newFilter() filterEngine { return &cgoFilter{f: e.c.OutputFilter()} }

type cgoFilter struct{ f *ffi.KeepStreamFilter }

func (f *cgoFilter) push(c string) string { return f.f.Push(c) }
func (f *cgoFilter) finish() string       { return f.f.Finish() }
func (f *cgoFilter) report() LeakReport {
	r := f.f.Report()
	return LeakReport{Emails: r.Emails, Secrets: r.Secrets, Markers: r.Markers, Total: r.Total}
}
func (f *cgoFilter) close() { f.f.Destroy() }
