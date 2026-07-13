// Tests for supersedePredecessors/flipPredecessorToSuperseded (accept.go):
// the D-12 rung-3 story-predecessor flip, and its round-6 ac-1
// (feature-supersession-state) extension to feature-class predecessors via
// a WHOLE-SPEC supersedes edge. Kept in its own file per this package's
// one-file-per-topic convention (accept_test.go covers the base accept
// ritual; this file covers the predecessor-flip mechanism specifically).
package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// Fixture spec bodies, planted directly (no design-start scaffold needed —
// mirrors accept_test.go's own buildAcceptNegativeRepo/alreadyAcceptedSpecMD
// minimal style), since these tests drive the flip mechanism directly
// through runAccept rather than the scaffold ritual.

const predFeatureAcceptedMD = `---
id: spec/pred-feature
kind: spec
title: "Predecessor feature"
owners: [platform-team]
class: feature
status: accepted-pending-build
story: jira:LOAN-3001
acceptance_criteria:
  - { id: ac-1, text: "v1 obligation", evidence: [static] }
frozen: { at: 2026-01-01, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# Predecessor feature
`

// predFeatureClosedMD is deliberately left under specs/active/ (rather than
// moved to specs/archive/, which VL-002 would otherwise require of a real
// closed feature/story spec) so this fixture directly exercises
// flipPredecessorToSuperseded's own status guard — dc-2's
// closed-is-not-flipped rule — rather than the separate "predecessor absent
// from active/" no-op path a fully VL-002-compliant repo would hit instead
// (disclosed judgment call: the guard itself is what dc-2 is about).
const predFeatureClosedMD = `---
id: spec/pred-feature
kind: spec
title: "Predecessor feature"
owners: [platform-team]
class: feature
status: closed
story: jira:LOAN-3001
acceptance_criteria:
  - { id: ac-1, text: "v1 obligation", evidence: [static] }
frozen: { at: 2026-01-01, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# Predecessor feature
`

const predFeatureSupersededMD = `---
id: spec/pred-feature
kind: spec
title: "Predecessor feature"
owners: [platform-team]
class: feature
status: superseded
story: jira:LOAN-3001
acceptance_criteria:
  - { id: ac-1, text: "v1 obligation", evidence: [static] }
frozen: { at: 2026-01-01, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# Predecessor feature
`

const succFeatureWholeSpecSupersedesMD = `---
id: spec/succ-feature
kind: spec
title: "Successor feature"
owners: [platform-team]
class: feature
status: draft
story: jira:LOAN-3002
acceptance_criteria:
  - { id: ac-1, text: "v2 obligation, corrected", evidence: [static] }
links:
  - { type: supersedes, ref: "spec/pred-feature" }
---
# Successor feature
`

// succFeatureFragmentSupersedesMD carries an OBJECT-FRAGMENT supersedes edge
// (#ac-1) rather than a whole-spec one — a decision-level override shape (03
// §Decision-conflict gate's rung-2 machinery), never the rung-3/feature
// chain edge wholeSpecSupersedesTarget identifies — so it must NOT trigger a
// feature-predecessor flip.
const succFeatureFragmentSupersedesMD = `---
id: spec/succ-feature
kind: spec
title: "Successor feature"
owners: [platform-team]
class: feature
status: draft
story: jira:LOAN-3002
acceptance_criteria:
  - { id: ac-1, text: "v2 obligation, corrected", evidence: [static] }
links:
  - { type: supersedes, ref: "spec/pred-feature#ac-1" }
---
# Successor feature
`

const predStoryAcceptedMD = `---
id: spec/pred-story
kind: spec
title: "Predecessor story"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:LOAN-4001
problem: { text: "borrowers see stale data", anchor: problem }
outcome: { text: "borrowers see current data", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "v1 obligation", evidence: [static] }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
frozen: { at: 2026-01-01, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# Predecessor story
`

