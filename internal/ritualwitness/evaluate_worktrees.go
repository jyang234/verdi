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

// preExistingWorktree judges the changes inside a worktree registered
// before the run (SI-325 (7), SI-329 (7′), (8′)): its HEAD, attributed
// only to a logged checkout there naming its target; each index entry,
// attributed as in the main worktree but for checkouts (whose tree
// difference a linked worktree's sensors do not read); each of its own
// refs, a create attributed to a logged update-ref there naming it; and
// its administrative entry, which no gitx primitive writes.
func (e *evaluation) preExistingWorktree(p string, bw, aw Worktree) []Verdict {
	admitted := e.worktreeAdmitted(p, true)
	var out []Verdict
	if headSwitched(bw.Head, aw.Head) {
		out = append(out, Verdict{Field: "worktrees", Detail: "worktree " + p + ": HEAD changed",
			Status: classify(admitted, e.at.switchesTo(p, headTarget(aw.Head)))})
	}
	before, after := indexByPath(bw.Index), indexByPath(aw.Index)
	for _, path := range unionKeys(before, after) {
		bv, had := before[path]
		av, has := after[path]
		if had && has && reflect.DeepEqual(bv, av) {
			continue
		}
		verb := "changed"
		switch {
		case !had:
			verb = "added"
		case !has:
			verb = "removed"
		}
		out = append(out, Verdict{Field: "worktrees", Detail: "worktree " + p + ": index entry " + path + " " + verb,
			Status: classify(admitted, e.indexEntryAttributed(p, path, false))})
	}
	for _, ref := range unionKeys(bw.Refs, aw.Refs) {
		bv, had := bw.Refs[ref]
		av, has := aw.Refs[ref]
		if had && has && bv == av {
			continue
		}
		attributed := false
		for _, c := range e.at.in(p, primUpdateRef) {
			attributed = attributed || (!had && c.createdRef() == ref)
		}
		out = append(out, Verdict{Field: "worktrees", Detail: "worktree " + p + ": ref " + ref + " " + changeVerb(had, has),
			Status: classify(admitted, attributed)})
	}
	if bw.ID != aw.ID || bw.Locked != aw.Locked || bw.LockReason != aw.LockReason || bw.Present != aw.Present {
		out = append(out, Verdict{Field: "worktrees", Detail: "worktree " + p + ": administrative entry changed",
			Status: classify(admitted, false)})
	}
	return out
}
