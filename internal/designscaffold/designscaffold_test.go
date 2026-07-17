package designscaffold

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

func TestHumanizeName(t *testing.T) {
	cases := map[string]string{
		"stale-decline":     "Stale Decline",
		"loan-mgmt":         "Loan Mgmt",
		"single":            "Single",
		"":                  "",
		"leading--doubled-": "Leading  Doubled ",
	}
	for in, want := range cases {
		if got := HumanizeName(in); got != want {
			t.Errorf("HumanizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

// mustCanonicalTemplate reads the embedded canonical template bytes for
// name ("feature.md"/"story.md") directly (not through LoadTemplate, which
// also consults a store root this package's own tests have none of) —
// render_test.go exercises LoadTemplate's override-vs-embedded resolution
// itself; this package's Feature/Story tests only need the shipped
// default.
func mustCanonicalTemplate(t *testing.T, name string) []byte {
	t.Helper()
	data, err := embeddedTemplates.ReadFile("templates/" + name)
	if err != nil {
		t.Fatalf("reading embedded canonical template %q: %v", name, err)
	}
	return data
}

// TestFeature proves Feature's output self-validates as a draft feature
// spec carrying the 05 §CLI exit criterion's minimum surface (attributes,
// ACs, a stub) — the exact content cmd/verdi/design.go's `design start`
// relies on. Rendering the embedded canonical feature.md template through
// Feature and decoding the result via SplitFrontmatter + DecodeSpec is
// spec/scaffold-templates ac-1's own equivalence proof: these decoded-
// field assertions are the "field-equal to what the retired string
// builder produced" check, now that the string builder itself is gone.
func TestFeature(t *testing.T) {
	tmpl := mustCanonicalTemplate(t, "feature.md")
	for _, storyRef := range []string{"", "jira:LOAN-1482"} {
		content, err := Feature(tmpl, "spec/stale-decline", storyRef, "Stale decline handling")
		if err != nil {
			t.Fatalf("Feature: %v", err)
		}
		fm, _, err := artifact.SplitFrontmatter([]byte(content))
		if err != nil {
			t.Fatalf("SplitFrontmatter: %v", err)
		}
		spec, err := artifact.DecodeSpec(fm)
		if err != nil {
			t.Fatalf("DecodeSpec: %v", err)
		}
		if spec.Class != artifact.ClassFeature {
			t.Fatalf("Class = %q, want feature", spec.Class)
		}
		if spec.Story != storyRef {
			t.Fatalf("Story = %q, want %q", spec.Story, storyRef)
		}
		if spec.Problem == nil || spec.Outcome == nil {
			t.Fatal("Feature scaffold has no problem/outcome")
		}
		if len(spec.AcceptanceCriteria) == 0 {
			t.Fatal("Feature scaffold has no acceptance criteria")
		}
		if len(spec.Stubs) == 0 {
			t.Fatal("Feature scaffold has no stubs")
		}
	}
}

// TestStory_Plain proves Story's non-spike path: a required story: ref, a
// placeholder AC, and the caller-supplied implements link(s), rendered
// through the embedded canonical story.md template (ac-1's equivalence
// proof, story class).
func TestStory_Plain(t *testing.T) {
	tmpl := mustCanonicalTemplate(t, "story.md")
	links := []StoryLink{{Type: artifact.LinkImplements, Ref: "spec/loan-mgmt#ac-1"}}
	content, err := Story(tmpl, "spec/loan-mgmt-story", "jira:LOAN-1482", "Loan Mgmt Story", false, links)
	if err != nil {
		t.Fatalf("Story: %v", err)
	}
	fm, _, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	if spec.Class != artifact.ClassStory {
		t.Fatalf("Class = %q, want story", spec.Class)
	}
	if spec.Spike {
		t.Fatal("Spike = true, want false")
	}
	if spec.Story != "jira:LOAN-1482" {
		t.Fatalf("Story = %q, want jira:LOAN-1482", spec.Story)
	}
	if spec.Problem == nil || spec.Outcome == nil {
		t.Fatal("Story scaffold has no problem/outcome")
	}
	var foundImplements bool
	for _, l := range spec.Links {
		if l.Type == artifact.LinkImplements && l.Ref == "spec/loan-mgmt#ac-1" {
			foundImplements = true
		}
	}
	if !foundImplements {
		t.Fatalf("Story scaffold links = %+v, want the supplied implements edge", spec.Links)
	}
}

// TestStory_Spike proves Story's spike path: spike: true, no implements
// edges, no acceptance_criteria placeholder, and the caller-supplied
// resolves link(s) — validateStory's "spike carries NO implements edges"
// / ">=1 resolves edge" grammar (02 §Kind registry), rendered through the
// same embedded canonical story.md template (ac-1's equivalence proof,
// spike variant).
func TestStory_Spike(t *testing.T) {
	tmpl := mustCanonicalTemplate(t, "story.md")
	links := []StoryLink{
		{Type: artifact.LinkResolves, Ref: "spec/scoping-canvas#oq-1"},
		{Type: artifact.LinkResolves, Ref: "spec/scoping-canvas#oq-2"},
	}
	content, err := Story(tmpl, "spec/retry-strategy-spike", "todo:REPLACE-ME", "Retry Strategy Spike", true, links)
	if err != nil {
		t.Fatalf("Story: %v", err)
	}
	fm, _, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	if !spec.Spike {
		t.Fatal("Spike = false, want true")
	}
	if len(spec.AcceptanceCriteria) != 0 {
		t.Fatalf("spike scaffold declares acceptance_criteria = %+v, want none", spec.AcceptanceCriteria)
	}
	var resolvesCount int
	for _, l := range spec.Links {
		if l.Type == artifact.LinkImplements {
			t.Fatal("spike scaffold carries an implements edge, want none")
		}
		if l.Type == artifact.LinkResolves {
			resolvesCount++
		}
	}
	if resolvesCount != 2 {
		t.Fatalf("resolves edge count = %d, want 2", resolvesCount)
	}
}

// TestStory_Negative_NoLinks proves the caller's contract is enforced:
// Story renders content that fails to decode when given zero links, since
// a story with no implements/resolves edges would decode as YAML but fail
// validateStory anyway — the canonical story.md template's own "links:\n"
// followed by zero entries renders a nil Links slice, exactly like the
// retired strings.Builder version did.
func TestStory_Negative_NoLinks(t *testing.T) {
	tmpl := mustCanonicalTemplate(t, "story.md")
	content, err := Story(tmpl, "spec/x", "jira:LOAN-1", "X", false, nil)
	if err != nil {
		t.Fatalf("Story: %v", err)
	}
	fm, _, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	if _, err := artifact.DecodeSpec(fm); err == nil {
		t.Fatal("Story with no links decoded successfully, want a validateStory failure (no implements edge)")
	}
}

// legacyFeature and legacyStory are BYTE-FOR-BYTE copies of the retired
// fmt.Sprintf/strings.Builder bodies designscaffold.go's Feature/Story
// carried before spec/scaffold-templates ac-1 replaced them with template
// rendering — kept here, ONLY in this test file (never in designscaffold.go
// itself, per the ac-1 obligation's "the retired bodies are deleted from
// designscaffold.go"), as the independent reference TestByteForByte
// pins the new template path against. Do not "clean these up" into calling
// the new Render path — that would make the pin test compare a function
// against itself.

func legacyFeature(specRef, storyRef, title string) string {
	storyLine := ""
	if storyRef != "" {
		storyLine = fmt.Sprintf("\nstory: %s", storyRef)
	}
	return fmt.Sprintf(`---
id: %s
kind: spec
title: %q
owners: [unassigned]
class: feature%s
status: draft
problem: { text: "TODO: replace with the real problem statement before accept", anchor: problem }
outcome: { text: "TODO: replace with the real outcome statement before accept", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "TODO: replace with real acceptance criteria before accept", evidence: [static, attestation], anchor: ac-1 }
stubs:
  - { slug: todo-replace-stub-slug, acceptance_criteria: [ac-1] }
---
# %s

## Problem

TODO: design notes.

## Outcome

TODO: design notes.

## Ac 1

TODO: design notes.
`, specRef, title, storyLine, title)
}

func legacyStory(specRef, storyRef, title string, spike bool, links []StoryLink) string {
	var b strings.Builder
	fmt.Fprintf(&b, `---
id: %s
kind: spec
title: %q
owners: [unassigned]
class: story
status: draft
story: %s
`, specRef, title, storyRef)
	if spike {
		b.WriteString("spike: true\n")
	}
	b.WriteString(`problem: { text: "TODO: replace with the real problem statement before accept", anchor: problem }
outcome: { text: "TODO: replace with the real outcome statement before accept", anchor: outcome }
`)
	if !spike {
		b.WriteString(`acceptance_criteria:
  - { id: ac-1, text: "TODO: replace with real acceptance criteria before accept", evidence: [static], anchor: ac-1 }
`)
	}
	b.WriteString("links:\n")
	for _, l := range links {
		fmt.Fprintf(&b, "  - { type: %s, ref: %q }\n", l.Type, l.Ref)
	}
	fmt.Fprintf(&b, `---
# %s

## Problem

TODO: design notes.

## Outcome

TODO: design notes.
`, title)
	if !spike {
		b.WriteString(`
## Ac 1

TODO: design notes.
`)
	}
	return b.String()
}

// TestByteForByte pins the stronger property spec/scaffold-templates'
// outcome text promises (ac-1's own floor is decode-equivalence only,
// proven above): the embedded canonical templates reproduce the retired
// string builders' output BYTE FOR BYTE, not merely field-equal after
// decode. Each case renders through the new template path and through
// the frozen legacy reference above from IDENTICAL inputs and asserts
// exact string equality.
func TestByteForByte(t *testing.T) {
	featureTmpl := mustCanonicalTemplate(t, "feature.md")
	storyTmpl := mustCanonicalTemplate(t, "story.md")

	t.Run("feature, no story ref", func(t *testing.T) {
		got, err := Feature(featureTmpl, "spec/stale-decline", "", "Stale decline handling")
		if err != nil {
			t.Fatalf("Feature: %v", err)
		}
		want := legacyFeature("spec/stale-decline", "", "Stale decline handling")
		if got != want {
			t.Fatalf("Feature output does not match the retired string builder byte-for-byte\ngot:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("feature, with story ref", func(t *testing.T) {
		got, err := Feature(featureTmpl, "spec/loan-mgmt", "jira:LOAN-1482", "Loan Mgmt")
		if err != nil {
			t.Fatalf("Feature: %v", err)
		}
		want := legacyFeature("spec/loan-mgmt", "jira:LOAN-1482", "Loan Mgmt")
		if got != want {
			t.Fatalf("Feature output does not match the retired string builder byte-for-byte\ngot:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("story, plain, one link", func(t *testing.T) {
		links := []StoryLink{{Type: artifact.LinkImplements, Ref: "spec/loan-mgmt#ac-1"}}
		got, err := Story(storyTmpl, "spec/loan-mgmt-story", "jira:LOAN-1482", "Loan Mgmt Story", false, links)
		if err != nil {
			t.Fatalf("Story: %v", err)
		}
		want := legacyStory("spec/loan-mgmt-story", "jira:LOAN-1482", "Loan Mgmt Story", false, links)
		if got != want {
			t.Fatalf("Story output does not match the retired string builder byte-for-byte\ngot:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("story, plain, zero links", func(t *testing.T) {
		got, err := Story(storyTmpl, "spec/x", "jira:LOAN-1", "X", false, nil)
		if err != nil {
			t.Fatalf("Story: %v", err)
		}
		want := legacyStory("spec/x", "jira:LOAN-1", "X", false, nil)
		if got != want {
			t.Fatalf("Story output does not match the retired string builder byte-for-byte\ngot:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("story, spike, two links", func(t *testing.T) {
		links := []StoryLink{
			{Type: artifact.LinkResolves, Ref: "spec/scoping-canvas#oq-1"},
			{Type: artifact.LinkResolves, Ref: "spec/scoping-canvas#oq-2"},
		}
		got, err := Story(storyTmpl, "spec/retry-strategy-spike", "todo:REPLACE-ME", "Retry Strategy Spike", true, links)
		if err != nil {
			t.Fatalf("Story: %v", err)
		}
		want := legacyStory("spec/retry-strategy-spike", "todo:REPLACE-ME", "Retry Strategy Spike", true, links)
		if got != want {
			t.Fatalf("Story output does not match the retired string builder byte-for-byte\ngot:\n%s\nwant:\n%s", got, want)
		}
	})
}
