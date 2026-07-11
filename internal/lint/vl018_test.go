package lint

import (
	"path/filepath"
	"testing"
)

// TestVL018_DanglingPositionKey is the primary negative case: a
// layout.json positions key that does not resolve to any declared object
// id on the sibling spec.
func TestVL018_DanglingPositionKey(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-018"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-018")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

const vl018CleanSpec = `---
id: spec/vl-018-clean
kind: spec
class: feature
title: "VL-018: layout.json keys all resolve"
status: draft
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: "#ac-1" }
constraints:
  - { id: co-1, text: "placeholder constraint", anchor: "#co-1" }
---
# VL-018: layout.json keys all resolve

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.

## CO-1

Placeholder constraint.
`

const vl018CleanLayoutJSON = `{
  "schema": "verdi.boardlayout/v1",
  "positions": {
    "ac-1": { "x": 40, "y": 20 }
  }
}
`

// TestVL018_AllKeysResolve_Clean is the positive complement: every
// positions key resolves to a real declared object id.
func TestVL018_AllKeysResolve_Clean(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, ".verdi/specs/active/vl-018-clean/spec.md"), vl018CleanSpec)
	writeTestFile(t, filepath.Join(dir, ".verdi/specs/active/vl-018-clean/layout.json"), vl018CleanLayoutJSON)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-018" {
			t.Fatalf("VL-018 fired on a layout.json whose keys all resolve: %s", f.String())
		}
	}
}

// TestVL018_NoLayoutJSON_NeverFires proves VL-018 "never gates on absence"
// (R4-I-5): a spec directory with no layout.json sidecar at all is simply
// not this rule's concern.
func TestVL018_NoLayoutJSON_NeverFires(t *testing.T) {
	repo := buildLintRepo(t) // golden corpus alone, no layout.json anywhere
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-018" {
			t.Fatalf("VL-018 fired with no layout.json present anywhere: %s", f.String())
		}
	}
}
