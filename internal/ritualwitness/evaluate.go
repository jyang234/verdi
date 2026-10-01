package ritualwitness

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/gitx"
	ws "github.com/jyang234/verdi/internal/writescope"
)

// Status is one effect's verdict against a Declaration (parent ac-1): it
// lies within the declaration, outside it, or the sensors cannot attribute
// it to anything the ritual itself did — and an unattributable effect
// never counts as within scope (obligation/ritual-effect-witness--ac-1).
type Status string

const (
	Within         Status = "within"
	Outside        Status = "outside"
	Unattributable Status = "unattributable"
)

// Verdict is one observed effect's classification: the Declaration field it
// was checked against, its Status, and a legible Detail naming the effect
// (RED/GREEN and failure output read Detail directly, never a field+status
// pair alone — the obligation requires failures to "name the effect").
type Verdict struct {
	Field  string
	Status Status
	Detail string
}

func (v Verdict) String() string {
	return fmt.Sprintf("%s: %s (%s)", v.Field, v.Status, v.Detail)
}

// evalFuncs is the small set of decision functions Evaluate's own
// correctness rests on. Production code always gets defaultEvalFuncs();
// harness_test.go substitutes one function at a time to prove each is
// load-bearing (the four mutants spec/ritual-effect-witness ac-1's GREEN
// list requires), mirroring how internal/writescope's own witness_test.go
// mutates registry DATA to prove Check catches it — here the thing under
// test is behavior, so the swapped unit is a function instead.
type evalFuncs struct {
	// classify turns (attributable, within) into a Status. The real
	// implementation never reports Within when !attributable (mutant:
	// "treat unattributable as within").
	classify func(attributable, within bool) Status
	// matchRef is writescope.RefPattern.Matches, indirected so a test can
	// substitute a version that ignores checkedOutBefore (mutant: "match
	// RefCheckedOut against any branch").
	matchRef func(p ws.RefPattern, ref, checkedOutBefore string) bool
	// noMutation reports whether before and after are identical across
	// every sensor. The real implementation compares all of them,
	// including the index and working-tree sensors (mutant: "skip the
	// index sensor" substitutes a version that ignores Index/Working/
	// Hashes — and, separately, "let refused pass with a mutation"
	// substitutes a version that always returns true, never checking any
	// sensor at all).
	noMutation func(before, after Snapshot) bool
}

func defaultEvalFuncs() evalFuncs {
	return evalFuncs{
		classify:   classify,
		matchRef:   func(p ws.RefPattern, ref, checkedOutBefore string) bool { return p.Matches(ref, checkedOutBefore) },
		noMutation: snapshotsEqual,
	}
}

// classify is the real, production decision: an effect the log cannot
// attribute is always Unattributable, regardless of whether it happens to
// fall inside a declared pattern.
func classify(attributable, within bool) Status {
	if !attributable {
		return Unattributable
	}
	if within {
		return Within
	}
	return Outside
}

// Evaluate compares before and after Snapshots — taken immediately before
// and after a Driver ran one ritual with exit classification exit and
// command log log — against decl, and returns one Verdict per observed
// effect. storeRoot relates a worktree's absolute path to a
// writescope.WorktreePattern's store-relative directory form (parent dc-7).
// No effect is reported for a sensor dimension that did not change: ac-1
// classifies OBSERVED effects, and a ritual that touched nothing in a given
// dimension left nothing there to classify.
func Evaluate(decl ws.Declaration, exit int, before, after Snapshot, storeRoot string, log CommandLog) []Verdict {
	return evaluateWith(decl, exit, before, after, storeRoot, log, defaultEvalFuncs())
}

func evaluateWith(decl ws.Declaration, exit int, before, after Snapshot, storeRoot string, log CommandLog, fns evalFuncs) []Verdict {
	checkedOutBefore := before.Head.Branch
	var out []Verdict
	out = append(out, evalLocalRefs(decl, before, after, checkedOutBefore, log, fns)...)
	out = append(out, evalRemoteRefs(decl, before, after, log, fns)...)
	out = append(out, evalHeadSwitch(decl, before, after)...)
	out = append(out, evalWorktrees(decl, before, after, storeRoot, log, fns)...)
	out = append(out, evalCommitsAndCarry(decl, exit, before, after, fns)...)
	return out
}

