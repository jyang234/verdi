package artifact

import (
	"os"
	"strings"
	"testing"
)

// customFeatureYAML is a minimal, otherwise-ordinary draft feature spec
// carrying a custom: block with a scalar, a nested map, and a list — the
// shape a team's own template-added section might populate (spec/
// scaffold-templates ac-2).
const customFeatureYAML = `
id: spec/custom-carrying-feature
kind: spec
class: feature
title: "Custom-carrying feature"
status: draft
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "does the thing", evidence: [static] }
custom:
  rollout_plan: "canary then full rollout"
  contacts:
    primary: "platform-team"
  reviewers: [alice, bob]
`

// TestBase_Custom_DecodesWithValues proves custom: decodes into Base.Custom
// with its nested shape intact — scalar, nested map, and list values all
// come through as the generic Go values yaml.v3 produces for map[string]any
// (spec/scaffold-templates ac-2's core decode proof).
func TestBase_Custom_DecodesWithValues(t *testing.T) {
	fm, err := DecodeSpec([]byte(customFeatureYAML))
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	if fm.Custom == nil {
		t.Fatal("Custom is nil, want the decoded custom: block")
	}
	if got := fm.Custom["rollout_plan"]; got != "canary then full rollout" {
		t.Fatalf(`Custom["rollout_plan"] = %#v, want "canary then full rollout"`, got)
	}
	contacts, ok := fm.Custom["contacts"].(map[string]any)
	if !ok {
		t.Fatalf(`Custom["contacts"] = %#v (%T), want a nested map`, fm.Custom["contacts"], fm.Custom["contacts"])
	}
	if contacts["primary"] != "platform-team" {
		t.Fatalf(`Custom["contacts"]["primary"] = %#v, want "platform-team"`, contacts["primary"])
	}
	reviewers, ok := fm.Custom["reviewers"].([]any)
	if !ok || len(reviewers) != 2 {
		t.Fatalf(`Custom["reviewers"] = %#v, want a 2-element list`, fm.Custom["reviewers"])
	}
}

// TestBase_Custom_AbsentIsNil proves a spec with no custom: block at all
// still decodes cleanly with a nil Custom map (omitempty; the "absence
// changes nothing" posture this story's outcome promises for a store with
// no template overrides also holds field-by-field here).
func TestBase_Custom_AbsentIsNil(t *testing.T) {
	fm, err := DecodeSpec([]byte(featureSpecDraftYAML))
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	if fm.Custom != nil {
		t.Fatalf("Custom = %#v, want nil when the document carries no custom: key", fm.Custom)
	}
}

// TestBase_Custom_UnknownKeyOutsideStillFails proves declaring Custom is a
// single named exemption, not a general KnownFields carve-out: a bogus
// top-level key that is NOT `custom` still fails strict decode exactly as
// before.
func TestBase_Custom_UnknownKeyOutsideStillFails(t *testing.T) {
	const y = `
id: spec/foo
kind: spec
class: feature
title: Foo
status: draft
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: a, evidence: [static] }
rollout_plan: "not inside custom, so still unknown"
`
	if _, err := DecodeSpec([]byte(y)); err == nil {
		t.Fatal("DecodeSpec with a bogus top-level key outside custom: want an error, got nil")
	}
}

// nonSpecCustomAttestationYAML is a minimal, otherwise-valid attestation
// (modeled on attestation_test.go's own valid literal) carrying a custom:
// block. The custom: namespace is SPEC-only (spec/scaffold-templates ac-2
// sanctions it for spec content; "no sanctioned extension surface for spec
// content at all" is what it opens, nothing wider): every other kind that
// embeds Base keeps its fully-strict posture, so this attestation must fail
// strict decode. Fixture for judged-custom-namespace-widened-to-all-base-
// kinds — the exemption belongs on SpecFrontmatter, not on Base.
const nonSpecCustomAttestationYAML = `id: attestation/story-1482--ac-2
kind: attestation
title: "AC-2 attested by QA lead"
owners: [qa-lead]
frozen: { at: 2026-05-01, commit: 3e91ab2 }
custom:
  rollout_plan: "canary then full rollout"
`

// TestAttestation_CustomKeyFailsStrictDecode proves custom: is not a free
// pass on non-spec kinds: an attestation carrying custom: fails strict
// decode, naming the unknown field (KnownFields) — the exemption is
// SpecFrontmatter-scoped, never a Base-wide carve-out that would loosen
// attestations/waivers/obligations/ADRs whose strict shape is load-bearing
// for the evidence model.
func TestAttestation_CustomKeyFailsStrictDecode(t *testing.T) {
	_, err := DecodeAttestation([]byte(nonSpecCustomAttestationYAML))
	if err == nil {
		t.Fatal("DecodeAttestation(attestation with custom:) = nil error, want a strict-decode failure (custom: is spec-only)")
	}
	if !strings.Contains(err.Error(), "custom") {
		t.Fatalf("error = %q, want it to name the unknown custom field", err.Error())
	}
}

// TestAttestation_NoCustomStillDecodes is the baseline the test above
// contrasts against: the identical attestation with the custom: block
// removed decodes cleanly, proving custom: is the SOLE reason the decode
// above fails — not some other invalid field.
func TestAttestation_NoCustomStillDecodes(t *testing.T) {
	const y = `id: attestation/story-1482--ac-2
kind: attestation
title: "AC-2 attested by QA lead"
owners: [qa-lead]
frozen: { at: 2026-05-01, commit: 3e91ab2 }
`
	if _, err := DecodeAttestation([]byte(y)); err != nil {
		t.Fatalf("DecodeAttestation(attestation without custom:) = %v, want it to decode cleanly", err)
	}
}

// TestBase_Custom_DialectViolationInsideFailsClosed drives the committed
// violation fixture (testdata/viol-custom-dialect-anchor.md, mirroring
// internal/model/testdata's one-fixture-per-rule convention): a YAML
// anchor inside a custom: block still fails the restricted frontmatter
// dialect wall (operating-model dc-2, spec/scaffold-templates ac-2) even
// though custom: is now a known Base field — checkDialect walks the raw
// node tree before any struct decode happens, so it has no notion of
// "inside a free-form namespace" to exempt.
func TestBase_Custom_DialectViolationInsideFailsClosed(t *testing.T) {
	raw, err := os.ReadFile("testdata/viol-custom-dialect-anchor.md")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	fm, _, err := SplitFrontmatter(raw)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	_, err = DecodeSpec(fm)
	if err == nil {
		t.Fatal("DecodeSpec(anchor inside custom:) = nil error, want a dialect violation failure")
	}
	if !strings.Contains(err.Error(), "dialect violation") || !strings.Contains(err.Error(), "anchor") {
		t.Fatalf("error = %q, want it to name the dialect violation and the anchor (never a bare/paraphrased failure)", err.Error())
	}
}