// succStorySupersedesMD is the rung-3 story-to-story chain edge D-12
// shipped: a whole-spec supersedes edge to a STORY-class predecessor.
const succStorySupersedesMD = `---
id: spec/succ-story
kind: spec
title: "Successor story"
owners: [platform-team]
class: story
status: draft
story: jira:LOAN-4001
problem: { text: "borrowers see stale data", anchor: problem }
outcome: { text: "borrowers see current data", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "v2 obligation, corrected", evidence: [static] }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
  - { type: supersedes, ref: "spec/pred-story" }
---
# Successor story
`

// buildPredecessorFlipRepo builds a minimal one-layer fixturegit repo
// carrying exactly the two named spec.md bodies under specs/active/ — no
// design-start scaffold, since these tests drive runAccept directly against
// hand-written frontmatter (mirroring accept_test.go's own
// buildAcceptNegativeRepo).
func buildPredecessorFlipRepo(t *testing.T, predName, predMD, succName, succMD string) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                            phase7ManifestYAML,
				".verdi/specs/active/" + predName + "/spec.md": predMD,
				".verdi/specs/active/" + succName + "/spec.md": succMD,
			},
			Message: "init store with predecessor + draft successor",
		},
	})
}

// TestFlipPredecessorToSuperseded_FeatureHappyPath is ac-1's core proof:
// accepting a feature v2 that carries a WHOLE-SPEC supersedes edge to a
// feature predecessor (accepted-pending-build, frozen) flips that
// predecessor's status to superseded, in the SAME acceptance commit, staying
// in specs/active/ with its frozen stamp and every other byte unchanged.
func TestFlipPredecessorToSuperseded_FeatureHappyPath(t *testing.T) {
	repo := buildPredecessorFlipRepo(t, "pred-feature", predFeatureAcceptedMD, "succ-feature", succFeatureWholeSpecSupersedesMD)
	ctx := context.Background()

	beforeHead, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	_, predRawBefore := readSpec(t, repo.Dir, "pred-feature")

	var stdout, stderr bytes.Buffer
	if got := runAccept(ctx, repo.Dir, "spec/succ-feature", &stdout, &stderr); got != 0 {
		t.Fatalf("runAccept(succ-feature) = %d, want 0; stderr=%s", got, stderr.String())
	}

	predAfter, predRawAfter := readSpec(t, repo.Dir, "pred-feature")
	if predAfter.Status != "superseded" {
		t.Fatalf("predecessor status = %q, want superseded", predAfter.Status)
	}
	if predAfter.Frozen == nil || predAfter.Frozen.Commit != "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef" || predAfter.Frozen.At != "2026-01-01" {
		t.Fatalf("predecessor frozen stamp changed across the flip: %+v", predAfter.Frozen)
	}
	wantRaw := bytes.Replace(predRawBefore, []byte("status: accepted-pending-build"), []byte("status: superseded"), 1)
	if bytes.Equal(wantRaw, predRawBefore) {
		t.Fatal("test setup: predecessor fixture did not carry the expected status line to flip")
	}
	if !bytes.Equal(predRawAfter, wantRaw) {
		t.Fatalf("predecessor content diverged beyond the single status line:\n--- got ---\n%s\n--- want ---\n%s", predRawAfter, wantRaw)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "active", "pred-feature", "spec.md")); err != nil {
		t.Fatalf("predecessor must remain at specs/active/: %v", err)
	}
	if !contains(stdout.String(), "superseded by spec/succ-feature") {
		t.Fatalf("stdout = %q, want a disclosed predecessor-superseded line", stdout.String())
	}

	// Same acceptance commit: exactly one new commit was created (its sole
	// parent is the pre-accept HEAD), and its diff touches both spec.md
	// files — the successor's own flip and the predecessor's flip land
	// together.
	afterHead, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if afterHead == beforeHead {
		t.Fatal("accept did not create a new commit")
	}
	parent, err := gitx.RevParse(ctx, repo.Dir, "HEAD^")
	if err != nil {
		t.Fatalf("RevParse(HEAD^): %v", err)
	}
	if parent != beforeHead {
		t.Fatalf("accept created more than one commit: HEAD^ = %q, want the pre-accept HEAD %q", parent, beforeHead)
	}
	entries, err := gitx.DiffNameStatus(ctx, repo.Dir, beforeHead, afterHead)
	if err != nil {
		t.Fatal(err)
	}
	var touchedPred, touchedSucc bool
	for _, e := range entries {
		switch e.Path {
		case ".verdi/specs/active/pred-feature/spec.md":
			touchedPred = true
		case ".verdi/specs/active/succ-feature/spec.md":
			touchedSucc = true
		}
	}
	if !touchedPred || !touchedSucc {
		t.Fatalf("the single acceptance commit must touch both spec.md files: entries=%+v", entries)
	}
}

