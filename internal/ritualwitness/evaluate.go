package ritualwitness

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	ws "github.com/jyang234/verdi/internal/writescope"
)

// Status is one effect's verdict against a Declaration: within it,
// outside it, or unattributable — and an unattributable effect never
// counts as within scope (story ac-1).
type Status string

// The three verdicts.
const (
	Within         Status = "within"
	Outside        Status = "outside"
	Unattributable Status = "unattributable"
)

// Verdict is one observed effect's classification: the field it was
// judged against (a Declaration field, or the effect class no field
// admits: index, working_tree, config, git_dir, exit, command_log), its
// Status, and a Detail naming the effect.
type Verdict struct {
	Field  string
	Status Status
	Detail string
}

func (v Verdict) String() string {
	return fmt.Sprintf("%s: %s (%s)", v.Field, v.Status, v.Detail)
}

// classify is every attributable effect's verdict (doc.go, "Precedence"):
// an effect no declared field admits is outside, whoever made it — the
// state diff alone witnesses it; an admitted effect is within only when
// attributed, and unattributable otherwise.
func classify(admitted, attributed bool) Status {
	if !admitted {
		return Outside
	}
	if !attributed {
		return Unattributable
	}
	return Within
}

// Evaluate compares before and after — Snapshots taken immediately before
// and after a Driver ran one ritual, which exited with exit and logged
// log — against decl, and returns one Verdict per observed effect,
// ordered by effect class and then by detail. It validates decl first,
// and refuses a snapshot pair from different repositories. Evaluate reads
// the filesystem only to canonicalize the log's paths.
func Evaluate(decl ws.Declaration, exit int, before, after Snapshot, log CommandLog) ([]Verdict, error) {
	return EvaluateIn(decl, "", exit, before, after, log)
}

// EvaluateIn is Evaluate for a ritual acting in the checkout at acting,
// which binds @checked-out (ledger SI-348 (4)): "" or the repository's
// root is the main worktree, as Evaluate binds it; any other path names a
// linked worktree, whose branch before the run @checked-out then is. A
// path that is no worktree registered before the run binds no branch, so
// every @checked-out effect is outside: the binding fails closed.
func EvaluateIn(decl ws.Declaration, acting string, exit int, before, after Snapshot, log CommandLog) ([]Verdict, error) {
	if err := decl.Validate(); err != nil {
		return nil, fmt.Errorf("ritualwitness: Evaluate: %w", err)
	}
	if before.Root != after.Root || before.StoreRoot != after.StoreRoot || before.CommonDir != after.CommonDir {
		return nil, errors.New("ritualwitness: Evaluate: the before and after snapshots are of different repositories")
	}
	e := newEvaluation(decl, exit, before, after, log)
	if acting != "" {
		if p := canonicalPath("", acting); p != after.Root {
			e.acting = p
		}
	}
	var out []Verdict
	for _, section := range []func() []Verdict{
		e.commandLog,
		e.localRefs,
		e.remoteRefs,
		e.headSwitch,
		e.index,
		e.workingTree,
		e.worktrees,
		e.commits,
		e.config,
		e.gitDir,
		e.refusal,
	} {
		vs := section()
		sort.SliceStable(vs, func(i, j int) bool {
			if vs[i].Field != vs[j].Field {
				return vs[i].Field < vs[j].Field
			}
			return vs[i].Detail < vs[j].Detail
		})
		out = append(out, vs...)
	}
	return out, nil
}

// evaluation is one Evaluate call's inputs and derived facts.
type evaluation struct {
	decl     ws.Declaration
	exit     int
	b, a     Snapshot
	at       attribution
	linkedB  map[string]Worktree
	linkedA  map[string]Worktree
	addedWT  map[string]bool // linked worktrees the ritual added (SI-329 (5′))
	switched bool            // the main worktree's HEAD switched (SI-325 (4))
	// acting is the canonical path of the linked worktree the ritual acts
	// in, or "" for the main worktree (EvaluateIn; SI-348 (4)).
	acting string
}

func newEvaluation(decl ws.Declaration, exit int, b, a Snapshot, log CommandLog) *evaluation {
	e := &evaluation{decl: decl, exit: exit, b: b, a: a, linkedB: byPath(b.Worktrees), linkedA: byPath(a.Worktrees)}
	roots := []string{a.Root}
	for p := range e.linkedB {
		roots = append(roots, p)
	}
	for p := range e.linkedA {
		roots = append(roots, p)
	}
	e.at = newAttribution(log, roots)
	e.addedWT = map[string]bool{}
	for p := range e.linkedA {
		if _, existed := e.linkedB[p]; !existed {
			e.addedWT[p] = true
		}
	}
	for p := range e.at.addedAndRemoved {
		if _, existed := e.linkedB[p]; !existed {
			e.addedWT[p] = true
		}
	}
	e.switched = headSwitched(b.Head, a.Head)
	return e
}

func byPath(wts []Worktree) map[string]Worktree {
	m := make(map[string]Worktree, len(wts))
	for _, w := range wts {
		m[w.Path] = w
	}
	return m
}

// headSwitched is SI-325 (4): HEAD switches when its branch changes, when
// it moves between attached and detached, or when a detached HEAD moves.
// A commit on the checked-out branch moves that branch, not HEAD.
func headSwitched(b, a Head) bool {
	if b.Ref != a.Ref || b.Detached != a.Detached {
		return true
	}
	return b.Detached && b.Commit != a.Commit
}

// site is where a worktree path is judged against a WorktreePattern.
func (e *evaluation) site(registeredBefore bool) ws.WorktreeSite {
	return ws.WorktreeSite{RepoRoot: e.a.Root, StoreRoot: e.a.StoreRoot, RegisteredBefore: registeredBefore}
}

