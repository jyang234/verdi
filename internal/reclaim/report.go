package reclaim

import "fmt"

// Kind is dc-4's own closed set of report-line kinds one reclaim unit can
// resolve to across BOTH invocation modes — mirroring
// internal/wtmanager.Decision's role as GC's own total, named outcome map
// for its sibling managed-worktree slice.
//
// KindEligible appears only in a dry-run report (the plan, not yet acted
// on). KindReclaimed, KindRefused, and KindPartial appear only in an
// --apply report (the SAME plan, acted on) — KindRefused and KindPartial
// are both apply-time SECOND-GUARD outcomes dc-2/AC-2 require (git itself,
// not this predicate, refused the mutation): KindRefused names a unit
// where the ONE step attempted (worktree-remove for a worktree+branch
// unit; branch-delete for a branch-only unit) was refused and nothing was
// removed; KindPartial names a worktree+branch unit whose worktree WAS
// removed before its paired branch-delete then failed — dc-4's own
// distinct outcome, never folded into a generic failure. KindKept appears,
// byte-identically, in both dry-run and --apply output — a plan-time
// exclusion (AC-1) is never re-decided by --apply (dc-1).
type Kind int

const (
	KindEligible  Kind = iota // dry-run only: AC-1-eligible, not yet acted on
	KindKept                  // AC-1 predicate-time exclusion — both modes, byte-identical
	KindReclaimed             // --apply only: fully removed (worktree, if any, then branch)
	KindRefused               // --apply only: git's own second guard refused the ONLY step attempted; nothing removed
	KindPartial               // --apply only: worktree removed, but the paired branch delete then failed
)

// Row is one reclaim UNIT's disclosed report line (dc-4: never two lines
// for one unit) — either its plan-time classification (KindEligible /
// KindKept) or, once --apply has acted on an eligible item, its execution
// outcome (KindReclaimed / KindRefused / KindPartial).
type Row struct {
	Unit Unit
	Kind Kind
	// Reason is meaningful only for Kind == KindKept.
	Reason KeptReason
	// Detail carries residue's own Reason text (KindKept, unresolved-state
	// only) or git's own refusal text (KindRefused, KindPartial).
	Detail string
	// Tip is the branch's pre-delete tip commit, meaningful only for
	// Kind == KindReclaimed.
	Tip string
}

// DryRunRows renders p exactly as computed, untouched: one Row per
// PlanItem, Kind KindEligible or KindKept. It takes no context.Context and
// no root — a TYPE-level guarantee that a dry run cannot mutate anything,
// not merely a tested behavior (AC-2: "--reclaim-unmanaged alone ...
// performs no git-mutating call").
func (p Plan) DryRunRows() []Row {
	rows := make([]Row, 0, len(p.Items))
	for _, item := range p.Items {
		if item.Eligible {
			rows = append(rows, Row{Kind: KindEligible, Unit: item.Unit})
			continue
		}
		rows = append(rows, Row{Kind: KindKept, Unit: item.Unit, Reason: item.Reason, Detail: item.Detail})
	}
	return rows
}

// Line renders r as gc's disclosed report line (dc-4), mirroring
// internal/wtmanager.Result.Line()'s own shape: a distinct, named template
// per Kind, one line per reclaim unit, so a human reading the output can
// always tell why a unit was or was not touched.
//
// KindRefused and KindPartial both name a second-guard git refusal, but
// deliberately NOT with the same leading word dc-2's prose uses loosely
// for both ("kept via git's own refusal") — dc-4's own bullet list defines
// wording only for KindReclaimed (x2), KindKept, and KindPartial; it names
// no literal wording for a plan-time-eligible item git itself then refused
// with nothing removed at all, so this rendering distinguishes it from a
// plan-time KindKept exclusion under its own "refused:" lead — an
// implementation choice, disclosed here, not a silent one.
func (r Row) Line() string {
	switch r.Kind {
	case KindEligible:
		return "eligible: " + r.unitDesc()
	case KindKept:
		return "kept: " + r.unitDesc() + " — " + r.reasonText()
	case KindReclaimed:
		return "reclaimed: " + r.unitDesc() + " (tip " + r.Tip + ")"
	case KindRefused:
		return "refused: " + r.unitDesc() + " — " + r.refusalStage() + " refused: " + r.Detail
	case KindPartial:
		return "partial: worktree " + r.Unit.WorktreePath + " removed, branch " + r.Unit.Branch + " NOT deleted — " + r.Detail
	default:
		return fmt.Sprintf("reclaim: internal error: unhandled row kind %d for %s", int(r.Kind), r.unitDesc())
	}
}

// unitDesc names r's unit the SAME way regardless of Kind — a worktree+
// branch unit as "worktree <path> + branch <name>", a branch-only unit as
// "branch <name>" — so a reader scanning mixed output recognizes a given
// unit's own identity across an eligible/kept/reclaimed/refused/partial
// line without re-parsing a different shape each time.
func (r Row) unitDesc() string {
	if !r.Unit.HasWorktree() {
		return "branch " + r.Unit.Branch
	}
	if r.Unit.Branch == "" {
		// A detached worktree row (KeptDetached is the only reason this
		// ever renders, by construction — classifyWorktreeRow excludes an
		// empty-Branch row before it could ever reach Eligible) — named
		// without a dangling "+ branch " for a name that does not exist,
		// never a guessed branch (dc-2/AC-1: "for a detached HEAD, its
		// commit" is the only name a caller has).
		return "worktree " + r.Unit.WorktreePath
	}
	return "worktree " + r.Unit.WorktreePath + " + branch " + r.Unit.Branch
}

// reasonText renders r's KeptReason, appending residue's own Reason detail
// text for the one reason that carries one (unresolved-state — dc-2:
// "naming residue's own Reason").
func (r Row) reasonText() string {
	if r.Reason == KeptUnresolvedState && r.Detail != "" {
		return r.Reason.String() + " (" + r.Detail + ")"
	}
	return r.Reason.String()
}

// refusalStage names which of the two mutating steps was refused — the
// only step KindRefused ever attempted (dc-3: worktree-remove before
// branch-delete; a worktree-remove refusal skips the branch-delete step
// entirely, so KindRefused never means "both failed").
func (r Row) refusalStage() string {
	if r.Unit.HasWorktree() {
		return "worktree removal"
	}
	return "branch deletion"
}
