// Package reclaim's execution-engine tests: five of ac-2--behavioral's six
// fixturegit cases (the sixth, an unresolvable default branch, is not a
// concept this package's own Compute/Apply signatures can even exhibit —
// Compute takes an already-resolved *residue.Result; that refusal lives at
// cmd/verdi/gc.go's own orchestration layer, proven there).
package reclaim

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/residue"
)

// rowFor finds the Row for branch among rows, failing the test if absent —
// AC-1/AC-3's own "never silently dropped" posture applies to test
// fixtures too: a missing row is a test bug or a real regression, never
// something to skip past quietly.
func rowFor(t *testing.T, rows []Row, branch string) Row {
	t.Helper()
	for _, r := range rows {
		if r.Unit.Branch == branch {
			return r
		}
	}
	t.Fatalf("no row for branch %q among %d rows: %+v", branch, len(rows), rows)
	return Row{}
}

// Case 1: dry-run performs zero git-mutating calls, and still prints the
// item as eligible. Plan.DryRunRows itself takes no context.Context and no
// root — a type-level guarantee it cannot mutate anything — this test
// additionally proves it BEHAVIORALLY, against the fixture's own git state.
func TestPlan_DryRunRows_ZeroMutation(t *testing.T) {
	root := newReclaimTestRepo(t)
	pair := cutEligiblePair(t, root, "clean")
	ctx := context.Background()

	branchesBefore, err := gitx.LocalBranches(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	worktreesBefore, err := gitx.WorktreeList(ctx, root)
	if err != nil {
		t.Fatal(err)
	}

	res, err := residue.Scan(ctx, root, "main")
	if err != nil {
		t.Fatal(err)
	}
	plan := Compute(res, root, "main")
	rows := plan.DryRunRows()

	branchesAfter, err := gitx.LocalBranches(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	worktreesAfter, err := gitx.WorktreeList(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(branchesBefore, branchesAfter) {
		t.Fatalf("local branches changed by a dry run: before=%v after=%v", branchesBefore, branchesAfter)
	}
	if !reflect.DeepEqual(worktreesBefore, worktreesAfter) {
		t.Fatalf("worktree registrations changed by a dry run: before=%+v after=%+v", worktreesBefore, worktreesAfter)
	}

	row := rowFor(t, rows, pair.branch)
	if row.Kind != KindEligible {
		t.Fatalf("Kind = %v, want KindEligible: %+v", row.Kind, row)
	}
	// realOrSelf: residue.Worktree.Path comes from git itself, already
	// symlink-resolved (e.g. macOS's /private/var/... form); pair.path is
	// the UNRESOLVED form this test originally passed to gitx.WorktreeAdd —
	// the same D6-8-class parity the reclaim package's own canonicalPath
	// exists to survive, mirrored here for the test's OWN assertion (not
	// production code, which never compares these two strings this way).
	if realOrSelf(row.Unit.WorktreePath) != realOrSelf(pair.path) {
		t.Fatalf("WorktreePath = %q, want %q", row.Unit.WorktreePath, pair.path)
	}
}

// Case 2: --apply on a clean eligible pair removes the worktree, then
// deletes the branch, in that order, printing the branch's own pre-delete
// tip commit.
func TestApply_CleanEligiblePair_ReclaimsInOrder_PrintsTip(t *testing.T) {
	root := newReclaimTestRepo(t)
	pair := cutEligiblePair(t, root, "clean")
	ctx := context.Background()

	res, err := residue.Scan(ctx, root, "main")
	if err != nil {
		t.Fatal(err)
	}
	plan := Compute(res, root, "main")

	rows := Apply(ctx, root, plan)
	row := rowFor(t, rows, pair.branch)
	if row.Kind != KindReclaimed {
		t.Fatalf("Kind = %v, want KindReclaimed: %+v", row.Kind, row)
	}
	if row.Tip != pair.tip {
		t.Fatalf("Tip = %q, want %q (the branch's own pre-delete tip)", row.Tip, pair.tip)
	}
	if _, err := os.Stat(pair.path); !os.IsNotExist(err) {
		t.Fatalf("worktree %s still on disk after Apply: err=%v", pair.path, err)
	}
	has, err := gitx.HasLocalBranch(ctx, root, pair.branch)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatalf("branch %s still exists after Apply", pair.branch)
	}
}

// Case 3 (first second-guard witness): a worktree reported clean at scan
// time but dirtied BEFORE --apply runs is refused via `git worktree
// remove`'s own refusal — disclosed, with the sweep continuing to a SECOND,
// unaffected item. The branch-delete step is proven never even attempted
// (not merely that the branch happens to survive, which git's own refusal
// would guarantee regardless).
func TestApply_WorktreeDirtiedBeforeApply_RefusedWithoutAttemptingDelete_SweepContinues(t *testing.T) {
	root := newReclaimTestRepo(t)
	dirtied := cutEligiblePair(t, root, "dirtied")
	clean := cutEligiblePair(t, root, "clean")
	ctx := context.Background()

	// The race, reproduced by SEQUENCING rather than mocking (co-1): scan
	// and compute the plan FIRST, while both worktrees are still clean (so
	// `dirtied` is genuinely plan-eligible), then dirty `dirtied`'s
	// worktree out of band — exactly ac-2's own "reported clean at scan
	// time but dirtied BEFORE --apply runs" race.
	res, err := residue.Scan(ctx, root, "main")
	if err != nil {
		t.Fatal(err)
	}
	plan := Compute(res, root, "main")
	if item := planItemFor(t, plan, dirtied.branch); !item.Eligible {
		t.Fatalf("precondition: dirtied item = %+v, want Eligible at plan time (dirtying happens AFTER this)", item)
	}

	if err := os.WriteFile(filepath.Join(dirtied.path, "wip.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var deleteAttempts int
	orig := deleteMergedBranch
	deleteMergedBranch = func(ctx context.Context, dir, name string) (string, error) {
		if name == dirtied.branch {
			deleteAttempts++
		}
		return orig(ctx, dir, name)
	}
	defer func() { deleteMergedBranch = orig }()

	rows := Apply(ctx, root, plan)

	dirtiedRow := rowFor(t, rows, dirtied.branch)
	if dirtiedRow.Kind != KindRefused {
		t.Fatalf("dirtied row = %+v, want KindRefused", dirtiedRow)
	}
	if dirtiedRow.Detail == "" {
		t.Fatal("Refused row Detail is empty; want git's own refusal disclosed")
	}
	if deleteAttempts != 0 {
		t.Fatalf("DeleteMergedBranch attempted %d time(s) for %s; want 0 (a worktree-remove refusal must skip the branch-delete step entirely)", deleteAttempts, dirtied.branch)
	}
	if _, err := os.Stat(dirtied.path); err != nil {
		t.Fatalf("dirtied worktree removed despite the refusal: %v", err)
	}
	has, err := gitx.HasLocalBranch(ctx, root, dirtied.branch)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("dirtied branch deleted despite the worktree-remove refusal")
	}

	cleanRow := rowFor(t, rows, clean.branch)
	if cleanRow.Kind != KindReclaimed {
		t.Fatalf("clean row = %+v, want KindReclaimed (the sweep must continue past the first item's refusal)", cleanRow)
	}
}

// Case 4 (second second-guard witness, ledger R4-I-80): a branch-only row
// (a merged branch with no worktree of its own) that happens to be checked
// out AT PRIMARY (residue.Scan's Worktrees never contains primary, so this
// can only ever surface as a branch-only row, never a worktree row) and is
// NOT the invoking checkout's own branch is marked ELIGIBLE by Compute
// (dc-2's own documented, disclosed limitation) — and then refused by
// `git branch -d`'s own second, independent guard at Apply time.
func TestApply_BranchOnlyRowCheckedOutAtPrimary_RefusedViaBranchDelete_LedgerR4I80(t *testing.T) {
	root := newReclaimTestRepo(t)
	ctx := context.Background()
	branch := "primary-branch"

	if err := gitx.CheckoutNewBranch(ctx, root, branch); err != nil {
		t.Fatalf("CheckoutNewBranch(%s): %v", branch, err)
	}
	if err := os.WriteFile(filepath.Join(root, "p.txt"), []byte("p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "work on "+branch)
	checkoutMain(t, root)
	runGit(t, root, "merge", "--quiet", "--no-ff", "-m", "merge "+branch, branch)
	// Leave PRIMARY checked out on `branch` again (not main) — the ONLY way
	// a merged branch can be "checked out somewhere" yet carry no worktree
	// row at all.
	runGit(t, root, "checkout", "--quiet", branch)

	res, err := residue.Scan(ctx, root, "main")
	if err != nil {
		t.Fatal(err)
	}
	for _, wt := range res.Worktrees {
		if wt.Branch == branch {
			t.Fatalf("test precondition violated: %s appears as a worktree row (residue.Scan must exclude primary): %+v", branch, wt)
		}
	}

	// invokingBranch deliberately NOT `branch`: gc is simulated as invoked
	// from somewhere else entirely, so the cheap invoking check cannot
	// catch this row — only git's own second guard can (dc-2's own point).
	plan := Compute(res, "/nonexistent/invoking/root", "main")
	item := planItemFor(t, plan, branch)
	if !item.Eligible {
		t.Fatalf("plan-time: %s = %+v, want Eligible (R4-I-80: the predicate does not pre-classify this corner)", branch, item)
	}

	rows := Apply(ctx, root, plan)
	row := rowFor(t, rows, branch)
	if row.Kind != KindRefused {
		t.Fatalf("apply-time: %s row = %+v, want KindRefused (git's own branch -d refusal)", branch, row)
	}
	if row.Detail == "" {
		t.Fatal("Refused row Detail is empty; want git's own refusal text disclosed")
	}
	has, err := gitx.HasLocalBranch(ctx, root, branch)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("branch deleted despite being checked out at primary — git's own refusal did not fire")
	}
}

// Case 5: a worktree whose removal succeeds but whose paired branch delete
// is then forced to fail asserts the dedicated Partial outcome, never
// KindReclaimed or a generic KindRefused. Forced hermetically (no mocking,
// co-1): a new commit lands ON THE BRANCH from inside its own worktree
// AFTER the scan, leaving the worktree perfectly clean (so `git worktree
// remove` still succeeds) but the branch no longer fully merged (so `git
// branch -d` refuses it).
func TestApply_BranchAdvancedAfterScan_PartialOutcome(t *testing.T) {
	root := newReclaimTestRepo(t)
	pair := cutEligiblePair(t, root, "moved")
	ctx := context.Background()

	res, err := residue.Scan(ctx, root, "main")
	if err != nil {
		t.Fatal(err)
	}
	plan := Compute(res, root, "main")
	if item := planItemFor(t, plan, pair.branch); !item.Eligible {
		t.Fatalf("precondition: %s = %+v, want Eligible before the race", pair.branch, item)
	}

	if err := os.WriteFile(filepath.Join(pair.path, "late.txt"), []byte("late\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, pair.path, "add", "-A")
	runGit(t, pair.path, "commit", "--quiet", "-m", "late work, after the scan")

	rows := Apply(ctx, root, plan)
	row := rowFor(t, rows, pair.branch)
	if row.Kind != KindPartial {
		t.Fatalf("row = %+v, want KindPartial", row)
	}
	if row.Detail == "" {
		t.Fatal("Partial row Detail is empty; want the branch-delete refusal disclosed")
	}
	if _, err := os.Stat(pair.path); !os.IsNotExist(err) {
		t.Fatalf("worktree %s still on disk after a Partial outcome; want it removed (only the branch delete failed): err=%v", pair.path, err)
	}
	has, err := gitx.HasLocalBranch(ctx, root, pair.branch)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("branch deleted despite a Partial outcome — it must remain (the delete failed)")
	}
}

// TestApply_BranchOnlyEligible_ReclaimsWithoutAnyWorktreeCall proves the
// branch-only shape skips the worktree step entirely — never calling
// worktreeRemove on an empty path — while still fully deleting the branch
// and printing its tip.
func TestApply_BranchOnlyEligible_ReclaimsWithoutAnyWorktreeCall(t *testing.T) {
	root := newReclaimTestRepo(t)
	ctx := context.Background()
	branch := "orphan-merged"

	if err := gitx.CheckoutNewBranch(ctx, root, branch); err != nil {
		t.Fatalf("CheckoutNewBranch(%s): %v", branch, err)
	}
	if err := os.WriteFile(filepath.Join(root, "o.txt"), []byte("o\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "work on "+branch)
	wantTip, err := gitx.RevParse(ctx, root, branch)
	if err != nil {
		t.Fatal(err)
	}
	checkoutMain(t, root)
	runGit(t, root, "merge", "--quiet", "--no-ff", "-m", "merge "+branch, branch)

	var worktreeRemoveCalled bool
	origWTRemove := worktreeRemove
	worktreeRemove = func(ctx context.Context, dir, path string) error {
		worktreeRemoveCalled = true
		return origWTRemove(ctx, dir, path)
	}
	defer func() { worktreeRemove = origWTRemove }()

	res, err := residue.Scan(ctx, root, "main")
	if err != nil {
		t.Fatal(err)
	}
	plan := Compute(res, root, "main")
	item := planItemFor(t, plan, branch)
	if !item.Eligible || item.Unit.HasWorktree() {
		t.Fatalf("precondition: %s = %+v, want an eligible branch-only unit", branch, item)
	}

	rows := Apply(ctx, root, plan)
	row := rowFor(t, rows, branch)
	if row.Kind != KindReclaimed {
		t.Fatalf("row = %+v, want KindReclaimed", row)
	}
	if row.Tip != wantTip {
		t.Fatalf("Tip = %q, want %q", row.Tip, wantTip)
	}
	if worktreeRemoveCalled {
		t.Fatal("worktreeRemove was called for a branch-only unit; it must be skipped entirely")
	}
}
