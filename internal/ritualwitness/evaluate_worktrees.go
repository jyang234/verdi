package ritualwitness

import "reflect"

// worktrees judges the linked worktrees (SI-325 (7); parent dc-7). A
// worktree added or removed is judged against the worktrees field, the
// administrative entry it brings or takes belonging to that add or remove;
// @temp matches only a worktree not registered before the run. A change
// inside a worktree registered before the run — its HEAD, its index, or
// its administrative entry (id, lock) — is outside unless a declared
// worktree pattern covers that worktree. Each is attributed to a logged
// gitx call naming the worktree's canonical path or running inside it; no
// gitx primitive writes an administrative entry directly, so an admitted
// change to one is unattributable.
func (e *evaluation) worktrees() []Verdict {
	var out []Verdict
	for _, p := range unionKeys(e.linkedB, e.linkedA) {
		bw, had := e.linkedB[p]
		aw, has := e.linkedA[p]
		switch {
		case !had:
			out = append(out, Verdict{Field: "worktrees", Detail: "worktree " + p + " added",
				Status: classify(e.worktreeAdmitted(p, false), e.at.namesWorktree(primWorktreeAdd, p))})
		case !has:
			out = append(out, Verdict{Field: "worktrees", Detail: "worktree " + p + " removed",
				Status: classify(e.worktreeAdmitted(p, true), e.at.namesWorktree(primWorktreeRemove, p))})
		default:
			out = append(out, e.preExistingWorktree(p, bw, aw)...)
		}
	}
	return out
}

func (e *evaluation) preExistingWorktree(p string, bw, aw Worktree) []Verdict {
	admitted := e.worktreeAdmitted(p, true)
	var out []Verdict
	if headSwitched(bw.Head, aw.Head) {
		out = append(out, Verdict{Field: "worktrees", Detail: "worktree " + p + ": HEAD changed",
			Status: classify(admitted, e.at.has(p, primCheckout, primCheckoutNew, primCommit))})
	}
	if !reflect.DeepEqual(bw.Index, aw.Index) {
		out = append(out, Verdict{Field: "worktrees", Detail: "worktree " + p + ": index changed",
			Status: classify(admitted, e.at.has(p, primStage, primCommit, primCheckout, primCheckoutNew, primFastForward))})
	}
	if bw.ID != aw.ID || bw.Locked != aw.Locked || bw.LockReason != aw.LockReason || bw.Present != aw.Present {
		out = append(out, Verdict{Field: "worktrees", Detail: "worktree " + p + ": administrative entry changed",
			Status: classify(admitted, false)})
	}
	return out
}
