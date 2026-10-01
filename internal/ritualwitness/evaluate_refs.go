package ritualwitness

import (
	"sort"
	"strings"

	ws "github.com/jyang234/verdi/internal/writescope"
)

// commandLog discloses a driver that supplied no command log (SI-325 (8)):
// nothing the log alone could attribute is attributed.
func (e *evaluation) commandLog() []Verdict {
	if e.at.ok {
		return nil
	}
	return []Verdict{{Field: "command_log", Status: Unattributable, Detail: "the driver supplied no git command log"}}
}

// localRefs judges every ref of the repository outside refs/remotes —
// branches, tags, and every other namespace, deletions included (SI-325
// (7)) — against refs_create, refs_move, and refs_delete. Those fields
// name only refs/heads, so a tag or a private ref is outside: no field
// admits it.
func (e *evaluation) localRefs() []Verdict {
	var out []Verdict
	for _, ref := range unionKeys(e.b.Refs, e.a.Refs) {
		if strings.HasPrefix(ref, "refs/remotes/") {
			continue
		}
		bv, had := e.b.Refs[ref]
		av, has := e.a.Refs[ref]
		switch {
		case !had:
			out = append(out, Verdict{Field: "refs_create", Detail: ref + " created",
				Status: classify(e.refAdmitted(e.decl.RefsCreate, ref), e.at.namesCreate(ref))})
		case !has:
			out = append(out, Verdict{Field: "refs_delete", Detail: ref + " deleted",
				Status: classify(e.refAdmitted(e.decl.RefsDelete, ref), e.at.namesDelete(ref))})
		case bv != av:
			attributed := bv.Symref == "" && av.Symref == "" &&
				(e.movedByCommit(ref, bv.Object, av.Object) || e.movedByFastForward(ref, av.Object))
			out = append(out, Verdict{Field: "refs_move", Detail: ref + " moved",
				Status: classify(e.refAdmitted(e.decl.RefsMove, ref), attributed)})
		}
	}
	return out
}

func (e *evaluation) refAdmitted(patterns []ws.RefPattern, ref string) bool {
	for _, p := range patterns {
		if p.Matches(ref, e.checkedOutBefore()) {
			return true
		}
	}
	return false
}

// branchesOf returns the branches that may have been checked out in the
// worktree wt during the run: its HEAD's branch before and after, and
// every branch a logged checkout there switched to.
func (e *evaluation) branchesOf(wt string) map[string]bool {
	m := map[string]bool{}
	if wt == e.a.Root {
		m[e.b.Head.Ref], m[e.a.Head.Ref] = true, true
	}
	if w, ok := e.linkedB[wt]; ok {
		m[w.Head.Ref] = true
	}
	if w, ok := e.linkedA[wt]; ok {
		m[w.Head.Ref] = true
	}
	for _, r := range e.at.checkedOutIn(wt) {
		m[r] = true
	}
	delete(m, "")
	return m
}

// movedByCommit is SI-325 (8)'s commit attribution: a logged `commit` ran
// in a worktree where ref may have been checked out, and the new tip's
// first parent is the old tip.
func (e *evaluation) movedByCommit(ref, oldTip, newTip string) bool {
	if e.firstParent(newTip) != oldTip {
		return false
	}
	for _, c := range e.at.in("", primCommit) {
		if e.branchesOf(e.at.worktreeOf(c.Dir))[ref] {
			return true
		}
	}
	return false
}

// movedByFastForward reports a logged fast-forward to newTip in a
// worktree where ref may have been checked out.
func (e *evaluation) movedByFastForward(ref, newTip string) bool {
	for _, c := range e.at.in("", primFastForward) {
		wt := e.at.worktreeOf(c.Dir)
		if e.branchesOf(wt)[ref] && e.at.fastForwardsTo(wt, newTip) {
			return true
		}
	}
	return false
}

