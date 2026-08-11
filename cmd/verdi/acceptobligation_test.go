package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/fixturegit"
)

// obligationSeamStoryCleanMD is a fully lint-clean draft story spec: two
// ACs, two different declared evidence kinds, neither with a pre-existing
// obligation — scaffoldMissingObligations' own core happy-path shape. An
// implements edge to someFeatureMD (supersedepredecessor_test.go) so the
// link resolves rather than dangling.
const obligationSeamStoryCleanMD = `---
id: spec/widget-story
kind: spec
title: "Widget story"
owners: [platform-team]
class: story
status: draft
story: jira:LOAN-9001
problem: { text: "widgets are stale", anchor: problem }
outcome: { text: "widgets are current", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "static evidence holds", evidence: [static], anchor: ac-1 }
  - { id: ac-2, text: "behavioral evidence holds", evidence: [behavioral], anchor: ac-2 }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
---
# Widget story

## Problem

Widgets are stale.

## Outcome

Widgets are current.

## AC-1

Static evidence holds.

## AC-2

Behavioral evidence holds.
`

const preExistingAc1StaticMD = `---
id: obligation/widget-story--ac-1--static
kind: obligation
title: "PRE-EXISTING-DISTINCTIVE-MARKER"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/widget-story" }
frozen: { at: 2026-01-01, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# PRE-EXISTING-DISTINCTIVE-MARKER

A hand-authored obligation that must never be clobbered.
`

// malformedAc2BehavioralMD sits at ac-2's exact convention path but fails
// artifact.DecodeObligation: its for_kind ("static") disagrees with its own
// id's "--behavioral" segment (DC-2's id/for_kind agreement) — the
// present-but-undecodable case spec/obligation-seam ac-2's third case
// covers.
const malformedAc2BehavioralMD = `---
id: obligation/widget-story--ac-2--behavioral
kind: obligation
title: "malformed on purpose"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/widget-story" }
frozen: { at: 2026-01-01, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# malformed on purpose

for_kind disagrees with the id's own --behavioral segment.
`

// misnamedDecodableAc1AsBehavioralMD is a DECODABLE obligation whose own
// id/for_kind are internally consistent (behavioral) — so
// artifact.DecodeObligation accepts it, path/id agreement being VL-011's job,
// not the decoder's — but which is filed at ac-1's STATIC convention path
// (.verdi/obligations/widget-story/ac-1--static.md). The coverage scan keys
// it under `behavioral` (its decoded for_kind), leaving `static` apparently
// uncovered, so the scaffold targets ac-1--static.md — the exact path this
// hand-authored file occupies (judged-coverage-predicate-forkind-keying).
const misnamedDecodableAc1AsBehavioralMD = `---
id: obligation/widget-story--ac-1--behavioral
kind: obligation
title: "HAND-AUTHORED-BEHAVIORAL-MISFILED-AT-STATIC-PATH"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/widget-story" }
frozen: { at: 2026-01-01, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# HAND-AUTHORED-BEHAVIORAL-MISFILED-AT-STATIC-PATH

A decodable, hand-authored obligation the scaffold must never clobber, even
though its filename names a different kind than its own for_kind.
`

// reverseGapStoryMD is widget-story declaring ONLY behavioral evidence on its
// single ac-1 — the judged-coverage-predicate-forkind-keying reverse scenario:
// with a decodable obligation misfiled at ac-1--static.md (an UNDECLARED kind's
// path) whose for_kind is behavioral, the scaffold must still create the
// DECLARED pair's own convention path ac-1--behavioral.md, never miscount the
// misfiled file as covering it. Lint-clean (mirrors obligationSeamStoryCleanMD).
const reverseGapStoryMD = `---
id: spec/widget-story
kind: spec
title: "Widget story"
owners: [platform-team]
class: story
status: draft
story: jira:LOAN-9001
problem: { text: "widgets are stale", anchor: problem }
outcome: { text: "widgets are current", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "behavioral evidence holds", evidence: [behavioral], anchor: ac-1 }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
---
# Widget story

## Problem

Widgets are stale.

## Outcome

Widgets are current.

## AC-1

Behavioral evidence holds.
`

// buildObligationSeamStoryRepo builds a one-layer fixturegit repo carrying
// obligationSeamStoryCleanMD, its implements-edge target (someFeatureMD,
// supersedepredecessor_test.go), and any extra files the caller supplies
// (obligation fixtures, ...).
func buildObligationSeamStoryRepo(t *testing.T, extra map[string]string) *fixturegit.Repo {
	t.Helper()
	files := map[string]string{
		".verdi/verdi.yaml":                        phase7ManifestYAML,
		".gitattributes":                           phase7GitAttributes,
		".verdi/specs/active/some-feature/spec.md": someFeatureMD,
		".verdi/specs/active/widget-story/spec.md": obligationSeamStoryCleanMD,
	}
	for k, v := range extra {
		files[k] = v
	}
	return fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "init store with widget-story draft"}})
}

