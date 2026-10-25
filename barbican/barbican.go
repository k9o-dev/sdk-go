// Package barbican is the free, embeddable L2 guardrail for Go: input normalization, deny-list matching over a
// signed ruleset, a local verdict, and a streaming output redactor.
//
// ZERO guardrail logic and ZERO crypto live here. Every normalization pass, match, verdict, redaction and every
// signature / key-manifest / anti-rollback decision is made by the audited native engine (k9o-keep-core, via the
// uniffi-bindgen-go cgo binding in ./k9o_keep_core_ffi_uniffi). This wrapper holds no key and takes no verifier or
// keyring argument: LoadRulesetSigned hands the blob to the engine, which verifies it under a key from a root-signed
// key manifest. A refusal is the engine's closed reason token, surfaced unchanged in Error.Reason.
//
// CLAIM BOUNDARY: Barbican is advisory / defense-in-depth (edge filtration), never the enforced sandbox boundary.
//
// This is a nested module (k9o.dev/sdk-go/barbican) so the core k9o.dev/sdk-go SDK stays pure-Go. Without cgo, or
// without the native engine linked, the package still compiles and New returns an *Error with Code
// CodeEngineMissing.
package barbican

// Error codes. Reason tokens (Error.Reason) are the engine's closed set, passed through verbatim.
const (
	CodeRulesetInvalid = "K9O_BARBICAN_RULESET_INVALID"
	CodeEngineMissing  = "K9O_BARBICAN_ENGINE_MISSING"
	CodeEngineError    = "K9O_BARBICAN_ENGINE_ERROR"
)

// Stage says which load a refusal came from.
const (
	StageRuleset     = "ruleset"
	StageKeyManifest = "key_manifest"
)

// Error mirrors the platform KError shape (Code + Message + Context). For CodeRulesetInvalid, Reason is the engine's
// closed token (unknown_kid, signature_invalid, signature_non_canonical, version_stale, malformed, ...,
// trust_root_unavailable on a release build) and Stage is StageRuleset or StageKeyManifest.
type Error struct {
	Code    string
	Message string
	Reason  string
	Stage   string
	Context map[string]any
}

func (e *Error) Error() string { return "[" + e.Code + "] " + e.Message }

// IsEngineMissing reports whether err is the typed "native engine is not available" error.
func IsEngineMissing(err error) bool {
	e, ok := err.(*Error)
	return ok && e.Code == CodeEngineMissing
}

func engineMissing(why string) *Error {
	return &Error{
		Code:    CodeEngineMissing,
		Message: "the Barbican native engine (k9o-keep-core uniffi library) is not available: " + why,
	}
}

// Rule is one deny-list rule. Severity is one of low|medium|high|critical.
type Rule struct {
	ID       string
	Pattern  string
	Severity string
}

// SignedRuleset is a ruleset plus the signature the engine will verify. There is no key or verifier to pass.
type SignedRuleset struct {
	RulesetVersion uint64
	Rules          []Rule
	SigningKeyKid  string
	Signature      []byte
	CreatedAt      int64
}

// ManifestInfo is the authenticated content of a verified ruleset manifest.
type ManifestInfo struct {
	RulesetVersion uint64
	SigningKeyKid  string
	ArtifactHash   *string
	ArtifactURL    *string
	ValidUntil     *uint64
}

// Matched is one rule hit in a verdict.
type Matched struct {
	RuleID      string
	SpanStart   uint32
	SpanEnd     uint32
	Severity    string
	MatchedForm string
}

// Verdict is the engine's local verdict. Decision/Reason are the engine's closed tokens.
type Verdict struct {
	Decision          string
	Reason            string
	Matched           []Matched
	SchemaValid       bool
	WaypointClassHint *string
	FailMode          *string
	RulesetVersion    uint64
	CoreVersion       string
}

// LeakReport is counts only, never the leaked values.
type LeakReport struct {
	Emails, Secrets, Markers, Total uint32
}

type config struct {
	failOpen    bool
	leet        bool
	decodeDepth uint8
}

// Option configures New.
type Option func(*config)

// WithFailOpen makes the engine fail open instead of closed.
func WithFailOpen(v bool) Option { return func(c *config) { c.failOpen = v } }

// WithLeet toggles leetspeak de-obfuscation (default true).
func WithLeet(v bool) Option { return func(c *config) { c.leet = v } }

// WithDecodeDepth sets the bounded base64/leet de-obfuscation depth; 0 uses the engine default (3).
func WithDecodeDepth(d uint8) Option { return func(c *config) { c.decodeDepth = d } }

// refusal is what an engine reports for a refused load; the wrapper maps it to *Error and never to success.
type refusal struct {
	reason string
	other  string // non-refusal engine failure text, when reason == ""
}

// engine is the seam to the native core (engine_cgo.go). Pure-Go types only, so this file builds without cgo.
type engine interface {
	loadKeyManifest(jws string, nowSecs uint64) (uint64, *refusal)
	loadRulesetUnsigned(version uint64, rules []Rule) (uint64, *refusal)
	loadRulesetSigned(r SignedRuleset) (uint64, *refusal)
	rulesetDigest(r SignedRuleset) (string, *refusal)
	verifyManifest(jws string) (ManifestInfo, *refusal)
	keyManifestSequence() uint64
	signedFloor() uint64
	activeRulesetIsSigned() bool
	trustedKids() []string
	deprecatedKids() []string
	inspect(text string, waypointClass *string) Verdict
	classify(text string) string
	newFilter() filterEngine
	coreVersion() string
	close()
}

type filterEngine interface {
	push(chunk string) string
	finish() string
	report() LeakReport
	close()
}

