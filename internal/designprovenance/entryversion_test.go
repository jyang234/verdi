package designprovenance

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// v2Entry returns a valid resolved-policy v2 entry, mirroring codec_test.go's
// own validEntry helper for v1.
func v2Entry(t *testing.T, policy Policy) Entry {
	t.Helper()
	e := Entry{
		Schema:         SchemaV2,
		Spec:           "spec/widget",
		PreviousDigest: digestA,
		ResultDigest:   digestB,
		Attribution:    governanceprincipal.NewUnauthenticatedAttribution(),
		Policy:         &policy,
		Context:        UnavailableContext(),
		Operations: []Operation{{
			Op:     OpSetProblem,
			Text:   "customers cannot retry safely",
			Anchor: "problem",
		}},
		Changes:  []Change{{Target: "problem", Change: ChangeReplaced, BeforeDigest: digestA, AfterDigest: digestB}},
		Excerpts: []Excerpt{},
	}
	if err := e.Seal(); err != nil {
		t.Fatalf("Seal: %v", err)
	}
	return e
}

// TestDesignProvenanceV2ResolvedAndNotApplicableRoundTrip proves §4.1's two
// closed v2 policy arms both seal, canonically encode, and strict-decode
// back to the identical entry — the explicit browser-human's honest
// non-adoption declaration is a first-class, round-trippable shape, not a
// degraded or lossy one.
func TestDesignProvenanceV2ResolvedAndNotApplicableRoundTrip(t *testing.T) {
	for _, tt := range []struct {
		name   string
		policy Policy
	}{
		{"resolved", Policy{State: PolicyResolved, Digest: digestC}},
		{"not-applicable", Policy{State: PolicyNotApplicable}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := v2Entry(t, tt.policy)
			raw, err := EncodeEntry(e)
			if err != nil {
				t.Fatalf("EncodeEntry: %v", err)
			}
			if bytes.Contains(raw, []byte("policy_digest")) {
				t.Fatalf("v2 entry carries forbidden policy_digest: %s", raw)
			}
			got, err := DecodeEntry(bytes.TrimSuffix(raw, []byte("\n")))
			if err != nil {
				t.Fatalf("DecodeEntry: %v", err)
			}
			if got.Policy == nil || *got.Policy != tt.policy || got.Schema != SchemaV2 {
				t.Fatalf("decoded entry = %+v, want policy %+v", got, tt.policy)
			}
		})
	}
}

// TestDesignProvenanceV2RejectsCrossVersionFields proves the two policy
// representations are mutually exclusive by schema version in both
// directions: v1 forbids `policy` even when it would otherwise be
// well-formed, and v2 forbids `policy_digest` even when it carries a
// syntactically valid digest.
func TestDesignProvenanceV2RejectsCrossVersionFields(t *testing.T) {
	v1 := validEntry(t)
	v1Raw, err := EncodeEntry(v1)
	if err != nil {
		t.Fatal(err)
	}
	v2 := v2Entry(t, Policy{State: PolicyResolved, Digest: digestC})
	v2Raw, err := EncodeEntry(v2)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			"v1 carrying policy",
			strings.Replace(string(v1Raw), `"policy_digest":`, `"policy":{"state":"not-applicable"},"policy_digest":`, 1),
			"v1 entry must not carry field \"policy\"",
		},
		{
			"v2 carrying policy_digest",
			strings.Replace(string(v2Raw), `"policy":`, `"policy_digest":"`+digestC+`","policy":`, 1),
			"v2 entry must not carry field \"policy_digest\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeEntry([]byte(tt.raw)); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("DecodeEntry error = %v, want it to contain %q", err, tt.want)
			}
		})
	}

	// The same exclusion holds for programmatically constructed entries,
	// not only decoded ones (Seal/Validate, not just UnmarshalJSON).
	forgedV1 := v1
	resolved := Policy{State: PolicyResolved, Digest: digestC}
	forgedV1.Policy = &resolved
	if err := forgedV1.Seal(); err == nil || !strings.Contains(err.Error(), "v1 entry must not carry a policy field") {
		t.Fatalf("Seal v1-with-policy error = %v", err)
	}

	forgedV2 := v2
	forgedV2.PolicyDigest = digestC
	if err := forgedV2.Seal(); err == nil || !strings.Contains(err.Error(), "v2 entry must not carry policy_digest") {
		t.Fatalf("Seal v2-with-policy_digest error = %v", err)
	}
}

