package designscaffold

import (
	"strings"
	"testing"
)

// mustCanonical reads an embedded canonical template through the same
// seam production callers use, failing the test on any error.
func mustCanonical(t *testing.T, filename string) []byte {
	t.Helper()
	data, err := Canonical(filename)
	if err != nil {
		t.Fatalf("Canonical(%q): %v", filename, err)
	}
	return data
}

// TestFields_EmbeddedStoryTemplate pins the D-1 field set the embedded
// canonical story.md enumerates (spec/creation-form ac-1): the FULL
// ordered descriptor slice — names AND kinds — in first-reference
// document order, deduplicated (Title and Spike both appear more than
// once in the template source; each enumerates once, at its first
// position).
func TestFields_EmbeddedStoryTemplate(t *testing.T) {
	got, err := Fields(mustCanonical(t, "story.md"))
	if err != nil {
		t.Fatalf("Fields(story.md): %v", err)
	}
	want := []Field{
		{Name: "Ref", Kind: FieldIdentity},
		{Name: "Title", Kind: FieldInput},
		{Name: "Owners", Kind: FieldInput},
		{Name: "StoryRef", Kind: FieldInput},
		{Name: "Spike", Kind: FieldStructural},
		{Name: "Problem", Kind: FieldStatement},
		{Name: "Outcome", Kind: FieldStatement},
		{Name: "Links", Kind: FieldStructural},
	}
	assertFields(t, got, want)
}

// TestFields_EmbeddedFeatureTemplate pins feature.md's D-1 field set the
// same way (no Spike, no Links — the feature template references
// neither).
func TestFields_EmbeddedFeatureTemplate(t *testing.T) {
	got, err := Fields(mustCanonical(t, "feature.md"))
	if err != nil {
		t.Fatalf("Fields(feature.md): %v", err)
	}
	want := []Field{
		{Name: "Ref", Kind: FieldIdentity},
		{Name: "Title", Kind: FieldInput},
		{Name: "Owners", Kind: FieldInput},
		{Name: "StoryRef", Kind: FieldInput},
		{Name: "Problem", Kind: FieldStatement},
		{Name: "Outcome", Kind: FieldStatement},
	}
	assertFields(t, got, want)
}

// TestFields_OverrideTemplateYieldsItsOwnFields proves the L-M12
// property the form depends on: enumeration follows the template it is
// given — a store override referencing FEWER fields, in a DIFFERENT
// order, yields exactly that template's own set in its own order, never
// the embedded canonical's.
func TestFields_OverrideTemplateYieldsItsOwnFields(t *testing.T) {
	override := []byte(`---
id: {{.Ref}}
kind: spec
title: {{printf "%q" .Title}}
owners: [team-fixed]
class: story
status: draft
outcome: { text: {{printf "%q" .Outcome}}, anchor: outcome }
problem: { text: {{printf "%q" .Problem}}, anchor: problem }
---
# {{.Title}}
`)
	got, err := Fields(override)
	if err != nil {
		t.Fatalf("Fields(override): %v", err)
	}
	want := []Field{
		{Name: "Ref", Kind: FieldIdentity},
		{Name: "Title", Kind: FieldInput},
		{Name: "Outcome", Kind: FieldStatement},
		{Name: "Problem", Kind: FieldStatement},
	}
	assertFields(t, got, want)
}

// TestFields_RangeBodyKeepsTheIteratedElementsFields pins the
// dot-context rule directly (spec/creation-form ac-1): a range pipeline
// contributes its own top-level field, while the body's relative fields
// belong to the iterated element and never enumerate.
func TestFields_RangeBodyKeepsTheIteratedElementsFields(t *testing.T) {
	tmpl := []byte(`links:
{{range .Links}}  - { type: {{.Type}}, ref: {{printf "%q" .Ref}} }
{{end}}`)
	got, err := Fields(tmpl)
	if err != nil {
		t.Fatalf("Fields(range template): %v", err)
	}
	want := []Field{{Name: "Links", Kind: FieldStructural}}
	assertFields(t, got, want)
}

// TestFields_Negative_UnknownPlaceholderFailsClosed: a placeholder
// outside the ScaffoldData contract (the guide's aspirational custom:
// placeholders) fails closed BY NAME — mirroring Render's own
// missingkey=error posture — rather than growing a form field whose
// submission cannot render (spec/creation-form ac-1's disclosed v1
// boundary).
func TestFields_Negative_UnknownPlaceholderFailsClosed(t *testing.T) {
	tmpl := []byte("custom:\n  runbook: {{.Runbook}}\n")
	_, err := Fields(tmpl)
	if err == nil {
		t.Fatal("Fields(unknown placeholder) = nil error, want a refusal naming .Runbook")
	}
	if !strings.Contains(err.Error(), "Runbook") {
		t.Fatalf("error %q does not name the unknown placeholder Runbook", err)
	}
}

// TestFields_Negative_ParseErrorFailsClosed: a syntactically broken
// template is an error, never a silently truncated descriptor list.
func TestFields_Negative_ParseErrorFailsClosed(t *testing.T) {
	_, err := Fields([]byte("{{.Title"))
	if err == nil {
		t.Fatal("Fields(broken template) = nil error, want a parse error")
	}
}

// TestFields_SafeFuncAvailable: templates using the package's own
// registered "safe" function (every embedded canonical does) enumerate
// without a function-undefined parse error.
func TestFields_SafeFuncAvailable(t *testing.T) {
	got, err := Fields([]byte("id: {{safe .Ref}}\n"))
	if err != nil {
		t.Fatalf("Fields(safe template): %v", err)
	}
	assertFields(t, got, []Field{{Name: "Ref", Kind: FieldIdentity}})
}

func assertFields(t *testing.T, got, want []Field) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("Fields = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Fields[%d] = %+v, want %+v (full: got %+v want %+v)", i, got[i], want[i], got, want)
		}
	}
}
