package lint

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestVL019_VerifiesFeatureAC is the primary testdata/violations overlay:
// an obligation whose verifies edge targets the whole spec spec/stale-decline
// — stale-decline is class: feature in the golden corpus, so it is not a
// STORY. Obligations attach to STORY acceptance criteria only (03 §The
// feature fold); the AC is named by the obligation's own id, not the edge.
func TestVL019_VerifiesFeatureAC(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-019"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-019")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	if !strings.Contains(findings[0].Message, "spec/stale-decline") {
		t.Errorf("finding does not name the offending target: %s", findings[0].Message)
	}
	if !strings.Contains(findings[0].Message, "obligation/stale-decline--ac-1--static") {
		t.Errorf("finding does not name the offending obligation: %s", findings[0].Message)
	}
}

// vl019StorySpecMD is a minimal ad hoc STORY spec declaring one real
// acceptance criterion (ac-1), mirroring vl003_test.go's own
// TestVL003_ResolvingFragment_Clean fixture (problem/outcome, a story:
// tracker ref, and an implements edge into the golden corpus's own
// stale-decline#ac-1 so this story itself decodes and lints cleanly).
const vl019StorySpecMD = `---
id: spec/vl-019-story
kind: spec
class: story
title: "VL-019: story spec with a real AC"
status: draft
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
story: jira:LOAN-0199
links:
  - { type: implements, ref: "spec/stale-decline#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: "#ac-1" }
---
# VL-019: story spec with a real AC

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.
`

const vl019StoryACCleanMD = `---
id: obligation/vl-019-story--ac-1--behavioral
kind: obligation
title: "VL-019: obligation verifies a real STORY AC"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/vl-019-story" }
frozen: { at: 2026-07-13, commit: 66588948af8b36c02c8fb8f423645afa0a58dbe4 }
---
# VL-019: obligation verifies a real STORY AC

spec/vl-019-story is class: story and declares ac-1 itself; this obligation
verifies the whole story spec and its id names ac-1 — VL-019 must accept it
cleanly (the AC lives in the id, mirroring an attestation).
`

// TestVL019_VerifiesStoryAC_Clean is the positive complement: an obligation
// that verifies a whole STORY spec whose own declared acceptance criteria
// include the id's <ac-id> is accepted — VL-019 never fires.
func TestVL019_VerifiesStoryAC_Clean(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, ".verdi/specs/active/vl-019-story/spec.md"), vl019StorySpecMD)
	writeTestFile(t, filepath.Join(dir, ".verdi/obligations/vl-019-story/ac-1--behavioral.md"), vl019StoryACCleanMD)

	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-019" {
			t.Fatalf("VL-019 fired on an obligation verifying a real STORY AC: %s", f.String())
		}
	}
}

const vl019StoryBadACMD = `---
id: obligation/vl-019-story--ac-9--static
kind: obligation
title: "VL-019: obligation id names an AC the story does not declare"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/vl-019-story" }
frozen: { at: 2026-07-13, commit: 66588948af8b36c02c8fb8f423645afa0a58dbe4 }
---
# VL-019: obligation id names an AC the story does not declare

spec/vl-019-story is class: story but declares only ac-1. This obligation
verifies the whole story spec, yet its own id names ac-9, which the story
does not declare as an acceptance criterion — VL-019 must refuse it (the AC
lives in the id, not the edge).
`

// TestVL019_IDNamesNonexistentAC is the "story spec, but the obligation's own
// id names an AC that story does not declare" refusal shape — the whole-spec
// analogue of the old non-AC-fragment case: the verifies target resolves to a
// real STORY, but the ac-id parsed from the obligation's id is not one of
// that story's declared acceptance criteria.
func TestVL019_IDNamesNonexistentAC(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, ".verdi/specs/active/vl-019-story/spec.md"), vl019StorySpecMD)
	writeTestFile(t, filepath.Join(dir, ".verdi/obligations/vl-019-story/ac-9--static.md"), vl019StoryBadACMD)

	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-019")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	if !strings.Contains(findings[0].Message, "spec/vl-019-story") {
		t.Errorf("finding does not name the offending target: %s", findings[0].Message)
	}
	if !strings.Contains(findings[0].Message, "ac-9") {
		t.Errorf("finding does not name the id's undeclared ac: %s", findings[0].Message)
	}
	if !strings.Contains(findings[0].Message, "does not declare") {
		t.Errorf("finding does not explain the ac is undeclared: %s", findings[0].Message)
	}
}

const vl019DanglingSpecMD = `---
id: obligation/no-such-story--ac-1--static
kind: obligation
title: "VL-019: obligation verifies a spec that does not exist"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/no-such-story" }
frozen: { at: 2026-07-13, commit: 66588948af8b36c02c8fb8f423645afa0a58dbe4 }
---
# VL-019: obligation verifies a spec that does not exist
`

// TestVL019_VerifiesUnresolvableSpec_FailsClosed proves the fail-closed
// posture for a verifies target that does not resolve at all (mirroring
// supersedesTargetsStory/supersedesTargetsFeature's own "fail closed toward
// no-flip, never toward one"): VL-019 must still fire (this rule does not
// silently pass an obligation just because its target cannot even be
// loaded). VL-003 also fires on the same dangling ref — this test checks
// VL-019's own presence, not rule exclusivity.
func TestVL019_VerifiesUnresolvableSpec_FailsClosed(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/obligations/no-such-story/ac-1--static.md", vl019DanglingSpecMD)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	found := false
	for _, f := range findings {
		if f.Rule == "VL-019" {
			found = true
			if !strings.Contains(f.Message, "spec/no-such-story") {
				t.Errorf("finding does not name the offending target: %s", f.Message)
			}
		}
	}
	if !found {
		t.Fatalf("VL-019 did not fire on an unresolvable verifies target:\n%s", findingsString(findings))
	}
}

const vl019FragmentFormMD = `---
id: obligation/stale-decline--ac-1--static
kind: obligation
title: "VL-019: obligation verifies a fragment (the old, invalid form)"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/stale-decline#ac-1" }
frozen: { at: 2026-07-13, commit: 66588948af8b36c02c8fb8f423645afa0a58dbe4 }
---
# VL-019: obligation verifies a fragment (the old, invalid form)

The canonical verifies form is a whole-spec ref (the AC is named by the id).
A fragment-bearing verifies edge is the old, now-invalid form: VL-003's
closed spec-object edge vocabulary rejects it, and VL-019 refuses it as not a
whole story-spec ref. This test checks VL-019's own presence, not rule
exclusivity.
`

// TestVL019_VerifiesFragment_FailsClosed proves the migration guard: an
// obligation still authored in the old fragment-bearing form
// (verifies: spec/<story>#ac-1) is refused. VL-019 fails closed on it as not
// a whole story-spec ref; VL-003 independently rejects the fragment via the
// closed edge vocabulary, so both fire.
func TestVL019_VerifiesFragment_FailsClosed(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/obligations/stale-decline/ac-1--static.md", vl019FragmentFormMD)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	found := false
	for _, f := range findings {
		if f.Rule == "VL-019" {
			found = true
			if !strings.Contains(f.Message, "spec/stale-decline#ac-1") {
				t.Errorf("finding does not name the offending target: %s", f.Message)
			}
			if !strings.Contains(f.Message, "whole story-spec ref") {
				t.Errorf("finding does not explain the whole-spec requirement: %s", f.Message)
			}
		}
	}
	if !found {
		t.Fatalf("VL-019 did not fire on a fragment-bearing verifies target:\n%s", findingsString(findings))
	}
}