func readObligation(t *testing.T, path string) (*artifact.ObligationFrontmatter, []byte) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	fm, body, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		t.Fatalf("splitting frontmatter of %s: %v", path, err)
	}
	ob, err := artifact.DecodeObligation(fm)
	if err != nil {
		t.Fatalf("decoding obligation %s: %v\n%s", path, err, raw)
	}
	return ob, body
}

func obligationPathFor(root, acID, kind string) string {
	return filepath.Join(root, ".verdi", "obligations", "widget-story", acID+"--"+kind+".md")
}

// decodeFixtureSpec strict-decodes md's frontmatter into a *SpecFrontmatter
// — a bare in-memory helper for scaffoldMissingObligations' own unit tests,
// which drive the function directly rather than through a CLI verb.
func decodeFixtureSpec(t *testing.T, md string) *artifact.SpecFrontmatter {
	t.Helper()
	fm, _, err := artifact.SplitFrontmatter([]byte(md))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	return spec
}

// TestScaffoldMissingObligations is scaffoldMissingObligations' own
// table-driven unit test (moved to obligation.go by Task 7's retirement of
// accept's freeze-moment backstop) — happy paths and negative paths per
// CLAUDE.md's testing rule, exercised directly (no CLI dispatch). Every
// case that actually writes needs a real git-backed root now (the function
// resolves its own frozen stamp off HEAD lazily); the no-op cases use a
// bare t.TempDir() since they never reach that git call.
func TestScaffoldMissingObligations(t *testing.T) {
	ctx := context.Background()

	t.Run("feature class is a no-op", func(t *testing.T) {
		root := t.TempDir()
		spec := decodeFixtureSpec(t, someFeatureMD)
		created, err := scaffoldMissingObligations(ctx, root, "some-feature", spec, "op")
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if created != nil {
			t.Errorf("created = %v, want nil (dc-3: features never carry obligations)", created)
		}
	})

	t.Run("story with everything already covered is a no-op", func(t *testing.T) {
		root := t.TempDir()
		writeTestFile(t, obligationPathFor(root, "ac-1", "static"), []byte(preExistingAc1StaticMD))
		spec := decodeFixtureSpec(t, `---
id: spec/widget-story
kind: spec
class: story
title: "t"
owners: [platform-team]
status: draft
story: jira:LOAN-1
problem: { text: "p", anchor: problem }
outcome: { text: "o", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "x", evidence: [static] }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
---
# t
`)
		created, err := scaffoldMissingObligations(ctx, root, "widget-story", spec, "op")
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if created != nil {
			t.Errorf("created = %v, want nil (already covered)", created)
		}
	})

	t.Run("scaffolds only the missing pair, multiple kinds on one AC", func(t *testing.T) {
		repo := buildObligationSeamStoryRepo(t, nil)
		spec := decodeFixtureSpec(t, `---
id: spec/widget-story
kind: spec
class: story
title: "t"
owners: [platform-team]
status: draft
story: jira:LOAN-1
problem: { text: "p", anchor: problem }
outcome: { text: "o", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "x", evidence: [static, behavioral] }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
---
# t
`)
		created, err := scaffoldMissingObligations(ctx, repo.Dir, "widget-story", spec, "op")
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		wantPaths := []string{
			obligationPathFor(repo.Dir, "ac-1", "static"),
			obligationPathFor(repo.Dir, "ac-1", "behavioral"),
		}
		if len(created) != len(wantPaths) {
			t.Fatalf("created = %v, want %v", created, wantPaths)
		}
		for _, p := range wantPaths {
			if _, err := os.Stat(p); err != nil {
				t.Errorf("expected scaffold at %s: %v", p, err)
			}
		}
	})

	t.Run("malformed existing file at the convention path errors, reporting whatever was created first", func(t *testing.T) {
		repo := buildObligationSeamStoryRepo(t, map[string]string{
			".verdi/obligations/widget-story/ac-2--behavioral.md": malformedAc2BehavioralMD,
		})
		spec := decodeFixtureSpec(t, `---
id: spec/widget-story
kind: spec
class: story
title: "t"
owners: [platform-team]
status: draft
story: jira:LOAN-1
problem: { text: "p", anchor: problem }
outcome: { text: "o", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "x", evidence: [static] }
  - { id: ac-2, text: "y", evidence: [behavioral] }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
---
# t
`)
		created, err := scaffoldMissingObligations(ctx, repo.Dir, "widget-story", spec, "op")
		if err == nil {
			t.Fatal("err = nil, want a decode-failure error")
		}
		if len(created) != 1 || created[0] != obligationPathFor(repo.Dir, "ac-1", "static") {
			t.Errorf("created = %v, want exactly [%s] (ac-1 scaffolded before the ac-2 failure)", created, obligationPathFor(repo.Dir, "ac-1", "static"))
		}
	})

	t.Run("misfiled decodable obligation at a declared pair's own path refuses, never clobbers", func(t *testing.T) {
		repo := buildObligationSeamStoryRepo(t, map[string]string{
			".verdi/obligations/widget-story/ac-1--static.md": misnamedDecodableAc1AsBehavioralMD,
		})
		spec := decodeFixtureSpec(t, obligationSeamStoryCleanMD)
		created, err := scaffoldMissingObligations(ctx, repo.Dir, "widget-story", spec, "op")
		if err == nil {
			t.Fatal("err = nil, want a refusal naming the occupied path")
		}
		if !contains(err.Error(), "ac-1--static.md") {
			t.Errorf("err = %v, want it to name the occupied convention path", err)
		}
		if len(created) != 0 {
			t.Errorf("created = %v, want none (ac-1 is the first pair processed)", created)
		}
		got, rerr := os.ReadFile(obligationPathFor(repo.Dir, "ac-1", "static"))
		if rerr != nil {
			t.Fatal(rerr)
		}
		if string(got) != misnamedDecodableAc1AsBehavioralMD {
			t.Fatalf("the misfiled hand-authored obligation was clobbered:\n--- got ---\n%s\n--- want (byte-identical) ---\n%s", got, misnamedDecodableAc1AsBehavioralMD)
		}
	})

	t.Run("misfiled at an undeclared kind's path still scaffolds the declared pair's own convention path", func(t *testing.T) {
		repo := buildObligationSeamStoryRepo(t, map[string]string{
			".verdi/specs/active/widget-story/spec.md":        reverseGapStoryMD,
			".verdi/obligations/widget-story/ac-1--static.md": misnamedDecodableAc1AsBehavioralMD,
		})
		spec := decodeFixtureSpec(t, reverseGapStoryMD)
		created, err := scaffoldMissingObligations(ctx, repo.Dir, "widget-story", spec, "op")
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		behavioralPath := obligationPathFor(repo.Dir, "ac-1", "behavioral")
		if len(created) != 1 || created[0] != behavioralPath {
			t.Fatalf("created = %v, want exactly [%s] (the declared pair's own convention path)", created, behavioralPath)
		}
		misfiled, rerr := os.ReadFile(obligationPathFor(repo.Dir, "ac-1", "static"))
		if rerr != nil {
			t.Fatal(rerr)
		}
		if string(misfiled) != misnamedDecodableAc1AsBehavioralMD {
			t.Fatalf("the misfiled obligation was modified:\n--- got ---\n%s\n--- want (byte-identical) ---\n%s", misfiled, misnamedDecodableAc1AsBehavioralMD)
		}
	})
}

