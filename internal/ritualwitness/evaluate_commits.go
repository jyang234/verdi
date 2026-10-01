package ritualwitness

import (
	"reflect"
	"sort"
	"strings"

	ws "github.com/jyang234/verdi/internal/writescope"
)

// commits judges every commit object the ritual created, reachable or not,
// in the repository or on the remote (SI-325 (5), SI-329 (5′)), every
// branch move by its net tree difference, and asserts the declared index
// carry exactly (story ac-1).
//
// A created commit owned by a worktree the ritual added (ownerWorktree) is
// judged with that worktree (parent dc-7), not against the stage paths,
// and does not count against no_commit. Every other created commit's file
// list (against its first parent) is judged against the stage paths and
// untracked_may_enter. The foreign-entry check (SI-325 (6)) runs on every
// created commit, owned or not, and is reported once, by the index_carry
// verdict.
func (e *evaluation) commits() []Verdict {
	var created []string
	for id := range e.a.Commits {
		if _, old := e.b.Commits[id]; !old {
			created = append(created, id)
		}
	}
	sort.Strings(created)
	var out []Verdict
	judged := 0
	carried := map[string]bool{}
	for _, id := range created {
		files := e.a.Commits[id].Files
		for _, f := range files {
			if e.foreign(f) {
				carried[f] = true
			}
		}
		if w := e.ownerWorktree(id); w != "" {
			out = append(out, Verdict{Field: "worktrees", Detail: "commit " + short(id) + " made in added worktree " + w,
				Status: classify(e.worktreeAdmitted(w, false), true)})
			continue
		}
		judged++
		if len(files) == 0 {
			out = append(out, Verdict{Field: "stage_paths", Detail: "commit " + short(id) + " recorded no file", Status: classify(true, e.commitLoggedInFixture())})
		}
		for _, f := range files {
			detail := "commit " + short(id) + " recorded " + f
			switch {
			case e.foreign(f):
			case e.stagePathAdmits(f):
				out = append(out, Verdict{Field: "stage_paths", Detail: detail, Status: classify(true, e.commitPathAttributed(f))})
			case untracked(e.b, f) && e.decl.UntrackedMayEnter:
				out = append(out, Verdict{Field: "untracked_may_enter", Detail: "commit " + short(id) + " recorded previously-untracked " + f,
					Status: classify(true, e.commitPathAttributed(f))})
			default:
				out = append(out, Verdict{Field: "stage_paths", Detail: detail, Status: Outside})
			}
		}
	}
	out = append(out, e.branchMoves(created)...)
	return append(out, e.carry(len(created), judged, carried))
}

// commitLoggedInFixture reports a logged commit or commit-tree that ran
// in the fixture: its main worktree or a worktree registered before the
// run.
func (e *evaluation) commitLoggedInFixture() bool {
	for _, c := range e.at.in("", primCommit, primCommitTree) {
		if !e.addedWT[e.at.worktreeOf(c.Dir)] {
			return true
		}
	}
	return false
}

// commitPathAttributed is SI-329 (8′) for a path a created commit
// recorded: a logged commit in the fixture whose pathspec matches it, or
// one that names no paths (commit without "--", commit-tree), whose
// attribution stays per worktree — the disclosed residual.
func (e *evaluation) commitPathAttributed(f string) bool {
	for _, c := range e.at.in("", primCommit, primCommitTree) {
		wt := e.at.worktreeOf(c.Dir)
		if e.addedWT[wt] {
			continue
		}
		specs, named := c.pathspecs()
		if !named || pathspecMatches(wt, c, specs, f) {
			return true
		}
	}
	return false
}

