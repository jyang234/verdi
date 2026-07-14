package main

import (
	"context"
	"errors"
	"testing"

	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/forge/fake"
)

// newSupersessionLoaderForTest mirrors reviewfeed_test.go's
// newFeedForTest: a resolvable default branch via CI_DEFAULT_BRANCH, so
// no git is touched — hermetic (CLAUDE.md: no network in any test).
func newSupersessionLoaderForTest(t *testing.T, f forge.Forge) *forgeSupersessionLoader {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	return newForgeSupersessionLoader(f, t.TempDir())
}

const supersessionFeedCandidateSpecMD = `---
id: spec/loan-workflow-v2
kind: spec
class: feature
title: "Loan workflow v2 (pending, unmerged)"
status: draft
owners: [platform-team]
links:
  - { type: supersedes, ref: spec/loan-workflow }
acceptance_criteria:
  - { id: ac-1, text: "tightened outcome", evidence: [runtime, attestation] }
supersession:
  carried: []
  amended: [ { id: ac-1, note: "tightened threshold" } ]
  amended_advisory: []
  removed: []
  added: []
---
# Loan workflow v2 (pending)
`

const supersessionFeedCandidatePath = ".verdi/specs/active/loan-workflow-v2/spec.md"

// TestForgeSupersessionLoader_LoadsConfirmedCandidates proves the adapter
// resolves the default branch and delegates to
// evidence.LoadPendingSupersessionCandidates (co-3's exact entry point),
// returning ok=true with the confirmed candidate set.
func TestForgeSupersessionLoader_LoadsConfirmedCandidates(t *testing.T) {
	f := fake.New()
	f.SeedOpenMR("main", forge.OpenMR{ID: "7", SourceBranch: "design/loan-workflow-v2"})
	f.SeedFile("design/loan-workflow-v2", supersessionFeedCandidatePath, []byte(supersessionFeedCandidateSpecMD))

	loader := newSupersessionLoaderForTest(t, f)
	candidates, ok, err := loader.LoadCandidates(context.Background(), "spec/loan-workflow", supersessionFeedCandidatePath)
	if err != nil {
		t.Fatalf("LoadCandidates: %v", err)
	}
	if !ok {
		t.Fatal("ok = false, want true (default branch resolved)")
	}
	if len(candidates) != 1 || candidates[0].MRID != "7" {
		t.Fatalf("candidates = %+v, want exactly one from MR 7", candidates)
	}
	if candidates[0].Digest == "" {
		t.Error("candidate Digest is empty, want a content digest of the fetched bytes")
	}
}

// TestForgeSupersessionLoader_NoDefaultBranch proves an unresolvable
// default branch yields ok=false, never an error — the disclosed-
// unproven case (badge-computes ac-3), mirroring forgeCommentFeed's own
// "nothing to mirror, never an error" posture for the identical condition.
func TestForgeSupersessionLoader_NoDefaultBranch(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "")
	loader := newForgeSupersessionLoader(fake.New(), t.TempDir())
	candidates, ok, err := loader.LoadCandidates(context.Background(), "spec/loan-workflow", supersessionFeedCandidatePath)
	if err != nil {
		t.Fatalf("LoadCandidates: %v", err)
	}
	if ok || candidates != nil {
		t.Errorf("ok=%v candidates=%v, want false,nil", ok, candidates)
	}
}

// erroringSupersessionForge fails ListOpenMRs with a genuine transport
// error — the operational-negative path, distinct from "no default
// branch"/"no open MRs".
type erroringSupersessionForge struct{ *fake.Forge }

func (erroringSupersessionForge) ListOpenMRs(ctx context.Context, targetBranch string) ([]forge.OpenMR, error) {
	return nil, errors.New("forge: simulated transport failure")
}

// TestForgeSupersessionLoader_TransportErrorPropagates proves a genuine
// forge failure surfaces as an error, never silently as ok=false.
func TestForgeSupersessionLoader_TransportErrorPropagates(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	loader := newForgeSupersessionLoader(erroringSupersessionForge{fake.New()}, t.TempDir())
	_, _, err := loader.LoadCandidates(context.Background(), "spec/loan-workflow", supersessionFeedCandidatePath)
	if err == nil {
		t.Fatal("got nil error, want the transport failure to propagate")
	}
}