// TestOperatorOwner proves the $USER/fallback-sentinel contract (O-6).
func TestOperatorOwner(t *testing.T) {
	t.Run("USER set", func(t *testing.T) {
		t.Setenv("USER", "alice")
		if got := operatorOwner(); got != "alice" {
			t.Errorf("operatorOwner() = %q, want alice", got)
		}
	})
	t.Run("USER unset falls back to the disclosed sentinel", func(t *testing.T) {
		t.Setenv("USER", "")
		if got := operatorOwner(); got != fallbackOperatorOwner {
			t.Errorf("operatorOwner() = %q, want the sentinel %q", got, fallbackOperatorOwner)
		}
	})
}

// TestBackstopObligationBody proves the rendered body always carries the
// disclosure line verbatim plus the acceptance criterion's own declared
// text — never a fabricated claim about what the evidence specifically
// shows.
func TestBackstopObligationBody(t *testing.T) {
	got := backstopObligationBody("spec/widget-story", "ac-1", artifact.EvidenceBehavioral, "the retry proves end to end")
	if !contains(got, evidence.UnauthoredObligationMarker) {
		t.Errorf("body missing the shared unauthored marker:\n%s", got)
	}
	if !contains(got, obligationBackstopDisclosureLine()) {
		t.Errorf("body missing the disclosure line verbatim:\n%s", got)
	}
	if !contains(got, "ac-1") || !contains(got, "behavioral") {
		t.Errorf("body does not name the (ac, kind) pair:\n%s", got)
	}
	if !contains(got, "the retry proves end to end") {
		t.Errorf("body does not carry the acceptance criterion's own declared text:\n%s", got)
	}
}
