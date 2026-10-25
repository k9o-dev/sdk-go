package barbican

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

type katBlob struct {
	RulesetVersion uint64 `json:"ruleset_version"`
	Rules          []struct {
		ID       string `json:"id"`
		Pattern  string `json:"pattern"`
		Severity string `json:"severity"`
	} `json:"rules"`
	SigningKeyKid string `json:"signing_key_kid"`
	SignatureB64  string `json:"signature_b64"`
	CreatedAt     int64  `json:"created_at"`
}

type katProbe struct {
	Text   string `json:"text"`
	Denied bool   `json:"denied"`
}

type katVector struct {
	Now         uint64 `json:"now"`
	KeyManifest struct {
		JWS         string   `json:"jws"`
		Sequence    uint64   `json:"sequence"`
		TrustedKids []string `json:"trustedKids"`
	} `json:"keyManifest"`
	KeyManifestCases []struct {
		Name   string `json:"name"`
		JWS    string `json:"jws"`
		Expect string `json:"expect"`
	} `json:"keyManifestCases"`
	ManifestCases []struct {
		Name   string         `json:"name"`
		JWS    string         `json:"jws"`
		Expect string         `json:"expect"`
		Info   map[string]any `json:"info"`
	} `json:"manifestCases"`
	RulesetCases []struct {
		Name            string     `json:"name"`
		Blob            katBlob    `json:"blob"`
		Expect          string     `json:"expect"`
		Version         uint64     `json:"version"`
		Digest          string     `json:"digest"`
		LoadKeys        bool       `json:"loadKeys"`
		SignedFloor     uint64     `json:"signedFloor"`
		ActiveIsSigned  bool       `json:"activeIsSigned"`
		Probes          []katProbe `json:"probes"`
		PreloadUnsigned *struct {
			RulesetVersion uint64 `json:"ruleset_version"`
		} `json:"preloadUnsigned"`
		PreloadBlob  *katBlob `json:"preloadBlob"`
		ThenUnsigned *struct {
			RulesetVersion uint64 `json:"ruleset_version"`
		} `json:"thenUnsigned"`
		AfterUnsigned *struct {
			SignedFloor    uint64     `json:"signedFloor"`
			ActiveIsSigned bool       `json:"activeIsSigned"`
			Probes         []katProbe `json:"probes"`
		} `json:"afterUnsigned"`
	} `json:"rulesetCases"`
}

func loadVector(t *testing.T) katVector {
	t.Helper()
	raw, err := os.ReadFile("testdata/trust-kat.json")
	if err != nil {
		t.Fatal(err)
	}
	var v katVector
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func (b katBlob) signed(t *testing.T) SignedRuleset {
	t.Helper()
	sig, err := base64.StdEncoding.DecodeString(b.SignatureB64)
	if err != nil {
		t.Fatal(err)
	}
	rs := make([]Rule, len(b.Rules))
	for i, r := range b.Rules {
		rs[i] = Rule{ID: r.ID, Pattern: r.Pattern, Severity: r.Severity}
	}
	return SignedRuleset{RulesetVersion: b.RulesetVersion, Rules: rs, SigningKeyKid: b.SigningKeyKid, Signature: sig, CreatedAt: b.CreatedAt}
}

// outcome: "ok" or the engine's closed reason token.
func outcome(err error) string {
	if err == nil {
		return "ok"
	}
	if e, ok := err.(*Error); ok && e.Code == CodeRulesetInvalid {
		return e.Reason
	}
	return "NON_REFUSAL:" + err.Error()
}
