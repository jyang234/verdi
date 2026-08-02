// Fixtures and tests for predecessor supersession under the merge-signaled
// design (docs/superpowers/specs/2026-08-01-merge-signals-spec-acceptance-
// design.md). Task 7 deletes cmd/verdi/supersede.go — the legacy
// accept-time predecessor status-flip mutation (D-12, extended by round 6's
// ac-1 feature-supersession-state) — since supersession is now derived
// entirely from Git reachability: a validly landed successor (a `links:
// {type: supersedes}` edge PLUS a validated `supersession:` block, both on
// the default branch) makes internal/specstate.Projector.Resolve report the
// predecessor Superseded, without ever touching the predecessor's own
// bytes. I-40 (open owner question, invention ledger): story-class specs
// cannot carry a `supersession:` block (feature-only field — internal/
// artifact's validateStory rejects it outright), so Git-derived
// supersession proof exists only for feature-class predecessor/successor
// pairs; deleting supersede.go removes the only writer of a story-class
// legacy `status: superseded` flip, and this file invents no replacement
// mechanism for it (disclosed, not silently worked around).
//
// The fixture bodies below (predecessor/successor spec.md content) are
// kept unchanged from the pre-Task-7 shape and reused across this package
// (accept_test.go's non-mutation characterization, vocabulary_cli_test.go's
// vocab-rename CLI proofs) — only the BEHAVIORAL tests that used to drive
// them through the now-deleted flip functions are rewritten here.
package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/specstate"
)

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
// §Decision-conflict gate's rung-2 machinery) — and no `supersession:` block
// at all, so it must never derive a predecessor flip under either the old
// mechanism or the new specstate two-signal proof (fm.Supersession == nil is
// scanSuccessors' own first, unconditional exclusion).
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

// succFeatureWithValidSupersessionMD is the two-signal shape
// internal/specstate/resolve.go's scanSuccessors actually requires to
// derive a predecessor's Superseded state: a whole-spec `links: {type:
// supersedes}` edge PLUS a validated `supersession:` block (feature-only,
// I-40) — the shape a real, reviewed successor pull request carries.
const succFeatureWithValidSupersessionMD = `---
id: spec/succ-feature
kind: spec
title: "Successor feature"
owners: [platform-team]
class: feature
story: jira:LOAN-3002
acceptance_criteria:
  - { id: ac-1, text: "v2 obligation, corrected", evidence: [static] }
links:
  - { type: supersedes, ref: "spec/pred-feature" }
supersession:
  amended:
    - { id: ac-1, note: "AC text corrected" }
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

// predStoryLegacySupersededMD is predStoryAcceptedMD's compatibility
// twin: a legacy, EXPLICIT `status: superseded` story predecessor — the
// shape the OLD accept ritual's story-rung flip (D-12, before this task)
// left behind, and the only shape a story predecessor's Superseded state
// can ever carry now that supersede.go's writer is gone (I-40: story-class
// specs cannot carry a `supersession:` block, so Git-derived two-signal
// proof can never independently confirm story-level supersession).
// internal/specstate's own compatibility fallback (probeLegacyStatus)
// reads this explicit field directly once these exact bytes are proven
// landed — preserved, not reinvented, by the test below.
const predStoryLegacySupersededMD = `---
id: spec/pred-story
kind: spec
title: "Predecessor story"
owners: [platform-team]
class: story
status: superseded
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
  - { id: ac-1, text: "v2 obligation, corrected", evidence: [static], anchor: ac-1 }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
  - { type: supersedes, ref: "spec/pred-story" }
---
# Successor story

## Problem

Borrowers see stale data.

## Outcome

Borrowers see current data.

## AC-1

v2 obligation, corrected.
`

// someFeatureMD is the implements-edge target both predStoryAcceptedMD and
// succStorySupersedesMD name (spec/some-feature#ac-1) — a plain v0-shaped
// feature spec (no problem/outcome/stubs) whose only job is to exist and
// declare ac-1, so that link resolves (VL-003) rather than dangling.
const someFeatureMD = `---
id: spec/some-feature
kind: spec
title: "Some feature"
owners: [platform-team]
class: feature
status: draft
acceptance_criteria:
  - { id: ac-1, text: "some feature ac", evidence: [static] }
