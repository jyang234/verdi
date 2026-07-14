package mcpserve

import (
	"context"
	"errors"
	"testing"

	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/forge/fake"
)

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

// newBackendSupersessionLoaderForTest mirrors get_board's own review-mode
// wiring pattern (backendCommentFeed): a resolvable default branch via
// CI_DEFAULT_BRANCH, so no git is touched — hermetic (CLAUDE.md: no
// network in any test).
func newBackendSupersessionLoaderForTest(t *testing.T, f forge.Forge) backendSupersessionLoader {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	return backendSupersessionLoader{f: f, root: t.TempDir()}
}

// TestBackendSupersessionLoader_LoadsConfirmedCandidates proves get_board's
// own adapter delegates to evidence.LoadPendingSupersessionCandidates
// (co-3's exact entry point) exactly like cmd/verdi's forgeSupersessionLoader.
func TestBackendSupersessionLoader_LoadsConfirmedCandidates(t *testing.T) {
	f := fake.New()
	f.SeedOpenMR("main", forge.OpenMR{ID: "7", SourceBranch: "design/loan-workflow-v2"})
	f.SeedFile("design/loan-workflow-v2", supersessionFeedCandidatePath, []byte(supersessionFeedCandidateSpecMD))

	loader := newBackendSupersessionLoaderForTest(t, f)
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

// TestBackendSupersessionLoader_NoDefaultBranch proves an unresolvable
// default branch yields ok=false, never an error.
func TestBackendSupersessionLoader_NoDefaultBranch(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "")
	loader := backendSupersessionLoader{f: fake.New(), root: t.TempDir()}
	candidates, ok, err := loader.LoadCandidates(context.Background(), "spec/loan-workflow", supersessionFeedCandidatePath)
	if err != nil {
		t.Fatalf("LoadCandidates: %v", err)
	}
	if ok || candidates != nil {
		t.Errorf("ok=%v candidates=%v, want false,nil", ok, candidates)
	}
}

// erroringSupersessionForge fails ListOpenMRs with a genuine transport
// error — the operational-negative path.
type erroringSupersessionForge struct{ *fake.Forge }

func (erroringSupersessionForge) ListOpenMRs(ctx context.Context, targetBranch string) ([]forge.OpenMR, error) {
	return nil, errors.New("forge: simulated transport failure")
}

// TestBackendSupersessionLoader_TransportErrorPropagates proves a genuine
// forge failure surfaces as an error, never silently as ok=false.
func TestBackendSupersessionLoader_TransportErrorPropagates(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	loader := backendSupersessionLoader{f: erroringSupersessionForge{fake.New()}, root: t.TempDir()}
	_, _, err := loader.LoadCandidates(context.Background(), "spec/loan-workflow", supersessionFeedCandidatePath)
	if err == nil {
		t.Fatal("got nil error, want the transport failure to propagate")
	}
}
