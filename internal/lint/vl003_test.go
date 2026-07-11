package lint

import (
	"path/filepath"
	"testing"
)

func TestVL003_DanglingLink(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-003", "dangling-link"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-003")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

func TestVL003_DanglingPin(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-003", "dangling-pin"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-003")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

// TestVL003_DanglingFragment is the R4-I-3 rescope's core new behavior: an
// object-id fragment (#<object-id>) naming a real target spec but an object
// id that spec does not declare fails VL-003, not a silent pass (the
// pre-rescope engine never resolved fragments against ByRef at all).
func TestVL003_DanglingFragment(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-003", "dangling-fragment"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-003")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

// TestVL003_ResolvingFragment_Clean proves the positive complement: a
// fragment naming a real object id on a real target resolves cleanly.
func TestVL003_ResolvingFragment_Clean(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/active/vl-003-resolving-fragment/spec.md", `---
id: spec/vl-003-resolving-fragment
kind: spec
class: story
title: "VL-003: resolving object-id fragment"
status: draft
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
story: jira:LOAN-0098
links:
  - { type: implements, ref: "spec/stale-decline#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: "#ac-1" }
---
# VL-003: resolving object-id fragment

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.
`)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-003" {
			t.Fatalf("VL-003 fired on a fragment that resolves cleanly: %s", f.String())
		}
	}
}

// TestVL003_UnknownEdgeTypeOnFragment_FailsClosed proves 02's "their edge
// types are the closed five-value enum ... unknown types fail closed"
// (VL-003's own amended row, R4-I-3): a fragment-targeting link whose type
// (here "verifies", a known link type but outside the closed
// implements/resolves/supersedes/exempts/depends-on set) is not eligible
// to target an object fragment fails VL-003. Note this is *not* caught
// anywhere else: internal/lint's walk deliberately decodes via
// artifact.DecodeStrict only, never the kind's own semantic Validate()
// (doc.go's design note), so VL-003 is the sole enforcement point.
func TestVL003_UnknownEdgeTypeOnFragment_FailsClosed(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/active/vl-003-bad-edge-type/spec.md", `---
id: spec/vl-003-bad-edge-type
kind: spec
class: story
title: "VL-003: fragment-targeting verifies edge"
status: draft
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
story: jira:LOAN-0097
links:
  - { type: implements, ref: "spec/stale-decline#ac-1" }
  - { type: verifies, ref: "spec/stale-decline#ac-2" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: "#ac-1" }
---
# VL-003: fragment-targeting verifies edge

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.
`)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-003")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1 VL-003:\n%s", len(findings), findingsString(findings))
	}
}
