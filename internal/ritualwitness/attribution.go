package ritualwitness

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

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
	// each worktree the log shows both added and removed (SI-329 (5′)).
	roots []string
	// addedAndRemoved is each worktree path a logged add and a logged
	// remove both name.
	addedAndRemoved map[string]bool
}

func newAttribution(log CommandLog, roots []string) attribution {
	at := attribution{ok: log.OK, roots: roots, addedAndRemoved: map[string]bool{}}
	if !log.OK {
		return at
	}
	prims := mutatingPrimitives()
	var fetchDirs []string
	added, removed := map[string]bool{}, map[string]bool{}
	for _, c := range log.Calls {
		if lc, ok := mutatingCall(c, prims); ok {
			at.calls = append(at.calls, lc)
			switch lc.Kind {
			case primWorktreeAdd:
				added[lc.worktreePath()] = true
			case primWorktreeRemove:
				removed[lc.worktreePath()] = true
			}
			continue
		}
		if dir, args, ok := splitGlobal(c.Dir, c.Args); ok && args[0] == "fetch" {
			fetchDirs = append(fetchDirs, canonicalPath("", dir))
		}
	}
	for p := range added {
		if removed[p] {
			at.addedAndRemoved[p] = true
			at.roots = append(at.roots, p)
		}
	}
	for _, d := range fetchDirs {
		at.fetched = at.fetched || at.worktreeOf(d) != ""
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

// switchesTo reports a logged checkout in wt naming target: a full
// refname, or a full commit id for a detached HEAD (SI-329 (8′)).
func (at attribution) switchesTo(wt, target string) bool {
	for _, c := range at.in(wt, primCheckout, primCheckoutNew) {
		if c.checkoutTarget() == target {
			return true
		}
	}
	return false
}

// pathspecMatches reports whether repoPath (relative to root) matches one
// of the call's pathspecs, which git resolves against the call's own
// directory. A pathspec without glob characters matches the path or a
// directory above it; one with them is matched by git's default glob
// rules, where "*" also matches "/".
func pathspecMatches(root string, c loggedCall, specs []string, repoPath string) bool {
	prefix, err := filepath.Rel(root, c.Dir)
	if err != nil || prefix == ".." || strings.HasPrefix(filepath.ToSlash(prefix), "../") {
		return false
	}
	for _, spec := range specs {
		full := path.Clean(path.Join(filepath.ToSlash(prefix), spec))
		if full == "." || repoPath == full || strings.HasPrefix(repoPath, full+"/") {
			return true
		}
		if strings.ContainsAny(spec, "*?[") && globMatches(full, repoPath) {
			return true
		}
	}
	return false
}

// globMatches matches name against git's default pathspec glob: "*" and
// "?" match any characters including "/", and "[...]" a class.
func globMatches(pattern, name string) bool {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch c := pattern[i]; c {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '[':
			j := strings.IndexByte(pattern[i:], ']')
			if j < 0 {
				b.WriteString(regexp.QuoteMeta("["))
				continue
			}
			b.WriteString("[" + strings.ReplaceAll(pattern[i+1:i+j], `\`, `\\`) + "]")
			i += j
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	re, err := regexp.Compile(b.String() + "(/.*)?$")
	return err == nil && re.MatchString(name)
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
