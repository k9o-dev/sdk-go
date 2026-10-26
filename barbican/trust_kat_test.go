//go:build cgo

package barbican

import (
	"os"
	"reflect"
	"testing"
)

// Replays the shared trust-kat.json through the Go binding (same checks as keep-core tests/replay/trust-kat.py and
// the Python/Ruby wrappers). Two modes, chosen by the build under test:
//   - engine built with --features test-trust-root: the full replay (default);
//   - K9O_EXPECT_PRODUCTION=1 with a release engine: every key manifest / signed ruleset must be REFUSED, none accepted.
//
// A mismatch between the env and the linked engine FAILS (never skips): a vacuous green is the failure to avoid.

type counter struct {
	t    *testing.T
	n    int
	fail int
}

func (c *counter) check(name string, got, want any) {
	c.n++
	if !reflect.DeepEqual(got, want) {
		c.fail++
		c.t.Errorf("FAIL %s: got %v, want %v", name, got, want)
	}
}

func mustNew(t *testing.T) *Barbican {
	t.Helper()
	b, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(b.Close)
	return b
}

func TestTrustKAT(t *testing.T) {
	v := loadVector(t)
	prod := os.Getenv("K9O_EXPECT_PRODUCTION") == "1"
	test, err := UsesTestTrustRoot()
	if err != nil {
		t.Fatal(err)
	}
	if prod && test {
		t.Fatal("K9O_EXPECT_PRODUCTION=1 but the linked engine trusts the TEST root")
	}
	if !prod && !test {
		t.Fatal("the linked engine does not trust the test root; set K9O_EXPECT_PRODUCTION=1 to replay the refusals")
	}
	c := &counter{t: t}
	withKeys := func() *Barbican {
		b := mustNew(t)
		if _, err := b.LoadKeyManifest(v.KeyManifest.JWS, v.Now); err != nil {
			t.Fatalf("key manifest: %v", err)
		}
		return b
	}

	if prod {
		for _, k := range v.KeyManifestCases {
			_, err := mustNew(t).LoadKeyManifest(k.JWS, v.Now)
			c.check("prod keys "+k.Name, outcome(err), "trust_root_unavailable")
			if err == nil {
				t.Errorf("ACCEPTED key manifest %s on a release engine", k.Name)
			}
		}
		for _, r := range v.RulesetCases {
			b := mustNew(t)
			_, err := b.LoadRulesetSigned(r.Blob.signed(t))
			want := "unknown_kid"
			if r.Expect == "malformed" {
				want = "malformed"
			}
			c.check("prod ruleset "+r.Name, outcome(err), want)
			if err == nil {
				t.Errorf("ACCEPTED signed ruleset %s on a release engine", r.Name)
			}
			c.check("prod ruleset "+r.Name+" loaded nothing", b.Inspect("please DROP TABLE users", nil).RulesetVersion, uint64(0))
			c.check("prod ruleset "+r.Name+" signed floor", b.SignedFloor(), uint64(0))
			c.check("prod ruleset "+r.Name+" not signed", b.ActiveRulesetIsSigned(), false)
		}
		t.Logf("trust-kat (go, production): %d refusals checked", c.n)
		if c.n < 20 {
			t.Fatalf("only %d production checks ran", c.n)
		}
		return
	}

	for _, k := range v.KeyManifestCases {
		_, err := mustNew(t).LoadKeyManifest(k.JWS, v.Now)
		c.check("keys "+k.Name, outcome(err), k.Expect)
	}
	kb := withKeys()
	c.check("keys sequence", kb.KeyManifestSequence(), v.KeyManifest.Sequence)
	c.check("keys kids", kb.TrustedKids(), v.KeyManifest.TrustedKids)

	for _, m := range v.ManifestCases {
		info, err := withKeys().VerifyManifest(m.JWS)
		c.check("manifest "+m.Name, outcome(err), m.Expect)
		if m.Expect == "ok" {
			got := map[string]any{
				"ruleset_version": float64(info.RulesetVersion),
				"signing_key_kid": info.SigningKeyKid,
			}
			if info.ArtifactHash != nil {
				got["artifact_hash"] = *info.ArtifactHash
			}
			if info.ArtifactURL != nil {
				got["artifact_url"] = *info.ArtifactURL
			}
			if info.ValidUntil != nil {
				got["valid_until"] = float64(*info.ValidUntil)
			}
			for key, want := range m.Info {
				c.check("manifest "+m.Name+"."+key, got[key], want)
			}
		}
	}

	for _, r := range v.RulesetCases {
		b := mustNew(t)
		if r.LoadKeys {
			if _, err := b.LoadKeyManifest(v.KeyManifest.JWS, v.Now); err != nil {
				t.Fatal(err)
			}
		}
		if r.PreloadUnsigned != nil {
			if _, err := b.LoadRuleset(r.PreloadUnsigned.RulesetVersion, nil); err != nil {
				t.Fatal(err)
			}
		}
		if r.PreloadBlob != nil {
			if _, err := b.LoadRulesetSigned(r.PreloadBlob.signed(t)); err != nil {
				t.Fatal(err)
			}
		}
		blob := r.Blob.signed(t)
		d, _ := b.RulesetDigest(blob)
		c.check("ruleset "+r.Name+" digest", d, r.Digest)
		ver, err := b.LoadRulesetSigned(blob)
		c.check("ruleset "+r.Name, outcome(err), r.Expect)
		if r.Expect == "ok" {
			c.check("ruleset "+r.Name+" version", ver, r.Version)
		}
		c.check("ruleset "+r.Name+" signed floor", b.SignedFloor(), r.SignedFloor)
		c.check("ruleset "+r.Name+" active is signed", b.ActiveRulesetIsSigned(), r.ActiveIsSigned)
		for _, p := range r.Probes {
			c.check("ruleset "+r.Name+" probe "+p.Text, b.Inspect(p.Text, nil).Decision == "deny", p.Denied)
		}
		if r.ThenUnsigned != nil {
			if _, err := b.LoadRuleset(r.ThenUnsigned.RulesetVersion, nil); err != nil {
				t.Fatal(err)
			}
			a := r.AfterUnsigned
			c.check("ruleset "+r.Name+" floor after unsigned replace", b.SignedFloor(), a.SignedFloor)
			c.check("ruleset "+r.Name+" provenance after unsigned replace", b.ActiveRulesetIsSigned(), a.ActiveIsSigned)
			for _, p := range a.Probes {
				c.check("ruleset "+r.Name+" after unsigned replace, probe "+p.Text, b.Inspect(p.Text, nil).Decision == "deny", p.Denied)
			}
		}
	}
	t.Logf("trust-kat (go): %d checks, %d failed", c.n, c.fail)
	if c.n < 115 {
		t.Fatalf("only %d checks ran; the Python replay runs 115 or more", c.n)
	}
}
