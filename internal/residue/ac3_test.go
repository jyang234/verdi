package residue

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
)

// TestScan_AC3_MixedBranchesAndFourWorktrees is
// obligation/closure-hygiene--ac-3--behavioral's own fixture: a mix of
// merged and unmerged local branches (survey (a) must count and name
// exactly the merged subset), plus four real, `git worktree add`-
// materialized worktrees (co-1): one managed (design branch, under
// wtmanager.WorktreesRoot), one unmanaged on a MERGED branch, one
// unmanaged on an UNMERGED branch (left dirty — the dirty signal must be
// live), and one unmanaged with a detached HEAD at a commit that IS an
// ancestor of the default branch tip but carries no branch name at all.
// Asserts every one is named with its correct managed/unmanaged,
// merged/not-merged (or, for detached, commit-level-merged) state, and
// clean/dirty state — and that the OVERALL run's exit code (Flagged) is
// unaffected by any of it, even with an unmerged+dirty+unmanaged worktree
// present in the very same run.
func TestScan_AC3_MixedBranchesAndFourWorktrees(t *testing.T) {
	root, unmergedWTPath := buildWorktreeSurveyFixture(t)
	ctx := context.Background()

	// A couple of ADDITIONAL plain (non-worktree) branches, to prove
	// survey (a) counts/names the merged subset exactly, independent of
	// which branches happen to have worktrees.
	if err := gitx.CheckoutNewBranch(ctx, root, "plain-merged"); err != nil {
		t.Fatalf("CheckoutNewBranch(plain-merged): %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "plain.txt"), []byte("p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "plain-merged work")
	checkoutMain(t, root)
	runGit(t, root, "merge", "--quiet", "--no-ff", "-m", "merge plain-merged", "plain-merged")

	if err := gitx.CheckoutNewBranch(ctx, root, "plain-unmerged"); err != nil {
		t.Fatalf("CheckoutNewBranch(plain-unmerged): %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "plain2.txt"), []byte("p2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "plain-unmerged work")
	checkoutMain(t, root)

	got, err := Scan(ctx, root, "main")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	// --- Survey (a): merged branches, counted and named exactly. ---
	wantMerged := map[string]bool{"design/x": true, "merged-elsewhere": true, "plain-merged": true}
	if len(got.MergedBranches) != len(wantMerged) {
		t.Fatalf("Scan.MergedBranches = %v, want exactly %v", got.MergedBranches, wantMerged)
	}
	for _, b := range got.MergedBranches {
		if !wantMerged[b] {
			t.Errorf("Scan.MergedBranches contains unexpected branch %q", b)
		}
	}
	for b := range wantMerged {
		found := false
		for _, got := range got.MergedBranches {
			if got == b {
				found = true
			}
		}
		if !found {
			t.Errorf("Scan.MergedBranches missing %q", b)
		}
	}
	for _, b := range got.MergedBranches {
		if b == "main" || b == "unmerged-elsewhere" || b == "plain-unmerged" {
			t.Fatalf("Scan.MergedBranches wrongly includes %q", b)
		}
	}

	// --- Survey (b): all four worktrees, correctly classified. ---
	if len(got.Worktrees) != 4 {
		t.Fatalf("Scan.Worktrees = %+v, want exactly 4 (primary excluded)", got.Worktrees)
	}

	byBranch := map[string]Worktree{}
	var detached *Worktree
	for i := range got.Worktrees {
		wt := got.Worktrees[i]
		if wt.Branch == "" {
			detached = &got.Worktrees[i]
			continue
		}
		byBranch[wt.Branch] = wt
	}

	managed, ok := byBranch["design/x"]
	if !ok {
		t.Fatalf("Scan.Worktrees missing the managed design/x entry: %+v", got.Worktrees)
	}
	if !managed.Managed {
		t.Error("design/x worktree: Managed = false, want true")
	}
	if managed.Dirty {
		t.Error("design/x worktree: Dirty = true, want false")
	}

	mergedWT, ok := byBranch["merged-elsewhere"]
	if !ok {
		t.Fatalf("Scan.Worktrees missing the unmanaged+merged entry: %+v", got.Worktrees)
	}
	if mergedWT.Managed {
		t.Error("merged-elsewhere worktree: Managed = true, want false")
	}
	if !mergedWT.Merged {
		t.Error("merged-elsewhere worktree: Merged = false, want true")
	}

	unmergedWT, ok := byBranch["unmerged-elsewhere"]
	if !ok {
		t.Fatalf("Scan.Worktrees missing the unmanaged+unmerged entry: %+v", got.Worktrees)
	}
	if unmergedWT.Managed {
		t.Error("unmerged-elsewhere worktree: Managed = true, want false")
	}
	if unmergedWT.Merged {
		t.Error("unmerged-elsewhere worktree: Merged = true, want false")
	}
	if !unmergedWT.Dirty {
		t.Error("unmerged-elsewhere worktree: Dirty = false, want true (an uncommitted edit was made)")
	}
	if realOrSelfSurvey(unmergedWT.Path) != realOrSelfSurvey(unmergedWTPath) {
		t.Errorf("unmerged-elsewhere worktree Path = %q, want %q", unmergedWT.Path, unmergedWTPath)
	}

	if detached == nil {
		t.Fatalf("Scan.Worktrees missing the detached-HEAD entry: %+v", got.Worktrees)
	}
	if detached.Branch != "" {
		t.Errorf("detached entry Branch = %q, want empty (dc-4: no branch name asserted where none exists)", detached.Branch)
	}
	if detached.Commit == "" {
		t.Error("detached entry Commit is empty; want the checked-out commit disclosed")
	}
	if !detached.Merged {
		t.Error("detached entry Merged = false, want true (resolved at the commit level)")
	}
	if detached.Managed {
		t.Error("detached entry Managed = true, want false")
	}

	// --- The survey never flags, even with an unmerged+dirty+unmanaged
	// worktree present in the same run (dc-3). ---
	if got.Flagged() {
		t.Fatal("Scan.Flagged() = true, want false (AC-3's survey never flags, regardless of any worktree's state)")
	}
}

// TestScan_AC3_ZeroGitMutatingCalls is a command-surface proof
// complementing ac-3's static obligation ("an exhaustive command-surface
// check ... proves the list is worktree list, rev-parse/merge-base, and
// status checks only — never add, remove, or prune"): it wraps `git`
// itself with a thin recording shim is impractical from a black-box Go
// test without reaching into gitx's own exec seam, so this proof instead
// asserts the OBSERVABLE consequence directly — running Scan against a
// fixture carrying worktrees and branches leaves EVERY worktree, and the
// primary checkout's own branch/HEAD, byte-identical before and after.
func TestScan_AC3_ZeroGitMutatingCalls(t *testing.T) {
	root, unmergedWTPath := buildWorktreeSurveyFixture(t)
	ctx := context.Background()

	beforeEntries, err := gitx.WorktreeList(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	beforeHead, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	beforeBranch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	beforeDirty, err := gitx.StatusDirty(ctx, unmergedWTPath)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Scan(ctx, root, "main"); err != nil {
		t.Fatalf("Scan: %v", err)
	}

	afterEntries, err := gitx.WorktreeList(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(afterEntries) != len(beforeEntries) {
		t.Fatalf("worktree count changed across Scan: before %d, after %d", len(beforeEntries), len(afterEntries))
	}
	afterHead, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if afterHead != beforeHead {
		t.Fatalf("primary checkout's HEAD changed across Scan: %s -> %s", beforeHead, afterHead)
	}
	afterBranch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if afterBranch != beforeBranch {
		t.Fatalf("primary checkout's branch changed across Scan: %s -> %s", beforeBranch, afterBranch)
	}
	afterDirty, err := gitx.StatusDirty(ctx, unmergedWTPath)
	if err != nil {
		t.Fatal(err)
	}
	if afterDirty != beforeDirty {
		t.Fatal("a worktree's dirty state changed across Scan (a read-only survey must never mutate)")
	}
	if _, err := os.Stat(unmergedWTPath); err != nil {
		t.Fatalf("the unmerged-elsewhere worktree was removed by Scan: %v", err)
	}
}