// TestFlipPredecessorToSuperseded_Negative table-drives the guard cases:
// dc-2's deliberately deferred closed predecessor, idempotence on an
// already-superseded predecessor, and an object-fragment supersedes edge
// (never a whole-spec chain edge) not triggering a feature flip at all —
// each leaves the predecessor's status exactly where it started.
func TestFlipPredecessorToSuperseded_Negative(t *testing.T) {
	cases := []struct {
		name       string
		predMD     string
		succMD     string
		wantStatus string
	}{
		{"closed feature predecessor is not flipped (dc-2)", predFeatureClosedMD, succFeatureWholeSpecSupersedesMD, "closed"},
		{"already-superseded feature predecessor is idempotent", predFeatureSupersededMD, succFeatureWholeSpecSupersedesMD, "superseded"},
		{"object-fragment supersedes does not trigger a feature flip", predFeatureAcceptedMD, succFeatureFragmentSupersedesMD, "accepted-pending-build"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := buildPredecessorFlipRepo(t, "pred-feature", tc.predMD, "succ-feature", tc.succMD)
			ctx := context.Background()

			var stdout, stderr bytes.Buffer
			if got := runAccept(ctx, repo.Dir, "spec/succ-feature", &stdout, &stderr); got != 0 {
				t.Fatalf("runAccept(succ-feature) = %d, want 0; stderr=%s", got, stderr.String())
			}
			predAfter, _ := readSpec(t, repo.Dir, "pred-feature")
			if string(predAfter.Status) != tc.wantStatus {
				t.Fatalf("predecessor status = %q, want %q", predAfter.Status, tc.wantStatus)
			}
		})
	}
}

// TestFlipPredecessorToSuperseded_StoryRegression proves the rung-3
// story-predecessor flip D-12 shipped (supersedePredecessors' original
// behavior, exercised here through the shared flipPredecessorToSuperseded
// helper the ac-1 refactor extracted) is unchanged by ac-1's feature-rung
// extension: accepting a story v2 that supersedes a story v1 still flips v1
// to superseded exactly as before.
func TestFlipPredecessorToSuperseded_StoryRegression(t *testing.T) {
	repo := buildPredecessorFlipRepo(t, "pred-story", predStoryAcceptedMD, "succ-story", succStorySupersedesMD)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	if got := runAccept(ctx, repo.Dir, "spec/succ-story", &stdout, &stderr); got != 0 {
		t.Fatalf("runAccept(succ-story) = %d, want 0; stderr=%s", got, stderr.String())
	}
	predAfter, _ := readSpec(t, repo.Dir, "pred-story")
	if predAfter.Status != "superseded" {
		t.Fatalf("predecessor story status = %q, want superseded (must not regress D-12's story flip)", predAfter.Status)
	}
	if !contains(stdout.String(), "superseded by spec/succ-story") {
		t.Fatalf("stdout = %q, want a disclosed predecessor-superseded line", stdout.String())
	}
}
