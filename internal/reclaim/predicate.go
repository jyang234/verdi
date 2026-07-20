package reclaim

import (
	"fmt"
	"path/filepath"

	"github.com/jyang234/verdi/internal/residue"
)

// Unit identifies one reclaim UNIT (spec/gc-reclaim outcome: "a LOCAL
// branch and its worktree (if any)") — a branch, and, where one exists,
// its worktree, always reported and acted on together, never as two
// separate lines or two separate actions (dc-4).
type Unit struct {
	Branch string
	// WorktreePath is "" for a branch-only unit (a merged branch with no
	// worktree of its own — AC-1: "eligible for branch deletion alone").
	WorktreePath string
}

// HasWorktree reports whether u carries a worktree component.
func (u Unit) HasWorktree() bool {
	return u.WorktreePath != ""
}

// KeptReason is AC-1's closed vocabulary for why a unit that is NOT
// eligible was kept — unmerged, dirty, unresolved-state, detached,
// managed, invoking, and NO seventh path. Declared in dc-2's own fixed
// CHECK order (unresolved-state first, invoking last); a future value
// added to this block picks up a matching keptReasonNames entry or the
// package fails to BUILD (see keptReasonNames' own doc comment) — never a
// silently blank or generic label.
type KeptReason int

const (
	KeptUnresolvedState KeptReason = iota
	KeptUnmerged
	KeptDirty
	KeptDetached
	KeptManaged
	KeptInvoking
	numKeptReasons // sentinel: always one past the last real value (iota-tracked, never hand-counted)
)

// keptReasonNames is KeptReason's own compile-time exhaustiveness check.
// The right-hand side is an ellipsis-sized array literal: its type is
// inferred solely from the highest explicit key present (here, [6]string,
// since KeptInvoking == 5). Assigning it to keptReasonNames, whose
// DECLARED type is the sentinel-sized [numKeptReasons]string, only
// compiles when those two array types are IDENTICAL — which in Go requires
// an identical length, a compile-time constant comparison. Appending a new
// KeptReason value before numKeptReasons (which auto-shifts via iota, no
// hand-maintained count to forget) without adding its own keyed entry here
// leaves the literal's inferred length one short of numKeptReasons: "cannot
// use ... (value of type [N]string) as [N+1]string value in variable
// declaration" — a genuine build failure, not a silently blank label at
// runtime. This is the ac-1--static obligation's own "compile-time
// exhaustiveness check... so a future case added to the type without a
// matching [entry] fails the build."
var keptReasonNames [numKeptReasons]string = [...]string{
	KeptUnresolvedState: "unresolved-state",
	KeptUnmerged:        "unmerged",
	KeptDirty:           "dirty",
	KeptDetached:        "detached",
	KeptManaged:         "managed",
	KeptInvoking:        "invoking",
}

// String renders r's closed-vocabulary label, or a self-naming "unknown"
// fallback for a value outside the closed set (unreachable through this
// package's own construction, but never silently blank or generic —
// CLAUDE.md: "unknown enum values fail closed").
func (r KeptReason) String() string {
	if r < 0 || int(r) >= len(keptReasonNames) {
		return fmt.Sprintf("unknown-kept-reason(%d)", int(r))
	}
	return keptReasonNames[r]
}

// PlanItem is AC-1's own per-unit classification: eligible, or kept with
// exactly one reason. Reason and Detail are meaningful only when
// !Eligible; Detail carries residue's own Reason text, populated only for
// KeptUnresolvedState (dc-2: "naming residue's own Reason").
type PlanItem struct {
	Unit     Unit
	Eligible bool
	Reason   KeptReason
	Detail   string
}

// Plan is AC-1's predicate applied to a whole *residue.Result: one
// PlanItem per reclaim unit, in residue's own deterministic order (worktree
// rows in Result.Worktrees' own path-sorted order, then branch-only rows in
// Result.MergedBranches' own sorted order) — never re-sorted here, so the
// plan's own order is as deterministic as residue.Scan's already is.
type Plan struct {
	Items []PlanItem
}