// evalLocalRefs classifies every refs/heads/* create, move, and delete
// between before and after against decl's RefsCreate/RefsMove/RefsDelete.
//
// Attribution: a create or delete is attributable only when the log
// mentions the ref's short name — the token every ref-naming gitx call
// (checkout -b, branch -d, update-ref, branch) carries. A move is
// attributable the same way, OR when the moved ref is the branch checked
// out when the ritual finished and the log shows any commit-shaped call:
// `git commit` moves the checked-out branch without ever naming it in argv
// (parent dc-2's "a mutation made outside gitx" concern is what this
// attribution step exists to catch — not an ordinary commit).
func evalLocalRefs(decl ws.Declaration, before, after Snapshot, checkedOutBefore string, log CommandLog, fns evalFuncs) []Verdict {
	var out []Verdict
	for _, ref := range sortedKeys(after.LocalRefs) {
		sha := after.LocalRefs[ref]
		beforeSHA, existed := before.LocalRefs[ref]
		if !existed {
			out = append(out, refVerdict("refs_create", ref, log.Mentions(shortRef(ref)), decl.RefsCreate, checkedOutBefore, fns,
				fmt.Sprintf("ref %s created at %s", ref, short(sha))))
			continue
		}
		if beforeSHA != sha {
			attributable := log.Mentions(shortRef(ref)) || (ref == "refs/heads/"+after.Head.Branch && log.HasVerb("commit"))
			out = append(out, refVerdict("refs_move", ref, attributable, decl.RefsMove, checkedOutBefore, fns,
				fmt.Sprintf("ref %s moved from %s to %s", ref, short(beforeSHA), short(sha))))
		}
	}
	for _, ref := range sortedKeys(before.LocalRefs) {
		if _, stillThere := after.LocalRefs[ref]; stillThere {
			continue
		}
		out = append(out, refVerdict("refs_delete", ref, log.Mentions(shortRef(ref)), decl.RefsDelete, checkedOutBefore, fns,
			fmt.Sprintf("ref %s deleted (was %s)", ref, short(before.LocalRefs[ref]))))
	}
	return out
}

func refVerdict(field, ref string, attributable bool, patterns []ws.RefPattern, checkedOutBefore string, fns evalFuncs, detail string) Verdict {
	within := false
	for _, p := range patterns {
		if fns.matchRef(p, ref, checkedOutBefore) {
			within = true
			break
		}
	}
	return Verdict{Field: field, Status: fns.classify(attributable, within), Detail: detail}
}

// evalRemoteRefs classifies every change to the bare remote's own refs
// against decl.MayPush (parent dc-7: a remote-tracking ref a push moves
// belongs to may-push). Attribution is log.HasVerb("push"): gitx.Push's
// argv ("push --set-upstream origin HEAD") never names the branch it
// pushes, so token matching on the ref name cannot work here.
func evalRemoteRefs(decl ws.Declaration, before, after Snapshot, log CommandLog, fns evalFuncs) []Verdict {
	var out []Verdict
	for _, ref := range sortedKeys(after.RemoteRefs) {
		sha := after.RemoteRefs[ref]
		if beforeSHA, existed := before.RemoteRefs[ref]; existed && beforeSHA == sha {
			continue
		}
		attributable := log.HasVerb("push")
		out = append(out, Verdict{
			Field:  "may_push",
			Status: fns.classify(attributable, decl.MayPush),
			Detail: fmt.Sprintf("the remote's ref %s changed to %s", ref, short(sha)),
		})
	}
	return out
}

// evalHeadSwitch reports HEAD's change, if any, against decl.HeadSwitch.
// Directly state-diff-provable (symbolic-ref before vs. after), so this
// needs no log attribution.
func evalHeadSwitch(decl ws.Declaration, before, after Snapshot) []Verdict {
	if before.Head == after.Head {
		return nil
	}
	status := Outside
	if decl.HeadSwitch {
		status = Within
	}
	return []Verdict{{
		Field:  "head_switch",
		Status: status,
		Detail: fmt.Sprintf("HEAD moved from %s to %s", headLabel(before.Head), headLabel(after.Head)),
	}}
}

func headLabel(h Head) string {
	if h.Detached {
		return "detached@" + short(h.Commit)
	}
	return h.Branch
}

