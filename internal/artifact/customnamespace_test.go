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