// remoteRefs judges the remote's refs and HEAD and the repository's
// remote-tracking refs against may_push (SI-325 (7), SI-329 (7′), (8′)): a
// remote branch is created or moved within may_push, attributed only for
// the branch a logged push pushed; a remote deletion is never attributed
// to gitx.Push; a remote tag, any other namespace, and the remote's HEAD
// are outside; a remote-tracking ref is within only as the mirror of a
// push the ritual may make, or of a fetch the log shows, and only when it
// equals the remote's ref after the run.
func (e *evaluation) remoteRefs() []Verdict {
	var out []Verdict
	pushed := e.pushedBranches()
	for _, ref := range unionKeys(e.b.Refs, e.a.Refs) {
		if !strings.HasPrefix(ref, "refs/remotes/") {
			continue
		}
		bv, had := e.b.Refs[ref]
		av, has := e.a.Refs[ref]
		if had && has && bv == av {
			continue
		}
		branch, isOrigin := strings.CutPrefix(ref, "refs/remotes/origin/")
		mirror := "refs/heads/" + branch
		pushMirror := isOrigin && e.decl.MayPush && e.remoteChanged(mirror)
		rv, onRemote := e.a.RemoteRefs[mirror]
		equal := has == onRemote && (!has || (av.Symref == "" && av.Object == rv.Object))
		admitted := isOrigin && (pushMirror || e.at.fetched) && equal
		out = append(out, Verdict{Field: "may_push", Detail: ref + " " + changeVerb(had, has),
			Status: classify(admitted, pushMirror && has && pushed[branch])})
	}
	for _, ref := range unionKeys(e.b.RemoteRefs, e.a.RemoteRefs) {
		bv, had := e.b.RemoteRefs[ref]
		av, has := e.a.RemoteRefs[ref]
		if had && has && bv == av {
			continue
		}
		branch, isBranch := strings.CutPrefix(ref, "refs/heads/")
		out = append(out, Verdict{Field: "may_push", Detail: "the remote's " + ref + " " + changeVerb(had, has),
			Status: classify(isBranch && e.decl.MayPush, has && pushed[branch])})
	}
	if e.b.RemoteHead != e.a.RemoteHead {
		out = append(out, Verdict{Field: "may_push", Status: Outside, Detail: "the remote's HEAD changed"})
	}
	return out
}

// pushedBranches returns the branch each logged push pushed (SI-329
// (8′)): gitx.Push pushes HEAD's branch, so each push is credited with
// the branch checked out in its worktree when it ran, replaying the
// worktree's HEAD before the run and the logged checkouts there.
func (e *evaluation) pushedBranches() map[string]bool {
	pushed := map[string]bool{}
	current := map[string]string{}
	for _, c := range e.at.calls {
		wt := e.at.worktreeOf(c.Dir)
		if wt == "" {
			continue
		}
		if _, seen := current[wt]; !seen {
			current[wt] = e.headBefore(wt)
		}
		switch c.Kind {
		case primCheckout, primCheckoutNew:
			current[wt] = c.checkoutTarget()
		case primWorktreeAdd:
			if c.Args[2] == "--detach" {
				current[c.worktreePath()] = ""
			} else {
				current[c.worktreePath()] = "refs/heads/" + c.Args[3]
			}
		case primPush:
			if b, ok := strings.CutPrefix(current[wt], "refs/heads/"); ok {
				pushed[b] = true
			}
		}
	}
	return pushed
}

// headBefore is the branch checked out in worktree wt before the run, or
// "" when its HEAD was detached or it did not exist.
func (e *evaluation) headBefore(wt string) string {
	if wt == e.a.Root {
		return e.checkedOutBefore()
	}
	if w, ok := e.linkedB[wt]; ok && !w.Head.Detached {
		return w.Head.Ref
	}
	return ""
}

// remoteChanged reports whether the remote's ref changed in the run.
func (e *evaluation) remoteChanged(ref string) bool {
	bv, had := e.b.RemoteRefs[ref]
	av, has := e.a.RemoteRefs[ref]
	return had != has || bv != av
}

// remotePushedTo reports whether a push created or moved the remote's ref
// in the run (a deletion is not a branch a push moved).
func (e *evaluation) remotePushedTo(ref string) bool {
	_, has := e.a.RemoteRefs[ref]
	return has && e.remoteChanged(ref)
}

// headSwitch judges the main worktree's HEAD switch (SI-325 (4)) against
// head_switch, attributed only to a logged checkout there naming its
// target (SI-329 (8′)).
func (e *evaluation) headSwitch() []Verdict {
	if !e.switched {
		return nil
	}
	b, a := e.b.Head, e.a.Head
	return []Verdict{{Field: "head_switch", Detail: "HEAD switched from " + headLabel(b) + " to " + headLabel(a),
		Status: classify(e.decl.HeadSwitch, e.headSwitchAttributed())}}
}

// headSwitchAttributed reports a logged checkout in the main worktree
// naming the switch's target: the branch HEAD ended on, or the commit it
// ended detached at.
func (e *evaluation) headSwitchAttributed() bool {
	return e.at.switchesTo(e.a.Root, headTarget(e.a.Head))
}

// headTarget is what a checkout naming h's state names: its branch, or
// the commit it is detached at.
func headTarget(h Head) string {
	if h.Detached {
		return h.Commit
	}
	return h.Ref
}

func unionKeys[V any](a, b map[string]V) []string {
	seen := map[string]bool{}
	var keys []string
	for _, m := range []map[string]V{a, b} {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	sort.Strings(keys)
	return keys
}
