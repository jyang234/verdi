package ritualwitness

import "reflect"

// index judges every changed entry of the main worktree's index (SI-325
// (1)). An entry is within when its path matches a declared stage path,
// when it is the tree difference of a declared HEAD switch or of a commit
// or fast-forward moving @checked-out (HEAD's tree changed at the path and
// the index now holds exactly the new tree's entry), or when it lies in a worktree the ritual added;
// otherwise it is outside, naming the path. It is attributed per effect
// (SI-329 (8′), indexEntryAttributed) — never to BuildTreeWithFile's
// update-index, which writes a scratch index.
func (e *evaluation) index() []Verdict {
	before, after := indexByPath(e.b.Index), indexByPath(e.a.Index)
	treeMove := (e.decl.HeadSwitch && e.switched) || (e.declaresCheckedOutMove() && e.checkedOutMoved())
	var out []Verdict
	for _, p := range unionKeys(before, after) {
		bv, had := before[p]
		av, has := after[p]
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
		treeDifference := e.treeDiffers(p) && e.indexMatchesHeadTree(av, p)
		admitted := e.stagePathAdmits(p) || e.inAddedWorktree(e.repoPathOf(p)) || (treeMove && treeDifference)
		out = append(out, Verdict{Field: "index", Detail: "index entry " + p + " " + verb,
			Status: classify(admitted, e.indexEntryAttributed(e.a.Root, p, treeDifference))})
	}
	return out
}

// indexEntryAttributed is SI-329 (8′) for an index entry at path p in
// worktree wt: an add or a commit whose pathspec matches it; `add -A`,
// which names no paths, per worktree (the disclosed residual); and a
// checkout or fast-forward only for the tree difference it makes in the
// main worktree. A commit without a pathspec changes no index entry.
func (e *evaluation) indexEntryAttributed(wt, p string, treeDifference bool) bool {
	for _, c := range e.at.in(wt, primStage, primCommit, primCheckout, primCheckoutNew, primFastForward) {
		switch c.Kind {
		case primStage, primCommit:
			specs, named := c.pathspecs()
			if (!named && c.Kind == primStage) || (named && pathspecMatches(wt, c, specs, p)) {
				return true
			}
		case primCheckout, primCheckoutNew:
			if wt == e.a.Root && treeDifference && e.switched && e.headSwitchAttributed() {
				return true
			}
		case primFastForward:
			if wt == e.a.Root && treeDifference && e.checkedOutMoved() {
				return true
			}
		}
	}
	return false
}

// treeDiffers reports whether HEAD's tree changed at path in the run.
func (e *evaluation) treeDiffers(path string) bool {
	bv, had := e.b.HeadTree[path]
	av, has := e.a.HeadTree[path]
	return had != has || bv != av
}

// indexMatchesHeadTree reports whether the after index's entries at path
// are exactly HEAD's tree after the run: absent where the tree lacks the
// path, one stage-0 entry equal to the tree's where it has it.
func (e *evaluation) indexMatchesHeadTree(entries []IndexEntry, path string) bool {
	te, has := e.a.HeadTree[path]
	if !has {
		return len(entries) == 0
	}
	return len(entries) == 1 && entries[0].Stage == 0 && entries[0].Mode == te.Mode && entries[0].Object == te.Object
}

// indexByPath groups index entries by path (a conflicted path has one
// entry per stage).
func indexByPath(entries []IndexEntry) map[string][]IndexEntry {
	m := map[string][]IndexEntry{}
	for _, ie := range entries {
		m[ie.Path] = append(m[ie.Path], ie)
	}
	return m
}

// workingTree judges every changed file of the main worktree (SI-325 (2)).
// A change to the operator's pre-existing work — a path dirty, staged, or
// untracked before the ritual — is outside unless the path matches a
// declared stage path or is explained by a declared HEAD switch (HEAD's
// tree changed at the path and the path is clean after the switch, so a
// forced checkout discarding dirty work where both trees agree is not). Any other change is a file write, not a git
// mutation (parent dc-6), reported within with that reason. A file write
// needs no attribution.
func (e *evaluation) workingTree() []Verdict {
	var out []Verdict
	for _, p := range unionKeys(e.b.Files, e.a.Files) {
		bv, had := e.b.Files[p]
		av, has := e.a.Files[p]
		if had == has && bv == av {
			continue
		}
		detail := p + " " + changeVerb(had, has)
		switch {
		case !listed(e.b, p):
			out = append(out, Verdict{Field: "working_tree", Status: Within, Detail: detail + ": a file write, not a git mutation"})
		case e.stagePathAdmits(p):
			out = append(out, Verdict{Field: "working_tree", Status: Within, Detail: detail + ": the operator's pre-existing work, inside a declared stage path"})
		case e.decl.HeadSwitch && e.switched && e.treeDiffers(p) && !listed(e.a, p):
			out = append(out, Verdict{Field: "working_tree", Status: Within, Detail: detail + ": the operator's pre-existing work, explained by a declared HEAD switch"})
		default:
			out = append(out, Verdict{Field: "working_tree", Status: Outside, Detail: detail + ": the operator's pre-existing work"})
		}
	}
	return out
}