// Barbican is one engine instance. Not safe for concurrent mutation; guard with your own lock if you share it.
type Barbican struct{ e engine }

// New constructs a Barbican. No network. Returns an *Error with Code CodeEngineMissing when the native engine is
// not available (built without cgo, or the library is not linked).
func New(opts ...Option) (*Barbican, error) {
	c := config{leet: true}
	for _, o := range opts {
		o(&c)
	}
	e, err := newEngine(c)
	if err != nil {
		return nil, err
	}
	return &Barbican{e: e}, nil
}

// Close releases the native handle.
func (b *Barbican) Close() { b.e.close() }

func mapRefusal(r *refusal, stage string) error {
	if r == nil {
		return nil
	}
	if r.reason != "" {
		return &Error{
			Code:    CodeRulesetInvalid,
			Message: "ruleset refused: " + r.reason,
			Reason:  r.reason,
			Stage:   stage,
			Context: map[string]any{"reason": r.reason, "stage": stage},
		}
	}
	return &Error{Code: CodeEngineError, Message: r.other}
}

// LoadRuleset loads a static, host-vouched ruleset (signature verification is skipped; the engine still enforces
// anti-rollback). Returns the accepted version.
func (b *Barbican) LoadRuleset(version uint64, rules []Rule) (uint64, error) {
	v, r := b.e.loadRulesetUnsigned(version, rules)
	if err := mapRefusal(r, StageRuleset); err != nil {
		return 0, err
	}
	return v, nil
}

// LoadKeyManifest loads a Barbican key manifest (compact JWS). The engine checks it against the root compiled into
// it. On a refusal the trusted keys are unchanged. Returns the manifest sequence.
func (b *Barbican) LoadKeyManifest(jws string, nowSecs uint64) (uint64, error) {
	v, r := b.e.loadKeyManifest(jws, nowSecs)
	if err := mapRefusal(r, StageKeyManifest); err != nil {
		return 0, err
	}
	return v, nil
}

// VerifyManifest verifies a ruleset manifest (compact JWS) under a trusted key.
func (b *Barbican) VerifyManifest(jws string) (ManifestInfo, error) {
	v, r := b.e.verifyManifest(jws)
	if err := mapRefusal(r, StageRuleset); err != nil {
		return ManifestInfo{}, err
	}
	return v, nil
}

// LoadRulesetSigned hands a signed ruleset to the engine, which verifies it itself. Any failure returns a
// *Error{Code: CodeRulesetInvalid} with the engine's reason and leaves the current ruleset running.
func (b *Barbican) LoadRulesetSigned(blob SignedRuleset) (uint64, error) {
	v, r := b.e.loadRulesetSigned(blob)
	if err := mapRefusal(r, StageRuleset); err != nil {
		return 0, err
	}
	return v, nil
}

// RulesetDigest is the lowercase hex SHA-256 of the ruleset's canonical signing bytes (what a manifest's
// artifact hash names). A digest only; it decides no trust.
func (b *Barbican) RulesetDigest(blob SignedRuleset) (string, error) {
	v, r := b.e.rulesetDigest(blob)
	if err := mapRefusal(r, StageRuleset); err != nil {
		return "", err
	}
	return v, nil
}

// KeyManifestSequence is the sequence of the key manifest in force (0 before one is loaded).
func (b *Barbican) KeyManifestSequence() uint64 { return b.e.keyManifestSequence() }

// SignedFloor is the highest VERIFIED ruleset version loaded (0 = none): the anti-rollback floor for signed updates.
func (b *Barbican) SignedFloor() uint64 { return b.e.signedFloor() }

// ActiveRulesetIsSigned reports whether the ruleset in force came from a verified load.
func (b *Barbican) ActiveRulesetIsSigned() bool { return b.e.activeRulesetIsSigned() }

// TrustedKids are the signing kids the loaded key manifest trusts, sorted.
func (b *Barbican) TrustedKids() []string { return b.e.trustedKids() }

// DeprecatedKids are the trusted kids the key manifest flags deprecated, sorted.
func (b *Barbican) DeprecatedKids() []string { return b.e.deprecatedKids() }

// Inspect normalizes, matches and returns a verdict. waypointClass is a meter hint and NEVER gates the decision.
func (b *Barbican) Inspect(text string, waypointClass *string) Verdict {
	return b.e.inspect(text, waypointClass)
}

// Classify is the local L3 stand-in ("clear" | "suspicious" | "malicious"). No receipt.
func (b *Barbican) Classify(text string) string { return b.e.classify(text) }

// CoreVersion is the engine's semver (audit anchor).
func (b *Barbican) CoreVersion() string { return b.e.coreVersion() }

// OutputFilter is a streaming redactor: buffer-and-scan-before-flush, never emits a byte the engine has not cleared.
type OutputFilter struct{ f filterEngine }

// NewOutputFilter opens an output filter bound to the active ruleset's exfil markers.
func (b *Barbican) NewOutputFilter() *OutputFilter { return &OutputFilter{f: b.e.newFilter()} }

// Push feeds a chunk and returns the cleared prefix safe to emit now.
func (o *OutputFilter) Push(chunk string) string { return o.f.push(chunk) }

// Finish flushes the residual (also scanned) buffer at end of stream.
func (o *OutputFilter) Finish() string { return o.f.finish() }

// Report returns counts only.
func (o *OutputFilter) Report() LeakReport { return o.f.report() }

// Close releases the native handle.
func (o *OutputFilter) Close() { o.f.close() }

// UsesTestTrustRoot is false on every shipped build. True means the linked engine trusts the PUBLISHED TEST root
// and must never be used outside tests.
func UsesTestTrustRoot() (bool, error) { return usesTestTrustRoot() }