// evalWorktrees classifies every linked-worktree add and remove against
// decl.Worktrees. Attribution is token matching on the worktree's absolute
// path, which every gitx worktree call (add, add --detach, remove) carries
// literally.
func evalWorktrees(decl ws.Declaration, before, after Snapshot, storeRoot string, log CommandLog, fns evalFuncs) []Verdict {
	beforeSet := worktreeSet(before.Worktrees)
	afterSet := worktreeSet(after.Worktrees)
	var out []Verdict
	for _, path := range sortedWorktreePaths(afterSet) {
		if _, existed := beforeSet[path]; existed {
			continue
		}
		out = append(out, worktreeVerdict(decl, path, storeRoot, false, log, fns, fmt.Sprintf("worktree added at %s", path)))
	}
	for _, path := range sortedWorktreePaths(beforeSet) {
		if _, stillThere := afterSet[path]; stillThere {
			continue
		}
		out = append(out, worktreeVerdict(decl, path, storeRoot, true, log, fns, fmt.Sprintf("worktree removed from %s", path)))
	}
	return out
}

func worktreeVerdict(decl ws.Declaration, path, storeRoot string, registeredBefore bool, log CommandLog, fns evalFuncs, detail string) Verdict {
	within := false
	for _, p := range decl.Worktrees {
		if p.Matches(path, storeRoot, registeredBefore) {
			within = true
			break
		}
	}
	return Verdict{Field: "worktrees", Status: fns.classify(log.Mentions(path), within), Detail: detail}
}

func worktreeSet(list []gitx.WorktreeEntry) map[string]gitx.WorktreeEntry {
	m := make(map[string]gitx.WorktreeEntry, len(list))
	for _, w := range list {
		m[w.Path] = w
	}
	return m
}

