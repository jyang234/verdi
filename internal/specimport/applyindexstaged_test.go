package specimport

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
)

// TestPreviewAndApply_IndexStagedDirty_Refused proves the cleanliness gate
// catches a change staged in the index (git add, never committed) — not
// only an unstaged working-tree edit or an untracked file, which existing
// tests already cover (spec-import-contract.md Task 3 handoff: "Include
// clean/stale/dirty/index-staged contexts"). Both Preview and a fresh Apply
// must refuse with ErrDirtyContext, and snapshotRepository/
// assertRepositoryEqual must prove neither call moved a ref, touched the
// index, or wrote to the worktree — no design/<slug> branch is created
// either.
func TestPreviewAndApply_IndexStagedDirty_Refused(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := evidencedRequest()
	ctx := context.Background()

	// Obtain a valid preview digest while the checkout is still clean, so
	// Apply's own dirty-context refusal is never confused with a stale-
	// digest refusal.
	cleanPreview, err := svc.Preview(ctx, repo.Dir, req)
	if err != nil {
		t.Fatalf("Preview (clean): %v", err)
	}

	// Stage a new file in the index without committing it.
	if err := os.WriteFile(joinPath(repo.Dir, "staged.txt"), []byte("staged, not committed"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitFixture(t, repo.Dir, "add", "staged.txt")

	before := snapshotRepository(t, repo.Dir)

	if _, err := svc.Preview(ctx, repo.Dir, req); !errors.Is(err, ErrDirtyContext) {
		t.Fatalf("Preview(index-staged) = %v, want ErrDirtyContext", err)
	}
	assertRepositoryEqual(t, before, snapshotRepository(t, repo.Dir))

	if _, err := svc.Apply(ctx, repo.Dir, req, cleanPreview.Digest, testAgent(t)); !errors.Is(err, ErrDirtyContext) {
		t.Fatalf("Apply(index-staged, fresh) = %v, want ErrDirtyContext", err)
	}
	assertRepositoryEqual(t, before, snapshotRepository(t, repo.Dir))

	if exists, _ := gitx.HasLocalBranch(ctx, repo.Dir, "design/sample-feature"); exists {
		t.Fatal("a branch was created despite the index-staged dirty refusal")
	}
}

// TestPreviewAndApply_IndexStagedModification_Refused proves the same gate
// for a STAGED MODIFICATION of an already-tracked file (as opposed to a
// staged new file above) — `git add` after editing a tracked path, with no
// commit.
func TestPreviewAndApply_IndexStagedModification_Refused(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := evidencedRequest()
	ctx := context.Background()

	cleanPreview, err := svc.Preview(ctx, repo.Dir, req)
	if err != nil {
		t.Fatalf("Preview (clean): %v", err)
	}

	if err := os.WriteFile(joinPath(repo.Dir, ".verdi", "verdi.yaml"), []byte("schema: verdi.layout/v1\nextra: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitFixture(t, repo.Dir, "add", ".verdi/verdi.yaml")

	before := snapshotRepository(t, repo.Dir)

	if _, err := svc.Preview(ctx, repo.Dir, req); !errors.Is(err, ErrDirtyContext) {
		t.Fatalf("Preview(index-staged modification) = %v, want ErrDirtyContext", err)
	}
	assertRepositoryEqual(t, before, snapshotRepository(t, repo.Dir))

	if _, err := svc.Apply(ctx, repo.Dir, req, cleanPreview.Digest, testAgent(t)); !errors.Is(err, ErrDirtyContext) {
		t.Fatalf("Apply(index-staged modification, fresh) = %v, want ErrDirtyContext", err)
	}
	assertRepositoryEqual(t, before, snapshotRepository(t, repo.Dir))

	if exists, _ := gitx.HasLocalBranch(ctx, repo.Dir, "design/sample-feature"); exists {
		t.Fatal("a branch was created despite the index-staged dirty refusal")
	}
}
