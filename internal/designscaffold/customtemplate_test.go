package designscaffold

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/artifact/splice"
)

// overrideStoryTemplateWithCustom is a store's own .verdi/templates/
// story.md override (spec/scaffold-templates ac-2's own worked example):
// it adds a "Rollout Plan" body section and a custom: frontmatter field
// carrying a real value, in place of the embedded canonical story.md.
const overrideStoryTemplateWithCustom = `---
id: {{.Ref}}
kind: spec
title: {{printf "%q" .Title}}
owners: {{.Owners}}
class: story
status: draft
story: {{.StoryRef}}
{{if .Spike}}spike: true
{{end}}problem: { text: "{{.Problem}}", anchor: problem }
outcome: { text: "{{.Outcome}}", anchor: outcome }
{{if not .Spike}}acceptance_criteria:
  - { id: ac-1, text: "TODO: replace with real acceptance criteria before accept", evidence: [static], anchor: ac-1 }
{{end}}links:
{{range .Links}}  - { type: {{.Type}}, ref: {{printf "%q" .Ref}} }
{{end}}custom:
  rollout_plan: "canary then full rollout"
---
# {{.Title}}

## Problem

TODO: design notes.

## Outcome

TODO: design notes.

## Rollout Plan

TODO: fill in the rollout plan.
{{if not .Spike}}
## Ac 1

TODO: design notes.
{{end}}`

// TestRender_StoreOverrideAddsCustomAndSection drives designscaffold's
// render path directly (spec/scaffold-templates ac-2, obligation's first
// reachable-call-site leg): a store override template that adds a body
// section and a custom: field scaffolds a spec carrying both, in place of
// the embedded canonical story.md.
func TestRender_StoreOverrideAddsCustomAndSection(t *testing.T) {
	links := []StoryLink{{Type: artifact.LinkImplements, Ref: "spec/loan-mgmt#ac-1"}}
	content, err := Story([]byte(overrideStoryTemplateWithCustom), "spec/loan-mgmt-story", "jira:LOAN-1482", "Loan Mgmt Story", false, links)
	if err != nil {
		t.Fatalf("Story (store override template): %v", err)
	}

	if !strings.Contains(content, "## Rollout Plan") {
		t.Fatalf("scaffolded body does not carry the override's added section:\n%s", content)
	}

	fm, _, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	if got := spec.Custom["rollout_plan"]; got != "canary then full rollout" {
		t.Fatalf(`spec.Custom["rollout_plan"] = %#v, want "canary then full rollout"`, got)
	}
}

// TestRender_StoreOverrideCustom_SurvivesSpliceRoundTrip proves the round
// trip the ac-2 obligation names: the scaffolded spec's custom: content,
// put through artifact.DecodeSpec and then the canonical re-emit path —
// internal/artifact/splice, this module's ONLY write path for an existing
// spec document (splice/doc.go: "never decode->struct->yaml.Marshal->
// reassemble"; Validate is its own strict-re-decode-before-write gate) —
// comes back unchanged after an ordinary, UNRELATED edit lands elsewhere
// in the same document. This is the same strict-decode-then-re-emit
// property every other frontmatter field already holds today: splice
// edits are surgical byte-range replacements against the pristine buffer,
// so a byte range it never touches (custom:) cannot be dropped or mangled
// by an edit to some other object.
func TestRender_StoreOverrideCustom_SurvivesSpliceRoundTrip(t *testing.T) {
	links := []StoryLink{{Type: artifact.LinkImplements, Ref: "spec/loan-mgmt#ac-1"}}
	content, err := Story([]byte(overrideStoryTemplateWithCustom), "spec/loan-mgmt-story", "jira:LOAN-1482", "Loan Mgmt Story", false, links)
	if err != nil {
		t.Fatalf("Story (store override template): %v", err)
	}

	doc, err := splice.Parse([]byte(content))
	if err != nil {
		t.Fatalf("splice.Parse: %v", err)
	}
	// An ordinary board edit to a DIFFERENT object (ac-1's own text) —
	// custom: is untouched byte-range, so its survival here is not a
	// coincidence of the edit chosen, it is splice's whole architecture.
	edit, err := doc.SetObjectText("ac-1", "Replaced acceptance criterion text")
	if err != nil {
		t.Fatalf("SetObjectText: %v", err)
	}
	out, err := doc.Apply([]splice.Edit{edit})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	// splice.Validate is the validate-before-write gate every real board
	// write already runs (boardSpecServer.spliceSpec) — the "canonical
	// re-emit path" a spec goes through today.
	if err := splice.Validate(out); err != nil {
		t.Fatalf("Validate(spliced result): %v", err)
	}

	fm, _, err := artifact.SplitFrontmatter(out)
	if err != nil {
		t.Fatalf("SplitFrontmatter(spliced result): %v", err)
	}
	respec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec(spliced result): %v", err)
	}

	// The edit actually landed (proving this was a real, unrelated edit,
	// not a no-op)...
	var editedAC string
	for _, ac := range respec.AcceptanceCriteria {
		if ac.ID == "ac-1" {
			editedAC = ac.Text
		}
	}
	if editedAC != "Replaced acceptance criterion text" {
		t.Fatalf("ac-1 text after splice = %q, want the edited text (proves this was a real edit)", editedAC)
	}
	// ...while custom: survived the same round trip byte-for-byte
	// unchanged.
	if got := respec.Custom["rollout_plan"]; got != "canary then full rollout" {
		t.Fatalf(`Custom["rollout_plan"] after splice round trip = %#v, want it unchanged at "canary then full rollout"`, got)
	}
}