// Compute is spec/gc-reclaim AC-1's predicate, DC-2's own ordered switch:
// a pure function of res plus the invoking checkout's own already-resolved
// root (compared against worktree rows' Path) and current branch (compared
// against branch-only rows' names) — never a git call, never a re-derived
// eligibility fact (dc-1: "internal/reclaim calls zero gitx eligibility
// primitives itself").
//
// invokingRoot is best-effort symlink-resolved before comparison (mirroring
// internal/residue/survey.go's own resolvedRoot precedent) so a caller that
// passes store.FindRoot(".")'s unresolved form still matches a worktree
// Path git itself already reports resolved (the same macOS /var-vs-
// /private/var parity class internal/residue's own tests guard against).
// invokingBranch "" (a detached invoking HEAD) matches no branch-only row,
// by construction — branch names are never empty.
func Compute(res *residue.Result, invokingRoot, invokingBranch string) Plan {
	canonicalInvokingRoot := canonicalPath(invokingRoot)

	branchesWithWorktrees := make(map[string]bool, len(res.Worktrees))
	items := make([]PlanItem, 0, len(res.Worktrees)+len(res.MergedBranches))

	for _, wt := range res.Worktrees {
		if wt.Branch != "" {
			branchesWithWorktrees[wt.Branch] = true
		}
		eligible, reason, detail := classifyWorktreeRow(wt, canonicalInvokingRoot)
		items = append(items, PlanItem{
			Unit:     Unit{Branch: wt.Branch, WorktreePath: wt.Path},
			Eligible: eligible,
			Reason:   reason,
			Detail:   detail,
		})
	}

	for _, name := range res.MergedBranches {
		if branchesWithWorktrees[name] {
			// Owned by a worktree row above (dc-2): one unit, one item —
			// never a second, branch-only entry for the same branch.
			continue
		}
		eligible, reason := classifyBranchOnlyRow(name, invokingBranch)
		items = append(items, PlanItem{
			Unit:     Unit{Branch: name},
			Eligible: eligible,
			Reason:   reason,
		})
	}

	return Plan{Items: items}
}

// classifyWorktreeRow is dc-2's fixed, ordered switch for a worktree row:
// unresolved-state -> unmerged -> dirty -> detached -> managed -> invoking
// -> eligible. A row with multiple simultaneously-true exclusion facts
// still yields exactly the FIRST one in this order, deterministically
// (mirroring internal/wtmanager.decideReclaim's own ordered-switch
// precedent) — never an arbitrary or combinatorial reason.
//
// canonicalInvokingRoot must already be canonicalPath-resolved by the
// caller (Compute resolves it once, not per row).
func classifyWorktreeRow(wt residue.Worktree, canonicalInvokingRoot string) (eligible bool, reason KeptReason, detail string) {
	switch {
	case wt.MergedUnresolved || wt.DirtyUnresolved:
		return false, KeptUnresolvedState, wt.Reason
	case !wt.Merged:
		return false, KeptUnmerged, ""
	case wt.Dirty:
		return false, KeptDirty, ""
	case wt.Branch == "":
		return false, KeptDetached, ""
	case wt.Managed:
		return false, KeptManaged, ""
	case canonicalPath(wt.Path) == canonicalInvokingRoot:
		return false, KeptInvoking, ""
	default:
		return true, 0, ""
	}
}

// classifyBranchOnlyRow is dc-2's single check for a branch-only row (a
// merged branch with no worktree of its own): kept:invoking iff name is
// the invoking checkout's own current branch, else eligible — the only
// check this shape needs, since a bare branch has no worktree to be
// managed, detached, dirty, or in an unresolved state.
func classifyBranchOnlyRow(name, invokingBranch string) (eligible bool, reason KeptReason) {
	if name == invokingBranch {
		return false, KeptInvoking
	}
	return true, 0
}

// canonicalPath best-effort symlink-resolves p for a stable comparison
// against git-reported worktree paths (which come back already resolved —
// internal/gitx.WorktreeEntry's own doc comment) — falling back to p
// itself, filepath.Clean'd, when resolution fails (e.g. p does not exist on
// disk), mirroring internal/residue/survey.go's own resolvedRoot precedent
// exactly ("best effort; git's own reported paths are already resolved
// either way").
func canonicalPath(p string) string {
	if p == "" {
		return ""
	}
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return filepath.Clean(real)
	}
	return filepath.Clean(p)
}
