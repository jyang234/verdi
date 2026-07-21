// R4-I-84's own behavioral witnesses (controller adjudication 2026-07-20,
// CRITICAL-class): a clean, unmanaged, non-primary, non-invoking worktree
// checked out ON the default branch must be KEPT (reason default-branch),
// never reclaimed — and the reason the plan-time keep is load-bearing is
// that git's own --apply second guard does NOT protect the default branch
// locally (`git branch -d <default>` deletes it). Both are proven against
// throwaway fixturegit repositories only; NEVER the real repo.
package reclaim

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/residue"
)

// addDefaultBranchWorktree reproduces this workspace's own hazard topology
// against root (whose primary must currently be on the default branch): it
// moves the primary OFF the default branch — onto design/primary, with its
// own commit so design/primary is AHEAD of main and never itself a merged-
// branch row — then cuts a second, clean, UNMANAGED worktree checked out ON
// the default branch (outside .verdi/data/worktrees, so it reads Managed=
// false). That second worktree is the exact ELIGIBLE-shaped input R4-I-84
// defends against: merged (its HEAD is the default tip, a reflexive
// ancestor), clean, unmanaged, and — when the sweep is invoked from a third
// location — non-invoking. Returns its path.
func addDefaultBranchWorktree(t *testing.T, root string) (mainWtPath string) {
	t.Helper()
	ctx := context.Background()

	if err := gitx.CheckoutNewBranch(ctx, root, "design/primary"); err != nil {
		t.Fatalf("CheckoutNewBranch(design/primary): %v", err)
	}
	mustWriteFile(t, root, "primary.txt", "p\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "primary work")

	mainWtPath = filepath.Join(t.TempDir(), "main-wt")
	if err := gitx.WorktreeAdd(ctx, root, mainWtPath, "main"); err != nil {
		t.Fatalf("WorktreeAdd(main): %v", err)
	}
	return mainWtPath
}

// TestCompute_DefaultBranchWorktree_KeptNotReclaimed is R4-I-84's core
// behavioral witness against real git: a worktree on the default branch is
// kept:default-branch, while a genuinely-eligible non-default worktree in the
// SAME survey stays eligible (the guard is not over-broad). The sweep is
// invoked from a third location, so the default-branch row is kept ONLY by
// the new arm — never incidentally by the invoking exclusion.
func TestCompute_DefaultBranchWorktree_KeptNotReclaimed(t *testing.T) {
	root := newReclaimTestRepo(t) // primary on main
	ctx := context.Background()

	// A genuinely-eligible, non-default, merged+clean+unmanaged pair — cut
	// while the primary is still on main (cutEligiblePair returns it to main).
	eligible := cutEligiblePair(t, root, "eligible")

	// The hazard: primary off main, a second unmanaged worktree ON main.
	mainWtPath := addDefaultBranchWorktree(t, root)

	// The sweep is invoked from a THIRD location (neither the primary nor the
	// default-branch worktree), on a branch matching nothing here — so nothing
	// is kept for the invoking reason and the default-branch arm is isolated.
	thirdLocation := filepath.Join(t.TempDir(), "third")

	res, err := residue.Scan(ctx, root, "main")
	if err != nil {
		t.Fatalf("residue.Scan: %v", err)
	}

	// The dangerous input shape, asserted at the residue seam: the default-
	// branch worktree surfaces merged+clean+unmanaged (reclaim-eligible-
	// shaped) with Branch == the default branch. This is precisely the row
	// the predicate must now keep — pre-fix it classified ELIGIBLE.
	var mainRow *residue.Worktree
	for i := range res.Worktrees {
		if res.Worktrees[i].Branch == "main" {
			mainRow = &res.Worktrees[i]
		}
	}
	if mainRow == nil {
		t.Fatalf("residue did not surface a worktree row on the default branch: %+v", res.Worktrees)
	}
	if !mainRow.Merged || mainRow.Dirty || mainRow.Managed || mainRow.MergedUnresolved || mainRow.DirtyUnresolved {
		t.Fatalf("fixture premise broken: the default-branch worktree must be merged+clean+unmanaged (reclaim-eligible-shaped), got %+v", *mainRow)
	}

	plan := Compute(res, thirdLocation, "some-invoking-branch", "main")

	// The default-branch worktree: kept, reason default-branch — never eligible.
	mainItem := planItemFor(t, plan, "main")
	if mainItem.Eligible {
		t.Fatalf("default-branch worktree classified ELIGIBLE; --apply could delete the local default-branch ref (R4-I-84)")
	}
	if mainItem.Reason != KeptDefaultBranch {
		t.Fatalf("default-branch worktree kept:%s, want kept:default-branch", mainItem.Reason)
	}
	if realOrSelf(mainItem.Unit.WorktreePath) != realOrSelf(mainWtPath) {
		t.Fatalf("default-branch item WorktreePath = %q, want %q", mainItem.Unit.WorktreePath, mainWtPath)
	}

	// The non-default eligible pair: still eligible — the guard is not over-broad.
	eligItem := planItemFor(t, plan, eligible.branch)
	if !eligItem.Eligible {
		t.Fatalf("non-default merged+clean worktree %s kept:%s; the default-branch guard must not be over-broad", eligible.branch, eligItem.Reason)
	}
	if realOrSelf(eligItem.Unit.WorktreePath) != realOrSelf(eligible.path) {
		t.Fatalf("eligible item WorktreePath = %q, want %q", eligItem.Unit.WorktreePath, eligible.path)
	}
}

// TestGitDoesNotProtectDefaultBranchLocally_SecondGuardAbsent documents WHY
// the plan-time keep above is load-bearing rather than merely tidy: git's own
// --apply second guard does NOT protect the default branch. Mirroring
// applyOne's exact sequence in a throwaway fixture — remove the worktree
// (git allows it, clean tree), then `git branch -d main` — git DELETES the
// local default-branch ref (HEAD, on design/primary, is ahead of main, so git
// sees main as fully merged into HEAD). The destructive `git branch -d` runs
// ONLY against a fixturegit t.TempDir; it is never, under any path, run
// against the real repository (R4-I-84's own "NEVER run that against the real
// repo").
func TestGitDoesNotProtectDefaultBranchLocally_SecondGuardAbsent(t *testing.T) {
	root := newReclaimTestRepo(t) // fixturegit temp repo, primary on main
	ctx := context.Background()

	mainWtPath := addDefaultBranchWorktree(t, root)

	// --apply step 1: `git worktree remove` (no --force) succeeds on a clean
	// tree — the first "second guard" does not fire.
	if err := gitx.WorktreeRemove(ctx, root, mainWtPath); err != nil {
		t.Fatalf("worktree removal refused unexpectedly: %v", err)
	}

	// --apply step 2: `git branch -d main`. If git protected the default
	// branch, this would refuse; it does not.
	cmd := exec.Command("git", "branch", "-d", "main")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected `git branch -d main` to SUCCEED (proving the second guard is absent), but it errored: %v\n%s", err, out)
	}
	t.Logf("second guard absent: `git branch -d main` succeeded: %s", out)

	has, err := gitx.HasLocalBranch(ctx, root, "main")
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("inconsistent: `main` still present after an unrefused `git branch -d main`")
	}
	// The local default-branch ref is gone — exactly the loss the plan-time
	// keep (TestCompute_DefaultBranchWorktree_KeptNotReclaimed) now prevents.
}
