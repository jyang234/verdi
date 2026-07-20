// obligation/gc-reclaim--ac-1--behavioral: one fixturegit repository whose
// internal/residue.Scan result, fed into Compute, contains — in one
// survey — every eligible and every excluded row shape AC-1 names, and
// every single row is asserted named exactly once, with its correct
// eligible-or-kept-and-reason classification — never silently dropped.
package reclaim

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/residue"
	"github.com/jyang234/verdi/internal/wtmanager"
)

func TestCompute_CombinedSurvey_EveryRowClassifiedExactlyOnce(t *testing.T) {
	root := newReclaimTestRepo(t)
	ctx := context.Background()

	// 1. Eligible worktree+branch pair.
	eligible := cutEligiblePair(t, root, "eligible")

	// 2. Eligible branch-only row: merged, no worktree at all (mirrors this
	// repository's own live board-polish-shaped witness).
	const orphanBranch = "merged-orphan"
	if err := gitx.CheckoutNewBranch(ctx, root, orphanBranch); err != nil {
		t.Fatalf("CheckoutNewBranch(%s): %v", orphanBranch, err)
	}
	mustWriteFile(t, root, "orphan.txt", "orphan\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "orphan work")
	checkoutMain(t, root)
	runGit(t, root, "merge", "--quiet", "--no-ff", "-m", "merge "+orphanBranch, orphanBranch)

	// 3. Unmerged row (mirrors the spec's own four live close/<name>
	// witnesses: superseded-elsewhere is not the same fact as merged).
	const unmergedBranch = "design/unmerged"
	if err := gitx.CheckoutNewBranch(ctx, root, unmergedBranch); err != nil {
		t.Fatalf("CheckoutNewBranch(%s): %v", unmergedBranch, err)
	}
	mustWriteFile(t, root, "unmerged.txt", "u\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "unmerged work")
	checkoutMain(t, root)
	unmergedWTPath := filepath.Join(t.TempDir(), "unmerged-wt")
	if err := gitx.WorktreeAdd(ctx, root, unmergedWTPath, unmergedBranch); err != nil {
		t.Fatalf("WorktreeAdd(%s): %v", unmergedBranch, err)
	}

	// 4. Dirty row: merged, worktree left with an uncommitted change.
	dirty := cutEligiblePair(t, root, "dirty")
	mustWriteFile(t, dirty.path, "wip.txt", "wip\n")

	// 5. Unresolved-state row: merged, worktree directory deleted WITHOUT
	// `git worktree remove` (git marks it prunable — residue's own
	// disclosed-rather-than-guessed path; mirrors residue/survey_test.go's
	// own TestScanWorktrees_StaleWorktreeDisclosedNotAborted).
	stale := cutEligiblePair(t, root, "stale")
	if err := os.RemoveAll(stale.path); err != nil {
		t.Fatalf("RemoveAll(%s): %v", stale.path, err)
	}

	// 6. Detached-HEAD row (mirrors the live w6-exit witness): no branch at
	// all, checked out at a commit that IS an ancestor of main's tip.
	detachAt, err := gitx.RevParse(ctx, root, "main")
	if err != nil {
		t.Fatal(err)
	}
	detachedPath := filepath.Join(t.TempDir(), "detached-wt")
	runGit(t, root, "worktree", "add", "--detach", "--quiet", detachedPath, detachAt)

	// 7. Managed-worktree row, cut via the real production entry point
	// (mirrors residue/survey_test.go's own buildWorktreeSurveyFixture).
	const managedBranch = "design/managed"
	if err := gitx.CheckoutNewBranch(ctx, root, managedBranch); err != nil {
		t.Fatalf("CheckoutNewBranch(%s): %v", managedBranch, err)
	}
	mustWriteFile(t, root, "managed.txt", "m\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "managed work")
	checkoutMain(t, root)
	runGit(t, root, "merge", "--quiet", "--no-ff", "-m", "merge "+managedBranch, managedBranch)
	if _, err := wtmanager.EnsureWorktree(ctx, root, managedBranch); err != nil {
		t.Fatalf("EnsureWorktree(%s): %v", managedBranch, err)
	}

	// 8. Invoking row (mirrors this very story's own worktree,
	// verdi-wt/residue-reclamation): otherwise eligible-shaped (merged,
	// clean, unmanaged), kept ONLY because it is where the sweep itself is
	// running from.
	invoking := cutEligiblePair(t, root, "invoking")

	res, err := residue.Scan(ctx, root, "main")
	if err != nil {
		t.Fatalf("residue.Scan: %v", err)
	}

	plan := Compute(res, invoking.path, "branch-matching-nothing-in-this-fixture")

	const wantRowCount = 8 // eligible, orphan, unmerged, dirty, stale, detached, managed, invoking
	if len(plan.Items) != wantRowCount {
		t.Fatalf("Compute produced %d items, want %d: %+v", len(plan.Items), wantRowCount, plan.Items)
	}

	byBranch := map[string]PlanItem{}
	var detachedCount int
	for _, item := range plan.Items {
		if item.Unit.Branch == "" {
			detachedCount++
			continue
		}
		if _, dup := byBranch[item.Unit.Branch]; dup {
			t.Fatalf("branch %q appears twice in the plan; every row must be classified EXACTLY once", item.Unit.Branch)
		}
		byBranch[item.Unit.Branch] = item
	}
	if detachedCount != 1 {
		t.Fatalf("got %d detached (empty-Branch) rows, want exactly 1", detachedCount)
	}

	wantKept := func(branch string, wantReason KeptReason) {
		t.Helper()
		item, ok := byBranch[branch]
		if !ok {
			t.Fatalf("%s missing from the plan entirely (row silently dropped)", branch)
		}
		if item.Eligible {
			t.Errorf("%s: Eligible = true, want kept:%s", branch, wantReason)
			return
		}
		if item.Reason != wantReason {
			t.Errorf("%s: Reason = %s, want %s", branch, item.Reason, wantReason)
		}
	}
	wantEligible := func(branch, wantWorktreePath string) {
		t.Helper()
		item, ok := byBranch[branch]
		if !ok {
			t.Fatalf("%s missing from the plan entirely (row silently dropped)", branch)
		}
		if !item.Eligible {
			t.Errorf("%s: Eligible = false (reason %s), want true", branch, item.Reason)
			return
		}
		if realOrSelf(item.Unit.WorktreePath) != realOrSelf(wantWorktreePath) {
			t.Errorf("%s: WorktreePath = %q, want %q", branch, item.Unit.WorktreePath, wantWorktreePath)
		}
	}

	wantEligible(eligible.branch, eligible.path)
	wantEligible(orphanBranch, "")
	wantKept(unmergedBranch, KeptUnmerged)
	wantKept(dirty.branch, KeptDirty)
	wantKept(stale.branch, KeptUnresolvedState)
	wantKept(managedBranch, KeptManaged)
	wantKept(invoking.branch, KeptInvoking)

	// The stale row's unresolved-state Detail must carry residue's own
	// Reason text (dc-2: "naming residue's own Reason"), never blank.
	if d := byBranch[stale.branch].Detail; d == "" {
		t.Error("stale row's unresolved-state Detail is empty; want residue's own Reason disclosed")
	}
}

func mustWriteFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