---
# Some feature
`

// buildPredecessorFlipRepo builds a minimal one-layer fixturegit repo
// carrying exactly the two named spec.md bodies under specs/active/, plus
// the shared someFeatureMD implements-edge target — hand-written
// frontmatter, no design-start scaffold.
func buildPredecessorFlipRepo(t *testing.T, predName, predMD, succName, succMD string) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                            phase7ManifestYAML,
				".verdi/specs/active/some-feature/spec.md":     someFeatureMD,
				".verdi/specs/active/" + predName + "/spec.md": predMD,
				".verdi/specs/active/" + succName + "/spec.md": succMD,
			},
			Message: "init store with predecessor + draft successor",
		},
	})
}

// resolveCandidate is a small helper wrapping specstate.NewProjector().Resolve
// for a spec at the given active-zone name, reading its current on-disk
// bytes as the candidate content — the CLI's own read-then-resolve pattern
// (buildstart.go's runBuildStart, obligation.go's runObligationScaffold).
func resolveCandidate(t *testing.T, ctx context.Context, root, name string) specstate.Result {
	t.Helper()
	relPath := ".verdi/specs/active/" + name + "/spec.md"
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relPath)))
	if err != nil {
		t.Fatalf("reading %s: %v", relPath, err)
	}
	result, err := specstate.NewProjector().Resolve(ctx, root, specstate.Candidate{Path: relPath, Content: content})
	if err != nil {
		t.Fatalf("Resolve(%s): %v", name, err)
	}
	return result
}

// TestDerivedSupersession_FeaturePredecessor is Step 5's core proof: a
// successor landed on the default branch, carrying both a whole-spec
// `links: {type: supersedes}` edge and a validated `supersession:` block,
// derives the predecessor's Superseded state through
// internal/specstate.Projector.Resolve — without the predecessor's own
// bytes ever changing. Mirrors D-12/ac-1's old accept-time flip's OUTCOME
// (predecessor reads as superseded once its successor is accepted) while
// removing the mutation that used to produce it.
func TestDerivedSupersession_FeaturePredecessor(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                        phase7ManifestYAML,
				".verdi/specs/active/pred-feature/spec.md": predFeatureAcceptedMD,
				".verdi/specs/active/succ-feature/spec.md": succFeatureWithValidSupersessionMD,
			},
			Message: "predecessor + validly superseding successor, both landed on the default branch",
		},
	})
	ctx := context.Background()

	predPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "pred-feature", "spec.md")
	before, err := os.ReadFile(predPath)
	if err != nil {
		t.Fatal(err)
	}

	result := resolveCandidate(t, ctx, repo.Dir, "pred-feature")
	if result.State != specstate.Superseded {
		t.Fatalf("Resolve(predecessor) = %+v, want Superseded", result)
	}

	after, err := os.ReadFile(predPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("predecessor bytes changed merely by resolving the successor's state — supersession must be derived, never written:\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}

	// The successor's own bytes are equally untouched — deriving state is
	// read-only for both sides of the pair.
	succResult := resolveCandidate(t, ctx, repo.Dir, "succ-feature")
	if succResult.State != specstate.AcceptedPendingBuild {
		t.Fatalf("Resolve(successor) = %+v, want AcceptedPendingBuild", succResult)
	}
}

// TestDerivedSupersession_ObjectFragmentEdgeDoesNotSupersede is the
// negative complement: a successor whose supersedes edge is an
// object-fragment (a decision-level override, never a chain edge) and
// which carries no `supersession:` block at all must NOT derive the
// predecessor's Superseded state — it stays AcceptedPendingBuild.
func TestDerivedSupersession_ObjectFragmentEdgeDoesNotSupersede(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                        phase7ManifestYAML,
				".verdi/specs/active/pred-feature/spec.md": predFeatureAcceptedMD,
				".verdi/specs/active/succ-feature/spec.md": succFeatureFragmentSupersedesMD,
			},
			Message: "predecessor + a successor carrying only a decision-level fragment edge",
		},
	})
	ctx := context.Background()

	result := resolveCandidate(t, ctx, repo.Dir, "pred-feature")
	if result.State != specstate.AcceptedPendingBuild {
		t.Fatalf("Resolve(predecessor) = %+v, want AcceptedPendingBuild (a fragment edge with no supersession: block must never derive supersession)", result)
	}
}

// TestDerivedSupersession_LegacySupersededStoryPreserved is Step 5's
// compatibility guard: a story predecessor that already carries the legacy,
// explicit `status: superseded` field (the shape supersede.go's now-deleted
// story-rung flip used to leave behind) still projects Superseded once its
// exact bytes are proven landed — internal/specstate's own compatibility
// fallback (resolve.go's probeLegacyStatus branch), preserved rather than
// reinvented: I-40 forbids inventing any NEW story-supersession mechanism,
// since a story spec can never carry a `supersession:` block.
func TestDerivedSupersession_LegacySupersededStoryPreserved(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                        phase7ManifestYAML,
				".verdi/specs/active/some-feature/spec.md": someFeatureMD,
				".verdi/specs/active/pred-story/spec.md":   predStoryLegacySupersededMD,
			},
			Message: "legacy-superseded story predecessor, landed on the default branch",
		},
	})
	ctx := context.Background()

	result := resolveCandidate(t, ctx, repo.Dir, "pred-story")
	if result.State != specstate.Superseded {
		t.Fatalf("Resolve(legacy-superseded story) = %+v, want Superseded (compatibility reading)", result)
	}
	if len(result.Disclosures) == 0 {
		t.Fatal("want a compatibility disclosure naming the legacy field as the deciding signal")
	}
}
