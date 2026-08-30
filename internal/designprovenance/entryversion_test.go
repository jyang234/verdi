package designprovenance

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// goldenV1Entry is one LITERAL historical `verdi.design-provenance/v1`
// sidecar line, captured byte-for-byte before the v2 correction. v1 is strict
// decode-only history (doc.go, design §4.1) and its writer surfaces refuse
// it, so every v1 fixture in this package is pinned as committed bytes rather
// than sealed fresh through Seal/EncodeEntry.
const goldenV1Entry = `{"attribution":{"unauthenticated":true},"changes":[{"after_digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","before_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","change":"replaced","target":"problem"}],"context":{"reason":"unavailable-before-context-compiler","state":"unavailable"},"digest":"sha256:38f36af1310237a7cb48c9a9074aa3b8d402913ecc78e3a09bfcc83200eccd6a","excerpts":[],"harness":"codex","operations":[{"anchor":"problem","op":"set-problem","text":"customers cannot retry safely"}],"policy_digest":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","previous_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","result_digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","schema":"verdi.design-provenance/v1","spec":"spec/widget"}
`

// goldenV1Digest is goldenV1Entry's own sealed digest, pinned independently of
// the record so a silent projection change cannot pass unnoticed.
const goldenV1Digest = "sha256:38f36af1310237a7cb48c9a9074aa3b8d402913ecc78e3a09bfcc83200eccd6a"

// decodeGoldenV1 returns the historical v1 entry recovered from
// goldenV1Entry's literal bytes through the decode path — the only route by
// which a v1 entry may now come into existence.
func decodeGoldenV1(t *testing.T) Entry {
	t.Helper()
	entry, err := DecodeEntry(bytes.TrimSuffix([]byte(goldenV1Entry), []byte("\n")))
	if err != nil {
		t.Fatalf("DecodeEntry golden v1: %v", err)
	}
	return entry
}

// freshV1Entry builds a NEW, never-committed v1 entry — the exact shape the
// writer surfaces must refuse.
func freshV1Entry() Entry {
	return Entry{
		Schema:         Schema,
		Spec:           "spec/widget",
		PreviousDigest: digestA,
		ResultDigest:   digestB,
		Attribution:    governanceprincipal.NewUnauthenticatedAttribution(),
		Harness:        "codex",
		PolicyDigest:   digestC,
		Context:        UnavailableContext(),
		Operations: []Operation{{
			Op:     OpSetProblem,
			Text:   "customers cannot retry safely",
			Anchor: "problem",
		}},
		Changes:  []Change{{Target: "problem", Change: ChangeReplaced, BeforeDigest: digestA, AfterDigest: digestB}},
		Excerpts: []Excerpt{},
	}
}

// TestDesignProvenanceV1WriterSurfacesRefuseFreshRecords proves §4.1's
// decode-only contract is ENFORCED, not merely documented: every public
// surface that can mint fresh v1 bytes or a fresh v1 digest refuses the
// schema outright. v1 cannot honestly express genuine policy non-adoption
// (its required policy_digest scalar has no not-applicable arm), so a writer
// that still emitted it would be fabricating a policy identity.
func TestDesignProvenanceV1WriterSurfacesRefuseFreshRecords(t *testing.T) {
	const want = "strict decode-only history"

	// Seal must not mint a fresh v1 digest.
	fresh := freshV1Entry()
	err := fresh.Seal()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("Seal fresh v1 error = %v, want it to contain %q", err, want)
	}
	if fresh.Digest != "" {
		t.Fatalf("refused Seal still minted digest %q", fresh.Digest)
	}

	// EncodeEntry must not emit fresh v1 bytes — not even for a fully valid,
	// correctly sealed historical entry recovered from committed bytes.
	historical := decodeGoldenV1(t)
	if err := historical.Validate(); err != nil {
		t.Fatalf("historical v1 entry must still validate: %v", err)
	}
	raw, err := EncodeEntry(historical)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("EncodeEntry v1 error = %v (bytes %q), want it to contain %q", err, raw, want)
	}
	if raw != nil {
		t.Fatalf("refused EncodeEntry still returned %q", raw)
	}

	// The refusal names the schema it refuses and the schema writers use.
	if !strings.Contains(err.Error(), Schema) || !strings.Contains(err.Error(), SchemaV2) {
		t.Fatalf("refusal %q must name both %q and %q", err, Schema, SchemaV2)
	}

	// v2 — the schema every current writer emits — is unaffected.
	v2 := v2Entry(t, Policy{State: PolicyResolved, Digest: digestC})
	if _, err := EncodeEntry(v2); err != nil {
		t.Fatalf("EncodeEntry v2: %v", err)
	}
}

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
	// The v1 side is LITERAL committed history: v1 has no writer surface to
	// produce it through any more.
	v1Raw := []byte(goldenV1Entry)
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

	// The same exclusion holds for programmatically constructed entries, not
	// only decoded ones. v1's half is asserted through Validate — the
	// version-neutral checker every historical entry is verified with —
	// because Seal/EncodeEntry now refuse v1 outright before any field rule
	// is reached (TestDesignProvenanceV1WriterSurfacesRefuseFreshRecords).
	forgedV1 := decodeGoldenV1(t)
	resolved := Policy{State: PolicyResolved, Digest: digestC}
	forgedV1.Policy = &resolved
	if err := forgedV1.Validate(); err == nil || !strings.Contains(err.Error(), "v1 entry must not carry a policy field") {
		t.Fatalf("Validate v1-with-policy error = %v", err)
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
			err := func() error {
				_, err := DecodeEntry([]byte(tt.raw))
				return err
			}()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("DecodeEntry error = %v, want it to contain %q", err, tt.want)
			}
			// DecodeEntry owns the one "decoding entry" prefix;
			// Entry.UnmarshalJSON must not add a second copy of it.
			if got := strings.Count(err.Error(), "designprovenance: decoding entry"); got > 1 {
				t.Fatalf("DecodeEntry error %q repeats the decode prefix %d times, want at most 1", err, got)
			}
		})
	}
}

