package ritualwitness

import (
	"reflect"
	"sort"
	"strings"

	ws "github.com/jyang234/verdi/internal/writescope"
)

// commits judges every commit object the ritual created, reachable or not,
// in the repository or on the remote (SI-325 (5)), and asserts the
// declared index carry exactly (story ac-1).
//
// A created commit belongs to a worktree the ritual added when that
// worktree's HEAD reaches it after the run, or when the log shows a commit
// made there and no ref or HEAD of the fixture reaches it: it is then
// judged with that worktree (parent dc-7), not against the stage paths,
// and it does not count against no_commit. Every other created commit's
// file list (against its first parent) is judged against the stage paths
// and untracked_may_enter, attributed to a logged commit or commit-tree in
// the fixture; a foreign entry (SI-325 (6)) it records is reported once,
// by the index_carry verdict.
func (e *evaluation) commits() []Verdict {
	var created []string
	for id := range e.a.Commits {
		if _, old := e.b.Commits[id]; !old {
			created = append(created, id)
		}
	}
	sort.Strings(created)
	attributed := e.commitLoggedInFixture()
	var out []Verdict
	var judged []string
	carried := map[string]bool{}
	for _, id := range created {
		if w := e.ownerWorktree(id); w != "" {
			out = append(out, Verdict{Field: "worktrees", Detail: "commit " + short(id) + " made in added worktree " + w,
				Status: classify(e.worktreeAdmitted(w, false), true)})
			continue
		}
		judged = append(judged, id)
		files := e.a.Commits[id].Files
		if len(files) == 0 {
			out = append(out, Verdict{Field: "stage_paths", Detail: "commit " + short(id) + " recorded no file", Status: classify(true, attributed)})
		}
		for _, f := range files {
			switch {
			case e.foreign(f):
				carried[f] = true
			case e.stagePathAdmits(f):
				out = append(out, Verdict{Field: "stage_paths", Detail: "commit " + short(id) + " recorded " + f, Status: classify(true, attributed)})
			case untracked(e.b, f) && e.decl.UntrackedMayEnter:
				out = append(out, Verdict{Field: "untracked_may_enter", Detail: "commit " + short(id) + " recorded previously-untracked " + f, Status: classify(true, attributed)})
			default:
				out = append(out, Verdict{Field: "stage_paths", Detail: "commit " + short(id) + " recorded " + f, Status: Outside})
			}
		}
	}
	return append(out, e.carry(len(judged), carried))
}

// commitLoggedInFixture reports a logged commit or commit-tree that ran
// in the fixture's main worktree or a worktree registered before the run.
func (e *evaluation) commitLoggedInFixture() bool {
	for _, c := range e.at.in("", primCommit, primCommitTree) {
		if !e.addedWT[e.at.worktreeOf(c.Dir)] {
			return true
		}
	}
	return false
}

// ownerWorktree returns the added worktree a created commit belongs to,
// or "".
func (e *evaluation) ownerWorktree(id string) string {
	added := make([]string, 0, len(e.addedWT))
	for w := range e.addedWT {
		added = append(added, w)
	}
	sort.Strings(added)
	for _, w := range added {
		if aw, ok := e.linkedA[w]; ok && e.reachable(id, aw.Head.Commit) {
			return w
		}
	}
	for _, w := range added {
		if e.at.has(w, primCommit, primCommitTree) && !e.reachableFromFixture(id) {
			return w
		}
	}
	return ""
}

// reachableFromFixture reports whether any ref, the main worktree's HEAD,
// or the HEAD of a worktree registered before the run reaches id.
func (e *evaluation) reachableFromFixture(id string) bool {
	starts := []string{e.a.Head.Commit}
	for _, r := range e.a.Refs {
		starts = append(starts, r.Object)
	}
	for p, w := range e.linkedA {
		if _, existed := e.linkedB[p]; existed {
			starts = append(starts, w.Head.Commit)
		}
	}
	for _, s := range starts {
		if e.reachable(id, s) {
			return true
		}
	}
	return false
}

// carry asserts the declared index carry exactly (story ac-1; parent
// dc-3). A refusal is exit 2 with no git mutation remaining and no commit
// created; refused is held to it only when a foreign entry existed to
// refuse (the clean state is where a refusing ritual is seen completing);
// no_commit holds when no commit was judged; scoped holds when the ritual
// neither refused nor carried a foreign entry; carried permits one.
func (e *evaluation) carry(judged int, carried map[string]bool) Verdict {
	refused := e.exit == 2 && len(e.remainingMutations()) == 0 && judged == 0
	var paths []string
	for p := range carried {
		paths = append(paths, p)
	}
	observed := ws.CarryScoped
	switch {
	case refused:
		observed = ws.CarryRefused
	case judged == 0:
		observed = ws.CarryNoCommit
	case len(paths) > 0:
		observed = ws.CarryCarried
	}
	var conforms bool
	switch e.decl.IndexCarry {
	case ws.CarryRefused:
		conforms = refused || !e.hadForeign()
	case ws.CarryNoCommit:
		conforms = judged == 0
	case ws.CarryScoped:
		conforms = !refused && len(paths) == 0
	case ws.CarryCarried:
		conforms = true
	}
	detail := "declares " + string(e.decl.IndexCarry) + "; observed " + string(observed)
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
// (SI-325 (3)): refs, the remote's refs, HEAD, the index, linked
// worktrees, config, and hooks or info. A working-tree file write is not a
// git mutation (parent dc-6), and a commit object no ref reaches is an
// unreferenced object (parent dc-6), so neither counts.
func (e *evaluation) remainingMutations() []string {
	var classes []string
	for _, c := range []struct {
		name    string
		changed bool
	}{
		{"refs", !reflect.DeepEqual(e.b.Refs, e.a.Refs)},
		{"the remote's refs", !reflect.DeepEqual(e.b.RemoteRefs, e.a.RemoteRefs)},
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