// ownerWorktree returns the added worktree a created commit belongs to
// (SI-329 (5′)), or "". The commit must have been made after the
// worktree was added — the commit the worktree started at does not reach
// it — and either (i) the worktree's HEAD after the run reaches it and no
// fixture ref or HEAD does, except through a logged fast-forward of
// @checked-out to a commit the worktree reaches (the context-execution
// hand-back), or (ii) the worktree is gone after the run, the log shows a
// commit made in it, and none made in the fixture.
func (e *evaluation) ownerWorktree(id string) string {
	added := make([]string, 0, len(e.addedWT))
	for w := range e.addedWT {
		added = append(added, w)
	}
	sort.Strings(added)
	for _, w := range added {
		start := e.worktreeStart(w)
		if start == "" || e.reachable(id, start) {
			continue
		}
		if aw, present := e.linkedA[w]; present {
			if e.reachable(id, aw.Head.Commit) && !e.fixtureReaches(id, e.handbackTips(aw)) {
				return w
			}
			continue
		}
		if e.at.has(w, primCommit, primCommitTree) && !e.commitLoggedInFixture() {
			return w
		}
	}
	return ""
}

// worktreeStart is the commit an added worktree started at: the first
// entry of its HEAD reflog, or else the commit (or branch, as it stood
// before the run) its logged add named; "" when neither is known.
func (e *evaluation) worktreeStart(w string) string {
	if aw, ok := e.linkedA[w]; ok && aw.Start != "" {
		return aw.Start
	}
	for _, c := range e.at.in("", primWorktreeAdd) {
		if c.worktreePath() != w {
			continue
		}
		at := c.addedCommit()
		if isObjectID(at) {
			return at
		}
		if r, ok := e.b.Refs["refs/heads/"+at]; ok {
			return r.Object
		}
	}
	return ""
}

// handbackTips returns the commit a logged fast-forward moved
// @checked-out to, when the added worktree aw reaches it.
func (e *evaluation) handbackTips(aw Worktree) map[string]bool {
	tips := map[string]bool{}
	if !e.checkedOutMoved() {
		return tips
	}
	tip := e.a.Refs[e.checkedOutBefore()].Object
	if e.at.fastForwardsTo(e.a.Root, tip) && e.reachable(tip, aw.Head.Commit) {
		tips[tip] = true
	}
	return tips
}

// fixtureReaches reports whether any ref of the repository or the remote,
// the main worktree's HEAD, or the HEAD of a worktree registered before
// the run reaches id, starting from no commit in except.
func (e *evaluation) fixtureReaches(id string, except map[string]bool) bool {
	starts := []string{e.a.Head.Commit}
	for _, refs := range []map[string]Ref{e.a.Refs, e.a.RemoteRefs} {
		for _, r := range refs {
			starts = append(starts, r.Object)
		}
	}
	for p, w := range e.linkedA {
		if _, existed := e.linkedB[p]; existed {
			starts = append(starts, w.Head.Commit)
		}
	}
	for _, s := range starts {
		if !except[s] && e.reachable(id, s) {
			return true
		}
	}
	return false
}

// branchMoves judges every local branch move by its net tree difference,
// old tip to new tip (SI-329 (5′)): a path the move changes that no
// created commit it brings in recorded — a merge whose first parent is
// not the old tip, or a move onto commits that already existed — is
// judged against the stage paths, attributed as the move is.
func (e *evaluation) branchMoves(created []string) []Verdict {
	var out []Verdict
	for _, ref := range unionKeys(e.b.Refs, e.a.Refs) {
		bv, had := e.b.Refs[ref]
		av, has := e.a.Refs[ref]
		if !strings.HasPrefix(ref, "refs/heads/") || !had || !has || bv.Object == av.Object {
			continue
		}
		covered := map[string]bool{}
		for _, id := range created {
			if e.reachable(id, av.Object) && !e.reachable(id, bv.Object) {
				for _, f := range e.a.Commits[id].Files {
					covered[f] = true
				}
			}
		}
		attributed := e.movedByCommit(ref, bv.Object, av.Object) || e.movedByFastForward(ref, av.Object)
		for _, p := range treeDiff(e.b.Trees[bv.Object], e.a.Trees[av.Object]) {
			if !covered[p] {
				out = append(out, Verdict{Field: "stage_paths", Detail: ref + " moved onto " + p + ", which no created commit recorded",
					Status: classify(e.stagePathAdmits(p), attributed)})
			}
		}
	}
	return out
}

