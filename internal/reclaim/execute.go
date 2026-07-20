package reclaim

import (
	"context"

	"github.com/jyang234/verdi/internal/gitx"
)

// worktreeRemove / deleteMergedBranch sit behind package-level vars — the
// same test seam internal/wtmanager's own worktreeRemove var establishes
// (gc.go) — so a test can wrap them to COUNT or force-fail invocations
// (e.g. proving a worktree-remove refusal skips the branch-delete step
// entirely) without a second, parallel implementation. Production callers
// never override these.
var (
	worktreeRemove     = gitx.WorktreeRemove
	deleteMergedBranch = gitx.DeleteMergedBranch
)

// Apply executes plan's eligible items, in plan's own order (ac-2, dc-3):
// per eligible item, its worktree (if any) is removed FIRST — via the
// existing gitx.WorktreeRemove, WITHOUT --force, git's own dirty-tree
// refusal a second, independent guard beyond the plan's own Dirty fact —
// then its branch is deleted via the new gitx.DeleteMergedBranch (never
// -D: git's own merged/checked-out-anywhere refusal a second, independent
// guard beyond the plan's own Merged fact). A branch-only item skips the
// worktree step entirely, never calling WorktreeRemove on an empty path.
//
// A worktree-remove refusal skips the branch-delete step entirely for that
// item (KindRefused) rather than attempting a delete git would refuse
// anyway; a branch-delete failure AFTER a successful worktree removal is
// its own, distinct KindPartial outcome (dc-4) — never folded into a
// generic failure. Every kept item (AC-1's own plan-time exclusion) passes
// through UNCHANGED — never re-decided by --apply (dc-1: "share the
// identical eligibility computation").
//
// A per-item refusal never aborts the sweep (ac-2: "the sweep continues to
// the next item"); Apply itself never returns an error — every outcome,
// including every refusal, is a Row, disclosed, not a propagated error.
func Apply(ctx context.Context, root string, plan Plan) []Row {
	rows := make([]Row, 0, len(plan.Items))
	for _, item := range plan.Items {
		rows = append(rows, applyOne(ctx, root, item))
	}
	return rows
}

// applyOne executes (or, for a kept item, simply reports) one PlanItem.
func applyOne(ctx context.Context, root string, item PlanItem) Row {
	if !item.Eligible {
		return Row{Kind: KindKept, Unit: item.Unit, Reason: item.Reason, Detail: item.Detail}
	}

	if item.Unit.HasWorktree() {
		if err := worktreeRemove(ctx, root, item.Unit.WorktreePath); err != nil {
			return Row{Kind: KindRefused, Unit: item.Unit, Detail: err.Error()}
		}
	}

	tip, err := deleteMergedBranch(ctx, root, item.Unit.Branch)
	if err != nil {
		if item.Unit.HasWorktree() {
			// The worktree is already gone; the branch is not — dc-4's own
			// distinct partial outcome, never a generic failure.
			return Row{Kind: KindPartial, Unit: item.Unit, Detail: err.Error()}
		}
		return Row{Kind: KindRefused, Unit: item.Unit, Detail: err.Error()}
	}
	return Row{Kind: KindReclaimed, Unit: item.Unit, Tip: tip}
}