// TestDesignProvenanceV2PolicyUnionStrictDecode is the closed-arm rejection
// matrix: missing, null, cross-arm, unknown-field, unknown-state, and
// duplicate-key policy shapes all fail closed, and the failure is keyed on
// field PRESENCE, not merely the decoded zero value (the "not-applicable
// with a null digest" and "resolved with an empty digest present" cases
// below would silently pass a value-only check).
func TestDesignProvenanceV2PolicyUnionStrictDecode(t *testing.T) {
	e := v2Entry(t, Policy{State: PolicyResolved, Digest: digestC})
	raw, err := EncodeEntry(e)
	if err != nil {
		t.Fatal(err)
	}
	trimmed := string(bytes.TrimSuffix(raw, []byte("\n")))

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"missing policy", strings.Replace(trimmed, `"policy":{"digest":"`+digestC+`","state":"resolved"},`, "", 1), "missing required field \"policy\""},
		{"null policy", strings.Replace(trimmed, `{"digest":"`+digestC+`","state":"resolved"}`, "null", 1), "must not be null"},
		{"resolved missing digest", strings.Replace(trimmed, `{"digest":"`+digestC+`","state":"resolved"}`, `{"state":"resolved"}`, 1), "requires field \"digest\""},
		{"not-applicable with digest present", strings.Replace(trimmed, `{"digest":"`+digestC+`","state":"resolved"}`, `{"digest":"`+digestC+`","state":"not-applicable"}`, 1), "forbids field \"digest\""},
		{"not-applicable with null digest present", strings.Replace(trimmed, `{"digest":"`+digestC+`","state":"resolved"}`, `{"digest":null,"state":"not-applicable"}`, 1), "forbids field \"digest\""},
		{"unknown policy state", strings.Replace(trimmed, `"state":"resolved"`, `"state":"pending"`, 1), "unknown policy state"},
		{"unknown field inside policy", strings.Replace(trimmed, `"state":"resolved"`, `"state":"resolved","mystery":true`, 1), "unknown field"},
		{"duplicate key inside policy", strings.Replace(trimmed, `"state":"resolved"`, `"state":"resolved","state":"resolved"`, 1), "duplicate"},
		{"policy is a scalar", strings.Replace(trimmed, `{"digest":"`+digestC+`","state":"resolved"}`, `"resolved"`, 1), "unmarshal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.raw == trimmed {
				t.Fatalf("mutation %q did not change the fixture", tt.name)
			}
			if _, err := DecodeEntry([]byte(tt.raw)); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("DecodeEntry error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

// TestDesignProvenanceV2MixedLogWithV1History proves decode/validate order
// over a log carrying a real historical v1 entry followed by a v2 entry
// (§4.1: "Mixed V1/V2 logs decode and validate in order, while no current
// writer emits V1"), and pins the v1 entry's canonical bytes unchanged by
// this correction (byte-for-byte historical preservation).
func TestDesignProvenanceV2MixedLogWithV1History(t *testing.T) {
	const goldenV1 = `{"attribution":{"unauthenticated":true},"changes":[{"after_digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","before_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","change":"replaced","target":"problem"}],"context":{"reason":"unavailable-before-context-compiler","state":"unavailable"},"digest":"sha256:38f36af1310237a7cb48c9a9074aa3b8d402913ecc78e3a09bfcc83200eccd6a","excerpts":[],"harness":"codex","operations":[{"anchor":"problem","op":"set-problem","text":"customers cannot retry safely"}],"policy_digest":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","previous_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","result_digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","schema":"verdi.design-provenance/v1","spec":"spec/widget"}
`
	first := validEntry(t)
	firstRaw, err := EncodeEntry(first)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstRaw) != goldenV1 {
		t.Fatalf("v1 entry bytes changed by the v2 correction:\ngot:  %s\nwant: %s", firstRaw, goldenV1)
	}

	second := v2Entry(t, Policy{State: PolicyResolved, Digest: digestC})
	second.PreviousDigest = first.ResultDigest
	second.ResultDigest = digestC
	if err := second.Seal(); err != nil {
		t.Fatal(err)
	}
	secondRaw, err := EncodeEntry(second)
	if err != nil {
		t.Fatal(err)
	}

	log := append(append([]byte{}, firstRaw...), secondRaw...)
	entries, err := DecodeLog(log)
	if err != nil {
		t.Fatalf("DecodeLog mixed v1/v2: %v", err)
	}
	if len(entries) != 2 || entries[0].Schema != Schema || entries[1].Schema != SchemaV2 {
		t.Fatalf("mixed log entries = %+v", entries)
	}
	if entries[1].PreviousDigest != entries[0].ResultDigest {
		t.Fatalf("mixed log chain broke: %+v", entries)
	}
}

// TestDesignProvenanceV2AgentAlwaysResolvedInvariant proves a harness-
// bearing (delegated-agent) v2 entry must always record a resolved policy —
// the not-applicable posture is reachable only through the explicit
// browser-human path, never through an agent's.
func TestDesignProvenanceV2AgentAlwaysResolvedInvariant(t *testing.T) {
	agentNotApplicable := v2Entry(t, Policy{State: PolicyResolved, Digest: digestC})
	agentNotApplicable.Harness = "codex"
	notApplicable := Policy{State: PolicyNotApplicable}
	agentNotApplicable.Policy = &notApplicable
	if err := agentNotApplicable.Seal(); err == nil || !strings.Contains(err.Error(), "delegated-agent entries must record a resolved policy") {
		t.Fatalf("Seal agent not-applicable error = %v", err)
	}

	agentResolved := v2Entry(t, Policy{State: PolicyResolved, Digest: digestC})
	agentResolved.Harness = "codex"
	if err := agentResolved.Seal(); err != nil {
		t.Fatalf("Seal agent resolved: %v", err)
	}
}

// TestDesignProvenanceV2SelfDigestBindingCoversPolicy proves the entry's own
// seal binds the Policy field: tampering it after Seal is detected exactly
// like tampering any other field (TestEntryAttributionAndOwnDigest's own
// "tampered own digest" case, extended to the new field).
func TestDesignProvenanceV2SelfDigestBindingCoversPolicy(t *testing.T) {
	e := v2Entry(t, Policy{State: PolicyResolved, Digest: digestC})
	want, err := canonjson.Digest(e.digestProjection())
	if err != nil {
		t.Fatal(err)
	}
	if e.Digest != want {
		t.Fatalf("entry digest = %q, want projection digest %q", e.Digest, want)
	}
	tampered := e
	notApplicable := Policy{State: PolicyNotApplicable}
	tampered.Policy = &notApplicable
	if err := tampered.Validate(); err == nil || !strings.Contains(err.Error(), "own digest") {
		t.Fatalf("Validate tampered policy error = %v", err)
	}
}
