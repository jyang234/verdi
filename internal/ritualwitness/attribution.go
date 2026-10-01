package ritualwitness

import "strings"

// attribution is what the command log can prove (SI-325 (8)): the logged
// calls of gitx's mutating primitives, each canonicalized, and the
// worktrees they may have run in. Every query is false when the driver
// supplied no log.
type attribution struct {
	ok    bool
	calls []loggedCall
	// fetched reports a logged `fetch` in the repository: not a mutating
	// primitive (no gitx function fetches), so it admits a remote-tracking
	// mirror (SI-325 (7)) but attributes nothing.
	fetched bool
	// roots is every worktree of the repository a call may run in: the
	// main worktree, each linked worktree before or after the run, and
	// each worktree a logged `worktree add` named.
	roots []string
}

func newAttribution(log CommandLog, roots []string) attribution {
	at := attribution{ok: log.OK, roots: roots}
	if !log.OK {
		return at
	}
	prims := mutatingPrimitives()
	for _, c := range log.Calls {
		if lc, ok := mutatingCall(c, prims); ok {
			at.calls = append(at.calls, lc)
			if p := lc.worktreePath(); p != "" && lc.Kind == primWorktreeAdd {
				at.roots = append(at.roots, p)
			}
			continue
		}
		if dir, args, ok := splitGlobal(c.Dir, c.Args); ok && args[0] == "fetch" {
			at.fetched = at.fetched || at.worktreeOf(canonicalPath("", dir)) != ""
		}
	}
	return at
}

// worktreeOf returns the worktree a canonical directory lies in — the
// deepest root containing it, since a linked worktree may sit inside the
// main one — or "" when it lies in none.
func (at attribution) worktreeOf(dir string) string {
	best := ""
	for _, r := range at.roots {
		if within(r, dir) && len(r) > len(best) {
			best = r
		}
	}
	return best
}

// in returns the logged calls of the given kinds that ran in a worktree
// of the repository; when wt is not "", only those that ran in wt.
func (at attribution) in(wt string, kinds ...primKind) []loggedCall {
	var out []loggedCall
	for _, c := range at.calls {
		where := at.worktreeOf(c.Dir)
		if where == "" || (wt != "" && where != wt) {
			continue
		}
		for _, k := range kinds {
			if c.Kind == k {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

func (at attribution) has(wt string, kinds ...primKind) bool {
	return len(at.in(wt, kinds...)) > 0
}

// namesCreate reports a logged call creating ref by its full refname.
func (at attribution) namesCreate(ref string) bool {
	for _, c := range at.in("", primCheckoutNew, primUpdateRef) {
		if c.createdRef() == ref {
			return true
		}
	}
	return false
}

// namesDelete reports a logged call deleting ref by its full refname.
func (at attribution) namesDelete(ref string) bool {
	for _, c := range at.in("", primBranchDelete) {
		if c.deletedRef() == ref {
			return true
		}
	}
	return false
}

// namesWorktree reports a logged worktree call of kind naming path.
func (at attribution) namesWorktree(kind primKind, path string) bool {
	for _, c := range at.in("", kind) {
		if c.worktreePath() == path {
			return true
		}
	}
	return false
}

// checkedOutIn returns the branches a logged checkout in wt switched to.
func (at attribution) checkedOutIn(wt string) []string {
	var refs []string
	for _, c := range at.in(wt, primCheckout, primCheckoutNew) {
		if r := c.checkedOutRef(); r != "" {
			refs = append(refs, r)
		}
	}
	return refs
}

// fastForwardsTo reports a logged fast-forward in wt to commit.
func (at attribution) fastForwardsTo(wt, commit string) bool {
	for _, c := range at.in(wt, primFastForward) {
		if strings.EqualFold(c.fastForwardTarget(), commit) {
			return true
		}
	}
	return false
}
