// Story-page ladder state (V1-P8; 05 §Lenses story lens, §Verdi-dex:
// "story pages carry ladder state — spec-stale and pending-supersession
// flags, §3b of the concept — read-only, computed identically to the
// workbench story lens"). The badges render the SAME computations the
// gates enforce (evidence.SpecStale via decisionsweep.ScanSpecStale;
// evidence.PendingSupersession over open MRs) — no dex-private logic
// path — and every non-rendered badge is either proven-unflagged or
// explicitly disclosed unproven, never a silent pass.
package dex

import (
	"fmt"
	"strings"
)

// ladderState is what a story page renders: header badges plus the
// metadata-card rows that disclose why (or why the answer is unknown).
type ladderState struct {
	Badges []string // subset of {"spec-stale", "pending-supersession"}, in ladder order
	Rows   []metaRow
}

// storyLadder computes p's ladder state from the build-wide lens data.
// Non-story pages get a zero state.
func storyLadder(lens *lensData, p *artifactPage) ladderState {
	var state ladderState
	if !isStoryPage(p) {
		return state
	}

	if stale, ok := lens.staleByRef[p.Entry.Ref]; ok && stale.Flagged {
		state.Badges = append(state.Badges, "spec-stale")
		detail := fmt.Sprintf("accepted-deviation count %d", stale.AcceptedDeviationCount)
		if len(stale.OwnTextFindingIDs) > 0 {
			detail = fmt.Sprintf("own-text finding(s) %s; %s", strings.Join(stale.OwnTextFindingIDs, ", "), detail)
		}
		state.Rows = append(state.Rows, metaRow{Label: "Spec-stale", Value: detail + " (03 §The amendment ladder)"})
	}

	pending, ok := lens.pendingByRef[p.Entry.Ref]
	if !ok {
		return state // implements no feature: nothing to prove, nothing to disclose
	}
	switch {
	case pending.Unproven:
		// Three-valued honesty (CLAUDE.md; the closure gate's own nil-forge
		// NOTICE): the input was unavailable, so the flag is neither raised
		// nor cleared — disclosed, never silently absent.
		state.Rows = append(state.Rows, metaRow{Label: "Pending supersession", Value: "unproven — open supersession MRs could not be enumerated at build time (no forge); not read as 'no pending MRs'"})
	case pending.Result.Flagged:
		state.Badges = append(state.Badges, "pending-supersession")
		state.Rows = append(state.Rows, metaRow{Label: "Pending supersession", Value: fmt.Sprintf("open supersession MR(s) %s touch object(s) %s (03 §The amendment ladder)", strings.Join(pending.Result.MRIDs, ", "), strings.Join(pending.Result.Touched, ", "))})
	}
	return state
}
