// Package branchcut holds the branch-cut unwind sequence close and recover
// both need (spec/readiness-recovery-v2 ac-9: "with the sequence close
// already uses"): re-prove a just-cut branch still points at its cut point,
// switch back to the branch it was cut from, and delete it — the single
// implementation both callers share so neither can drift.
package branchcut

import (
	"context"
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/gitx"
)

// Outcome is what Unwind did, for callers that need more than stderr.
type Outcome int

const (
	Unwound           Outcome = iota // switched back and deleted the branch
	LeftUninspectable                // RevParse failed
	LeftAheadOfCut                   // tip != cutPoint: nothing discarded
	LeftSwitchFailed                 // CheckoutExisting failed
	LeftDeleteFailed                 // switched back, DeleteBranch failed
)

// String names o for logging/disclosure, self-naming an out-of-set value
// rather than printing a bare int (the package's usual shape, e.g.
// internal/execworkspace's Outcome.String).
func (o Outcome) String() string {
	switch o {
	case Unwound:
		return "unwound"
	case LeftUninspectable:
		return "left-uninspectable"
	case LeftAheadOfCut:
		return "left-ahead-of-cut"
	case LeftSwitchFailed:
		return "left-switch-failed"
	case LeftDeleteFailed:
		return "left-delete-failed"
	default:
		return fmt.Sprintf("branchcut.Outcome(%d)", int(o))
	}
}

// Unwind is verdi close's branch-cut unwind, moved here verbatim so
// close and recover run ONE sequence (spec/readiness-recovery-v2 ac-9:
// "with the sequence close already uses"): re-prove branch still points
// at cutPoint, switch back to originalBranch (or cutPoint itself when
// originalBranch is "", the detached-HEAD case), then `git branch -d`.
// It NEVER discards committed work and NEVER force-deletes; every
// giving-up path is disclosed on stderr with prefix "<verb>: ".
func Unwind(ctx context.Context, root, originalBranch, branch, cutPoint, verb string, stderr io.Writer) Outcome {
	tip, err := gitx.RevParse(ctx, root, branch)
	if err != nil {
		fmt.Fprintf(stderr, "%s: left %s in place: could not inspect it to unwind the branch cut (%v); switch back and delete it before retrying\n", verb, branch, err)
		return LeftUninspectable
	}
	if tip != cutPoint {
		fmt.Fprintf(stderr, "%s: left %s in place: it carries commit(s) beyond its cut point %s and nothing was discarded; remove it manually if unneeded before retrying\n", verb, branch, cutPoint)
		return LeftAheadOfCut
	}
	restore := originalBranch
	if restore == "" {
		// The cut ran from a detached HEAD (CurrentBranch is "" there, not
		// an error): return to the cut commit itself rather than a branch
		// name.
		restore = cutPoint
	}
	if err := gitx.CheckoutExisting(ctx, root, restore); err != nil {
		fmt.Fprintf(stderr, "%s: left %s in place: could not switch back to %s to unwind the branch cut (%v); remove it manually before retrying\n", verb, branch, restore, err)
		return LeftSwitchFailed
	}
	if err := gitx.DeleteBranch(ctx, root, branch); err != nil {
		fmt.Fprintf(stderr, "%s: switched back to %s but left %s in place: %v; remove it manually before retrying\n", verb, restore, branch, err)
		return LeftDeleteFailed
	}
	return Unwound
}