// worktreeAdmitted reports whether a declared worktree pattern covers the
// worktree at path.
func (e *evaluation) worktreeAdmitted(path string, registeredBefore bool) bool {
	for _, p := range e.decl.Worktrees {
		if p.Matches(path, e.site(registeredBefore)) {
			return true
		}
	}
	return false
}

// stagePathAdmits reports whether a repository-relative path lies in the
// store and matches a declared stage path.
func (e *evaluation) stagePathAdmits(repoPath string) bool {
	rel, ok := storeRelative(e.a.Prefix, repoPath)
	if !ok {
		return false
	}
	for _, p := range e.decl.StagePaths {
		if p.Matches(rel) {
			return true
		}
	}
	return false
}

// staged reports whether s's index differed from HEAD at path.
func staged(s Snapshot, path string) bool {
	for _, st := range s.Status {
		if (st.Path == path || st.OrigPath == path) && st.X != ' ' && st.X != '?' && st.X != '!' {
			return true
		}
	}
	return false
}

// listed reports whether path appears in s's status at all: dirty,
// staged, or untracked.
func listed(s Snapshot, path string) bool {
	for _, st := range s.Status {
		if (st.Path == path || st.OrigPath == path) && st.X != '!' {
			return true
		}
	}
	return false
}

// untracked reports whether path was untracked in s.
func untracked(s Snapshot, path string) bool {
	for _, st := range s.Status {
		if st.Path == path && st.X == '?' {
			return true
		}
	}
	return false
}

// foreign is SI-325 (6): an index entry that differed from HEAD before the
// ritual and lies outside every declared stage path.
func (e *evaluation) foreign(path string) bool {
	return staged(e.b, path) && !e.stagePathAdmits(path)
}

// hadForeign reports whether any foreign entry existed before the ritual.
func (e *evaluation) hadForeign() bool {
	for _, st := range e.b.Status {
		for _, p := range []string{st.Path, st.OrigPath} {
			if p != "" && e.foreign(p) {
				return true
			}
		}
	}
	return false
}

// checkedOutBefore is @checked-out's binding: the branch checked out,
// before the ritual, in the checkout the ritual acts on (the write-scope
// grammar; ledger SI-348 (4)), as a full refname. That is the main
// worktree's branch unless the ritual acts in a linked worktree (acting),
// whose branch it then is. It is "" when that checkout's HEAD was
// detached, or when the acting checkout is no worktree registered before
// the run, so no ref matches @checked-out (fail closed).
func (e *evaluation) checkedOutBefore() string {
	if e.acting == "" {
		return e.mainBranchBefore()
	}
	w, ok := e.linkedB[e.acting]
	if !ok || w.Head.Detached {
		return ""
	}
	return w.Head.Ref
}

// mainBranchBefore is the main worktree's branch before the ritual, as a
// full refname, or "" when HEAD was detached.
func (e *evaluation) mainBranchBefore() string {
	if e.b.Head.Detached {
		return ""
	}
	return e.b.Head.Ref
}

// checkedOutMoved reports a move of @checked-out in the main worktree: the
// ritual acts there, HEAD stayed attached to the branch it started on, and
// that branch moved. It feeds the main worktree's own readings, the tree
// difference such a move makes in its index and the hand-back exception;
// a ritual acting in a linked worktree has its changes there judged as
// that worktree's (preExistingWorktree), so this is false for it.
func (e *evaluation) checkedOutMoved() bool {
	if e.acting != "" {
		return false
	}
	ref := e.mainBranchBefore()
	return ref != "" && !e.switched && e.b.Refs[ref].Object != e.a.Refs[ref].Object
}

// declaresCheckedOutMove reports whether decl's refs_move names
// @checked-out.
func (e *evaluation) declaresCheckedOutMove() bool {
	for _, r := range e.decl.RefsMove {
		if r == ws.RefCheckedOut {
			return true
		}
	}
	return false
}

// reachable reports whether commit is reachable from start in the after
// snapshot's commit graph.
func (e *evaluation) reachable(commit, start string) bool {
	seen := map[string]bool{}
	stack := []string{start}
	for len(stack) > 0 {
		c := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if c == "" || seen[c] {
			continue
		}
		if c == commit {
			return true
		}
		seen[c] = true
		stack = append(stack, e.a.Commits[c].Parents...)
	}
	return false
}

// firstParent returns commit's first parent in the after snapshot, or "".
func (e *evaluation) firstParent(commit string) string {
	if ps := e.a.Commits[commit].Parents; len(ps) > 0 {
		return ps[0]
	}
	return ""
}

func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func headLabel(h Head) string {
	if h.Detached {
		return "detached at " + short(h.Commit)
	}
	return h.Ref
}

// changeVerb names a change from present-before to present-after.
func changeVerb(before, after bool) string {
	switch {
	case !before:
		return "created"
	case !after:
		return "deleted"
	default:
		return "changed"
	}
}

// repoPathOf returns the absolute path of a repository-relative path in
// the main worktree.
func (e *evaluation) repoPathOf(p string) string {
	return filepath.Join(e.a.Root, filepath.FromSlash(p))
}

// inAddedWorktree reports whether an absolute path lies in a worktree the
// ritual added.
func (e *evaluation) inAddedWorktree(abs string) bool {
	for w := range e.addedWT {
		if within(w, abs) {
			return true
		}
	}
	return false
}

func joinSorted(items []string) string {
	sort.Strings(items)
	return strings.Join(items, ", ")
}
