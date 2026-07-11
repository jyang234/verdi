package lint

import (
	"path/filepath"
	"testing"
)

func TestVL014_MissingSticky(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-014", "missing-sticky"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-014")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

func TestVL014_DanglingDisposition(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-014", "dangling-disposition"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-014")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

func TestVL014_IncorporatedWithoutWhere(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-014", "incorporated-without-where"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-014")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

func TestVL014_ContradictedWithoutNote(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-014", "contradicted-without-note"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-014")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

func TestVL014_UnresolvableWhereAnchor(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-014", "unresolvable-where-anchor"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-014")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

// vl014NewStyleNoDispositionsSpec is a new-style feature spec (round-four
// surface, no dispositions: block at all) sharing its directory with a
// leftover board.json whose sticky the spec doesn't mention anywhere.
const vl014NewStyleNoDispositionsSpec = `---
id: spec/vl-014-new-style
kind: spec
class: feature
title: "VL-014 grandfather-scope-negative: new-style spec, no dispositions"
status: draft
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: "#ac-1" }
---
# VL-014 grandfather-scope-negative: new-style spec, no dispositions

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.
`

const vl014NewStyleBoardJSON = `{
  "schema": "verdi.board/v1",
  "pins": [],
  "stickies": [
    { "id": "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", "x": 10, "y": 10 }
  ],
  "yarn": []
}
`

// TestVL014_NewStyleSpec_NoDispositionsBlock_NeverFires is the exit
// criterion's "grandfather scope proven negative": a new-style spec (no
// dispositions: block) never trips VL-014, even when its board state (a
// stale/leftover sibling board.json) and spec disagree (R4-I-9).
func TestVL014_NewStyleSpec_NoDispositionsBlock_NeverFires(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, ".verdi/specs/active/vl-014-new-style/spec.md"), vl014NewStyleNoDispositionsSpec)
	writeTestFile(t, filepath.Join(dir, ".verdi/specs/active/vl-014-new-style/board.json"), vl014NewStyleBoardJSON)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-014" {
			t.Fatalf("VL-014 fired on a new-style spec with no dispositions: block: %s", f.String())
		}
	}
}
