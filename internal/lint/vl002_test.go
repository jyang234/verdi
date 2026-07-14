package lint

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestVL002_PathMismatch(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-002", "path-mismatch"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-002")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

// TestVL002_DuplicateRef proves the global-uniqueness sub-check fires for
// both files sharing id "adr/vl-002-duplicate". The overlay's on-disk
// filenames (vl-002-a.md / vl-002-b.md — neither can equal the shared id's
// implied filename, since two distinct files cannot both occupy that one
// path) also legitimately trip VL-002's own id/path-agreement sub-check;
// that is still exactly VL-002 firing, not a different rule, so onlyRule
// (rule-id equality) is satisfied either way.
func TestVL002_DuplicateRef(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-002", "duplicate-ref"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-002")

	dupCount := 0
	for _, f := range findings {
		if strings.Contains(f.Message, "declared by more than one file") {
			dupCount++
		}
	}
	if dupCount != 2 {
		t.Fatalf("got %d duplicate-ref findings, want 2 (one per file):\n%s", dupCount, findingsString(findings))
	}
}

// closedStorySpec is a clean, closed round-four story spec (problem/outcome
// present and resolving, AC anchored, implements edge resolving) whose only
// possible lint concern is its status-in-path placement. 02 §Lint rules'
// VL-002 row binds status-in-path to "the feature and story classes", so a
// closed story spec belongs under specs/archive/ — the same rule as a closed
// feature spec (FR-I-1). No frozen: block, so the frozen-scoped rules
// (VL-008/009/015) never engage; the placement is the sole variable.
const closedStorySpec = `---
id: spec/vl-002-closed-story
kind: spec
class: story
title: "VL-002: closed story spec, status-in-path"
status: closed
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
story: jira:LOAN-0087
links:
  - { type: implements, ref: "spec/stale-decline#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: "#ac-1" }
---
# VL-002: closed story spec, status-in-path

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.
`

// TestVL002_ClosedStoryUnderActive_Fires proves the FR-I-1 fix: a class:
// story spec with status: closed left under specs/active/ fires VL-002
// (before the fix it never did, because the status-in-path guard covered
// only class: feature). Status-in-path is the sole enforcement site for the
// active→archive invariant (no close verb performs the move), so this is the
// only place the invariant is caught.
//
// closedStorySpec deliberately carries no frozen: block (this const's own
// doc comment: "so the frozen-scoped rules VL-008/009/015 never engage") —
// but that same omission means storyresolve.LoadSpec (VL-019's own
// resolution, which fully re-validate-decodes a target, not just
// artifact.DecodeStrict) can never resolve it, so this fixture cannot ALSO
// carry a clean backing obligation the way vl019_test.go's own fixtures do
// (giving it one would only trade a VL-020 finding for a VL-019 one). VL-020
// (evidence-obligations wave 2, added after this fixture was written)
// therefore legitimately fires here too now, alongside VL-002: a real,
// non-draft story AC declaring a kind with no obligation. Both are the
// expected, correct findings; onlyRules (unlike onlyRule) guards against any
// OTHER rule storm while tolerating exactly these two.
func TestVL002_ClosedStoryUnderActive_Fires(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/active/vl-002-closed-story/spec.md", closedStorySpec)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRules(t, findings, "VL-002", "VL-020")
	if n := countRule(findings, "VL-002"); n != 1 {
		t.Fatalf("got %d VL-002 findings, want 1 (closed story belongs under specs/archive/):\n%s", n, findingsString(findings))
	}
}

// TestVL002_ClosedStoryUnderArchive_Clean is the positive complement: the
// same closed story spec correctly sitting under specs/archive/ is clean —
// VL-002 does not fire, so the fix introduces no false positive on the
// well-placed case.
func TestVL002_ClosedStoryUnderArchive_Clean(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/archive/vl-002-closed-story/spec.md", closedStorySpec)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-002" {
			t.Fatalf("VL-002 fired on a correctly-archived closed story spec: %s", f.String())
		}
	}
}

// TestVL002_AcceptedPendingUnderArchive_Fires locks in the exact defect
// symptom the round-6 close-status flip (D6-11) removes: an archived spec
// left at status: accepted-pending-build — the un-flipped state the close
// verb used to produce — fires VL-002. A terminal status (closed) is what
// belongs under specs/archive/, never accepted-pending-build. `verdi close`
// now flips the status AS the archive move, so this state never reaches the
// store; this test guards against a regression that would let it.
// closedStorySpec's own "no frozen: block" design (see
// TestVL002_ClosedStoryUnderActive_Fires's doc comment) means VL-020 also
// legitimately fires here alongside VL-002, for the same reason.
func TestVL002_AcceptedPendingUnderArchive_Fires(t *testing.T) {
	apbStorySpec := strings.Replace(closedStorySpec, "status: closed", "status: accepted-pending-build", 1)
	dir := adHocOverlayDir(t, ".verdi/specs/archive/vl-002-closed-story/spec.md", apbStorySpec)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRules(t, findings, "VL-002", "VL-020")
	if n := countRule(findings, "VL-002"); n != 1 {
		t.Fatalf("got %d VL-002 findings, want 1 (accepted-pending-build belongs under specs/active/, not archive/):\n%s", n, findingsString(findings))
	}
}