// TestDesignProvenanceV2MixedLogWithV1History proves decode/validate order
// over a log carrying a real historical v1 entry followed by a v2 entry
// (§4.1: "Mixed V1/V2 logs decode and validate in order, while no current
// writer emits V1"), and pins the v1 entry's canonical bytes unchanged by
// this correction (byte-for-byte historical preservation). The v1 half is a
// LITERAL committed line, never a freshly sealed one — v1 has no writer
// surface left to seal it through.
func TestDesignProvenanceV2MixedLogWithV1History(t *testing.T) {
	firstRaw := []byte(goldenV1Entry)

	// Byte preservation, proven three ways from the LITERAL committed line —
	// no fresh v1 is sealed or encoded through any writer surface here:
	//   1. DecodeEntry succeeds, and its own canonical re-encode comparison
	//      already requires the re-encode to equal these exact bytes;
	//   2. the version-neutral encoder reproduces the line byte-for-byte;
	//   3. the entry's own sealed digest still equals the pinned golden.
	first := decodeGoldenV1(t)
	if first.Schema != Schema {
		t.Fatalf("golden fixture schema = %q, want %q", first.Schema, Schema)
	}
	reencoded, err := encodeCanonical(first)
	if err != nil {
		t.Fatalf("encodeCanonical historical v1: %v", err)
	}
	if string(reencoded) != goldenV1Entry {
		t.Fatalf("v1 entry bytes changed by the v2 correction:\ngot:  %s\nwant: %s", reencoded, goldenV1Entry)
	}
	if first.Digest != goldenV1Digest {
		t.Fatalf("v1 entry digest = %q, want pinned golden %q", first.Digest, goldenV1Digest)
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

// TestDesignProvenanceV2NotApplicableRequiresBrowserHumanShape binds the
// not-applicable arm to the ONE writer shape that can reach it: the
// explicit browser-human actor's unauthenticated attribution with no
// principal id and no harness/session. A principal-carrying entry claiming
// non-adoption could never have been produced by a conforming writer (a
// resolved principal human always routes through AuthorizePolicy, which
// records a resolved digest), so it fails closed instead of passing as
// honest history.
func TestDesignProvenanceV2NotApplicableRequiresBrowserHumanShape(t *testing.T) {
	principal, err := governanceprincipal.CanonicalPrincipalID("github-org", "alice")
	if err != nil {
		t.Fatal(err)
	}
	principalAttribution, err := governanceprincipal.NewPrincipalAttribution(principal)
	if err != nil {
		t.Fatal(err)
	}
	notApplicable := Policy{State: PolicyNotApplicable}

	for _, tt := range []struct {
		name    string
		mutate  func(*Entry)
		wantErr string
	}{
		{
			"principal attribution claiming non-adoption",
			func(e *Entry) { e.Attribution = principalAttribution },
			"not-applicable policy requires the explicit unauthenticated-human shape",
		},
		{
			"delegated-agent harness claiming non-adoption",
			func(e *Entry) { e.Harness = "codex" },
			"delegated-agent entries must record a resolved policy",
		},
		{
			"harness and session claiming non-adoption",
			func(e *Entry) { e.Harness = "codex"; e.Session = "session-1" },
			"delegated-agent entries must record a resolved policy",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := v2Entry(t, Policy{State: PolicyResolved, Digest: digestC})
			e.Policy = &notApplicable
			tt.mutate(&e)
			if err := e.Seal(); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Seal(%s) error = %v, want it to contain %q", tt.name, err, tt.wantErr)
			}
		})
	}

	// The one conforming shape is still accepted, and a principal-carrying
	// RESOLVED entry (the other real writer) is untouched by the new rule.
	browserHuman := v2Entry(t, notApplicable)
	if err := browserHuman.Validate(); err != nil {
		t.Fatalf("browser-human not-applicable entry rejected: %v", err)
	}
	resolvedPrincipal := v2Entry(t, Policy{State: PolicyResolved, Digest: digestC})
	resolvedPrincipal.Attribution = principalAttribution
	if err := resolvedPrincipal.Seal(); err != nil {
		t.Fatalf("Seal principal resolved: %v", err)
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