// treeDiff returns the paths whose entries differ between two trees,
// sorted.
func treeDiff(a, b map[string]TreeEntry) []string {
	var paths []string
	for _, p := range unionKeys(a, b) {
		av, had := a[p]
		bv, has := b[p]
		if had != has || av != bv {
			paths = append(paths, p)
		}
	}
	return paths
}

// carry asserts the declared index carry exactly (story ac-1; parent
// dc-3; SI-329 (3′)), with two refusal predicates. A strict refusal is
// exit 2 with no git mutation remaining and no commit object created;
// refused conforms only to it, and is held to it only when a foreign entry
// existed to refuse. A refusal is exit 2 with nothing remaining, whatever
// unreferenced objects it left (parent dc-6); scoped is violated by it,
// and by carrying a foreign entry. no_commit holds when no commit was
// judged; carried permits a foreign entry.
func (e *evaluation) carry(created, judged int, carried map[string]bool) Verdict {
	refusal := e.exit == 2 && len(e.remainingMutations()) == 0
	strict := refusal && created == 0
	var paths []string
	for p := range carried {
		paths = append(paths, p)
	}
	observed := string(ws.CarryScoped)
	switch {
	case strict:
		observed = string(ws.CarryRefused)
	case refusal:
		observed = "refused, leaving an unreferenced commit"
	case judged == 0:
		observed = string(ws.CarryNoCommit)
	case len(paths) > 0:
		observed = string(ws.CarryCarried)
	}
	var conforms bool
	switch e.decl.IndexCarry {
	case ws.CarryRefused:
		conforms = strict || !e.hadForeign()
	case ws.CarryNoCommit:
		conforms = judged == 0
	case ws.CarryScoped:
		conforms = !refusal && len(paths) == 0
	case ws.CarryCarried:
		conforms = true
	}
	detail := "declares " + string(e.decl.IndexCarry) + "; observed " + observed
	if len(paths) > 0 {
		detail += "; carried foreign " + joinSorted(paths)
	}
	status := Outside
	if conforms {
		status = Within
	}
	return Verdict{Field: "index_carry", Status: status, Detail: detail}
}

// remainingMutations names each class of git state the run left changed
// (SI-325 (3)): refs, the remote's refs and HEAD, HEAD, the index, linked
// worktrees, config, and hooks or info. A working-tree file write is not
// a git mutation (parent dc-6), and a commit object no ref reaches is an
// unreferenced object (parent dc-6), so neither counts.
func (e *evaluation) remainingMutations() []string {
	var classes []string
	for _, c := range []struct {
		name    string
		changed bool
	}{
		{"refs", !reflect.DeepEqual(e.b.Refs, e.a.Refs)},
		{"the remote's refs", !reflect.DeepEqual(e.b.RemoteRefs, e.a.RemoteRefs) || e.b.RemoteHead != e.a.RemoteHead},
		{"HEAD", e.b.Head != e.a.Head},
		{"index", !reflect.DeepEqual(e.b.Index, e.a.Index)},
		{"linked worktrees", !reflect.DeepEqual(e.b.Worktrees, e.a.Worktrees)},
		{"config", !reflect.DeepEqual(e.b.Config, e.a.Config)},
		{"hooks or info", !reflect.DeepEqual(e.b.GitFiles, e.a.GitFiles)},
	} {
		if c.changed {
			classes = append(classes, c.name)
		}
	}
	return classes
}

// refusal is SI-325 (3): exit 2 with any git mutation remaining after the
// run is outside for every declaration, since a refusal is a guard before
// any mutation.
func (e *evaluation) refusal() []Verdict {
	if e.exit != 2 {
		return nil
	}
	remaining := e.remainingMutations()
	if len(remaining) == 0 {
		return nil
	}
	return []Verdict{{Field: "exit", Status: Outside, Detail: "exit 2 (a refusal) with git mutations remaining: " + strings.Join(remaining, ", ")}}
}