func sortedWorktreePaths(m map[string]gitx.WorktreeEntry) []string {
	out := make([]string, 0, len(m))
	for p := range m {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// carryFacts are the raw, declaration-independent facts an index-carry
// check needs.
type carryFacts struct {
	refused        bool     // exit 2 and no mutation at all (noMutation held)
	commits        int      // new commits the ritual created
	hadForeign     bool     // the index already differed from HEAD before the ritual ran
	carriesForeign bool     // a new commit recorded a path that was already staged before the ritual ran
	carriedPaths   []string // the specific paths carriesForeign names, sorted
}

// label is what facts actually show, for Detail text — not what the
// declaration said, which conforms checks separately.
func (f carryFacts) label() ws.IndexCarry {
	switch {
	case f.refused:
		return ws.CarryRefused
	case f.commits == 0:
		return ws.CarryNoCommit
	case f.carriesForeign:
		return ws.CarryCarried
	default:
		return ws.CarryScoped
	}
}

// conforms reports whether facts is consistent with declared (parent
// dc-3's four states, read per-run rather than as one flat label):
//
//   - refused promises a refusal only when there was a foreign entry to
//     refuse over; parent ac-2 runs a refusing ritual in the clean state
//     too precisely so it is ALSO observed completing, so a refused
//     declaration is not held to refusing when nothing was there to refuse.
//   - no_commit promises no commit object exists, in either state.
//   - scoped promises it neither refuses nor carries a foreign entry into
//     a commit: a ritual that silently refuses instead of completing its
//     declared scoped commit is itself a defect this must catch
//     (obligation: "refuses while declaring scoped fails").
//   - carried permits (never requires) a foreign entry to enter, so it is
//     never falsified by this check alone.
func (f carryFacts) conforms(declared ws.IndexCarry) bool {
	switch declared {
	case ws.CarryRefused:
		if !f.hadForeign {
			return true
		}
		return f.refused
	case ws.CarryNoCommit:
		return f.commits == 0
	case ws.CarryScoped:
		return !f.refused && !f.carriesForeign
	case ws.CarryCarried:
		return true
	default:
		return false
	}
}

// evalCommitsAndCarry classifies every file every new commit recorded
// against decl.StagePaths/UntrackedMayEnter, and asserts decl.IndexCarry
// exactly (parent ac-1: "asserts the declared index carry exactly").
// Commits are state-diff-provable by construction — a snapshot pair always
// brackets exactly one ritual run, so any commit reachable after and not
// before is unambiguously this run's — so, unlike refs and worktrees, no
// log attribution applies here.
func evalCommitsAndCarry(decl ws.Declaration, exit int, before, after Snapshot, fns evalFuncs) []Verdict {
	var newCommits []string
	for sha := range after.Commits {
		if !before.Commits[sha] {
			newCommits = append(newCommits, sha)
		}
	}
	sort.Strings(newCommits)

	foreign := preStagedBefore(before)
	beforeUntracked := map[string]bool{}
	for _, w := range before.Working {
		if w.Index == '?' {
			beforeUntracked[w.Path] = true
		}
	}

	var out []Verdict
	for _, sha := range newCommits {
		for _, path := range after.CommitFiles[sha] {
			if foreign[path] {
				continue // reported once, via the index-carry verdict below
			}
			within := false
			for _, p := range decl.StagePaths {
				if p.Matches(path) {
					within = true
					break
				}
			}
			switch {
			case within:
				out = append(out, Verdict{Field: "stage_paths", Status: Within, Detail: fmt.Sprintf("commit %s recorded %s", short(sha), path)})
			case beforeUntracked[path] && decl.UntrackedMayEnter:
				out = append(out, Verdict{Field: "untracked_may_enter", Status: Within, Detail: fmt.Sprintf("commit %s recorded previously-untracked %s", short(sha), path)})
			default:
				out = append(out, Verdict{Field: "stage_paths", Status: Outside, Detail: fmt.Sprintf("commit %s recorded %s, which is not a declared stage path", short(sha), path)})
			}
		}
	}

	facts := observeCarryFacts(fns, exit, before, after, newCommits, foreign)
	status := Within
	if !facts.conforms(decl.IndexCarry) {
		status = Outside
	}
	out = append(out, Verdict{
		Field:  "index_carry",
		Status: status,
		Detail: fmt.Sprintf("declares %s; observed %s (new commits=%d, refused=%v, carried foreign entries=%v)",
			decl.IndexCarry, facts.label(), facts.commits, facts.refused, facts.carriedPaths),
	})
	return out
}

func observeCarryFacts(fns evalFuncs, exit int, before, after Snapshot, newCommits []string, foreign map[string]bool) carryFacts {
	cf := carryFacts{
		refused:    exit == 2 && fns.noMutation(before, after),
		commits:    len(newCommits),
		hadForeign: len(foreign) > 0,
	}
	carried := map[string]bool{}
	for _, sha := range newCommits {
		for _, path := range after.CommitFiles[sha] {
			if foreign[path] {
				cf.carriesForeign = true
				carried[path] = true
			}
		}
	}
	for path := range carried {
		cf.carriedPaths = append(cf.carriedPaths, path)
	}
	sort.Strings(cf.carriedPaths)
	return cf
}

// preStagedBefore returns the paths whose index entry already differed
// from HEAD before the ritual ran: git status's index column is anything
// other than ' ' (unchanged), '?' (untracked), or '!' (ignored). This is
// the fixture's "pre-staged unrelated index entry" generalized: whatever
// the operator had already staged, by any name, that a scoped ritual must
// never let ride into its own commit (parent dc-3, dc-7).
func preStagedBefore(before Snapshot) map[string]bool {
	staged := map[string]bool{}
	for _, w := range before.Working {
		if w.Index != ' ' && w.Index != '?' && w.Index != '!' {
			staged[w.Path] = true
			if w.OrigPath != "" {
				staged[w.OrigPath] = true
			}
		}
	}
	return staged
}

// snapshotsEqual reports whether before and after are identical across
// every sensor — refs, HEAD, the index, the working tree and its content
// hashes, the linked-worktree list, and the commit set — the "no mutation
// at all" half of a refusal's exact assertion (parent ac-1).
func snapshotsEqual(before, after Snapshot) bool {
	return reflect.DeepEqual(before.LocalRefs, after.LocalRefs) &&
		reflect.DeepEqual(before.RemoteRefs, after.RemoteRefs) &&
		before.Head == after.Head &&
		reflect.DeepEqual(before.Index, after.Index) &&
		reflect.DeepEqual(before.Working, after.Working) &&
		reflect.DeepEqual(before.Hashes, after.Hashes) &&
		reflect.DeepEqual(worktreeSet(before.Worktrees), worktreeSet(after.Worktrees)) &&
		reflect.DeepEqual(before.Commits, after.Commits)
}

func short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

func shortRef(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
