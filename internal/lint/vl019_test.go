package lint

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestVL019_VerifiesFeatureAC is the primary testdata/violations overlay:
// an obligation whose verifies edge targets spec/stale-decline#ac-1 —
// stale-decline is class: feature in the golden corpus, so this is a
// FEATURE ac. Obligations attach to STORY acceptance criteria only (03
// §The feature fold).
func TestVL019_VerifiesFeatureAC(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-019"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-019")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	if !strings.Contains(findings[0].Message, "spec/stale-decline#ac-1") {
		t.Errorf("finding does not name the offending target: %s", findings[0].Message)
	}
	if !strings.Contains(findings[0].Message, "obligation/stale-decline--ac-1--static") {
		t.Errorf("finding does not name the offending obligation: %s", findings[0].Message)
	}
}

const vl019WholeSpecMD = `---
id: obligation/stale-decline--ac-4--runtime
kind: obligation
title: "VL-019: obligation verifies a whole spec"
owners: [platform-team]
for_kind: runtime
links:
  - { type: verifies, ref: "spec/stale-decline" }
frozen: { at: 2026-07-13, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# VL-019: obligation verifies a whole spec

No object-id fragment at all — VL-019 must refuse this: obligations attach
to a STORY acceptance criterion specifically, never a whole spec.
`

// TestVL019_VerifiesWholeSpec is the "whole spec" refusal shape: the
// verifies ref names a real spec (the golden corpus's own stale-decline),
// but carries no object-id fragment at all.
func TestVL019_VerifiesWholeSpec(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/obligations/stale-decline/ac-4--runtime.md", vl019WholeSpecMD)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-019")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	if !strings.Contains(findings[0].Message, "spec/stale-decline") {
		t.Errorf("finding does not name the offending target: %s", findings[0].Message)
	}
}

// vl019NonACSpecMD is a minimal ad hoc feature spec declaring one
// acceptance criterion (ac-1) and one CONSTRAINT (co-1) — neither
// borrower-update-api nor accepted-pending-build (which would otherwise
// supply a ready-made non-AC object) is wired into testdata/corpus's own
// layers.txt, so this test builds its own small fixture, mirroring
// vl018_test.go's vl018CleanSpec template (problem/outcome present, every
// declared object anchored and resolving against a real body heading, so
// VL-006's new-class requiredness check stays silent).
const vl019NonACSpecMD = `---
id: spec/vl-019-nonac
kind: spec
class: feature
title: "VL-019: feature spec with a non-AC object"
status: draft
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: "#ac-1" }
constraints:
  - { id: co-1, text: "placeholder constraint", anchor: "#co-1" }
---
# VL-019: feature spec with a non-AC object

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.

## CO-1

Placeholder constraint.
`

const vl019NonACFragmentMD = `---
id: obligation/vl-019-nonac--ac-1--static
kind: obligation
title: "VL-019: obligation verifies a non-AC fragment"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/vl-019-nonac#co-1" }
frozen: { at: 2026-07-13, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# VL-019: obligation verifies a non-AC fragment

spec/vl-019-nonac#co-1 is a declared CONSTRAINT, not an acceptance
criterion — VL-019 must refuse this regardless of the target spec's class.
`

// TestVL019_VerifiesNonACFragment is the "non-AC fragment" refusal shape:
// the target resolves, and the fragment id itself resolves (VL-003 is
// satisfied — co-1 is a genuinely declared object), but it names a
// constraint, not one of the target spec's own declared acceptance
// criteria.
func TestVL019_VerifiesNonACFragment(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, ".verdi/specs/active/vl-019-nonac/spec.md"), vl019NonACSpecMD)
	writeTestFile(t, filepath.Join(dir, ".verdi/obligations/vl-019-nonac/ac-1--static.md"), vl019NonACFragmentMD)

	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-019")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	if !strings.Contains(findings[0].Message, "spec/vl-019-nonac#co-1") {
		t.Errorf("finding does not name the offending target: %s", findings[0].Message)
	}
	if !strings.Contains(findings[0].Message, "non-AC fragment") {
		t.Errorf("finding does not say non-AC fragment: %s", findings[0].Message)
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
  - { type: verifies, ref: "spec/vl-019-story#ac-1" }
frozen: { at: 2026-07-13, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# VL-019: obligation verifies a real STORY AC

spec/vl-019-story is class: story and declares ac-1 itself — VL-019 must
accept this obligation cleanly.
`

// TestVL019_VerifiesStoryAC_Clean is the positive complement: an obligation
// whose verifies edge targets a real STORY acceptance criterion is
// accepted — VL-019 never fires.
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

const vl019DanglingSpecMD = `---
id: obligation/no-such-story--ac-1--static
kind: obligation
title: "VL-019: obligation verifies a spec that does not exist"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/no-such-story#ac-1" }
frozen: { at: 2026-07-13, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
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
			if !strings.Contains(f.Message, "spec/no-such-story#ac-1") {
				t.Errorf("finding does not name the offending target: %s", f.Message)
			}
		}
	}
	if !found {
		t.Fatalf("VL-019 did not fire on an unresolvable verifies target:\n%s", findingsString(findings))
	}
}
