package ritualwitness

import (
	"reflect"
	"strings"
	"testing"

	ws "github.com/jyang234/verdi/internal/writescope"
)

// testHarnessMutants demonstrates the four mutants spec/ritual-effect-
// witness ac-1's GREEN list requires: for each, it re-evaluates an already-
// captured (or hand-built) before/after Snapshot pair through evaluateWith
// with exactly one of evalFuncs' three decision functions swapped for a
// broken one, and shows the verdict flips from the correct answer the real
// function gives to a wrong one — the same flip that would make
// TestHarness_SensorsAndVerdicts's own assertions go red if the mutant
// were live in production (mirrors internal/writescope's witness_test.go
// liveMutants, mutating behavior here instead of registry data, since the
// thing under test is the harness's own decision logic, not a data table).
func testHarnessMutants(t *testing.T, unattributableCase Result, unattributableDecl ws.Declaration, checkedOutCase Result, checkedOutDecl ws.Declaration) {
	t.Run("treat unattributable as within", func(t *testing.T) {
		real := verdictFor(t, unattributableCase, "refs_create", "ritual/sneaky")
		if real.Status != Unattributable {
			t.Fatalf("setup: real classify gave %s, want unattributable", real.Status)
		}

		mutantClassify := func(_, within bool) Status {
			if within {
				return Within // BUG: never checks attributable at all
			}
			return Outside
		}
		mutant := verdictFor(t, reEvaluate(unattributableCase, unattributableDecl, evalFuncs{
			classify:   mutantClassify,
			matchRef:   defaultEvalFuncs().matchRef,
			noMutation: defaultEvalFuncs().noMutation,
		}), "refs_create", "ritual/sneaky")
		if mutant.Status != Within {
			t.Fatalf("mutant classify gave %s, want within (the danger this mutant demonstrates)", mutant.Status)
		}
		if mutant.Status == real.Status {
			t.Fatal("the mutant did not change the verdict; it demonstrates nothing")
		}
	})

	t.Run("match RefCheckedOut against any branch", func(t *testing.T) {
		real := verdictFor(t, checkedOutCase, "refs_move", "refs/heads/side")
		if real.Status != Outside {
			t.Fatalf("setup: real matchRef gave %s, want outside", real.Status)
		}

		mutantMatchRef := func(p ws.RefPattern, ref, checkedOutBefore string) bool {
			if p == ws.RefCheckedOut {
				return true // BUG: ignores checkedOutBefore, matches any branch
			}
			return p.Matches(ref, checkedOutBefore)
		}
		mutant := verdictFor(t, reEvaluate(checkedOutCase, checkedOutDecl, evalFuncs{
			classify:   defaultEvalFuncs().classify,
			matchRef:   mutantMatchRef,
			noMutation: defaultEvalFuncs().noMutation,
		}), "refs_move", "refs/heads/side")
		if mutant.Status != Within {
			t.Fatalf("mutant matchRef gave %s, want within (the danger this mutant demonstrates)", mutant.Status)
		}
		if mutant.Status == real.Status {
			t.Fatal("the mutant did not change the verdict; it demonstrates nothing")
		}
	})

	t.Run("skip the index sensor", func(t *testing.T) {
		decl := ws.Declaration{
			Ritual:     "probe_refused",
			Verbs:      []ws.Verb{ws.CLI("probe refused")},
			RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"},
			HeadSwitch: true,
			StagePaths: []ws.PathPattern{"owned/*"},
			IndexCarry: ws.CarryRefused,
		}
		// before/after are hand-built: identical everywhere EXCEPT the
		// index, where a stray entry appeared — exactly what a guard that
		// checks too late, after already staging something, would leak.
		before := Snapshot{
			LocalRefs:   map[string]string{"refs/heads/main": sha('a')},
			Head:        Head{Branch: "main", Commit: sha('a')},
			Index:       []IndexEntry{{Mode: "100644", SHA: "blob1", Stage: 0, Path: ForeignFile}},
			Working:     []WorkingEntry{{Index: 'A', Worktree: ' ', Path: ForeignFile}},
			Commits:     map[string]bool{sha('a'): true},
			CommitFiles: map[string][]string{sha('a'): nil},
		}
		after := Snapshot{
			LocalRefs: map[string]string{"refs/heads/main": sha('a')},
			Head:      Head{Branch: "main", Commit: sha('a')},
			Index: []IndexEntry{
				{Mode: "100644", SHA: "blob1", Stage: 0, Path: ForeignFile},
				{Mode: "100644", SHA: "blob2", Stage: 0, Path: "owned/leak.txt"},
			},
			Working:     []WorkingEntry{{Index: 'A', Worktree: ' ', Path: ForeignFile}, {Index: 'A', Worktree: ' ', Path: "owned/leak.txt"}},
			Commits:     map[string]bool{sha('a'): true},
			CommitFiles: map[string][]string{sha('a'): nil},
		}
		const exit = 2 // the ritual claims a clean refusal

		real := verdictFor(t, Result{Verdicts: evaluateWith(decl, exit, before, after, "/repo", CommandLog{OK: true}, defaultEvalFuncs())}, "index_carry", "")
		if real.Status != Outside {
			t.Fatalf("setup: real noMutation gave %s, want outside (a stray staged entry leaked despite the claimed refusal)", real.Status)
		}

		mutantNoMutation := func(before, after Snapshot) bool {
			// BUG: never looks at Index, Working, or Hashes at all.
			return reflect.DeepEqual(before.LocalRefs, after.LocalRefs) &&
				reflect.DeepEqual(before.RemoteRefs, after.RemoteRefs) &&
				before.Head == after.Head &&
				reflect.DeepEqual(worktreeSet(before.Worktrees), worktreeSet(after.Worktrees)) &&
				reflect.DeepEqual(before.Commits, after.Commits)
		}
		mutant := verdictFor(t, Result{Verdicts: evaluateWith(decl, exit, before, after, "/repo", CommandLog{OK: true}, evalFuncs{
			classify:   defaultEvalFuncs().classify,
			matchRef:   defaultEvalFuncs().matchRef,
			noMutation: mutantNoMutation,
		})}, "index_carry", "")
		if mutant.Status != Within {
			t.Fatalf("mutant noMutation gave %s, want within (the danger this mutant demonstrates)", mutant.Status)
		}
		if mutant.Status == real.Status {
			t.Fatal("the mutant did not change the verdict; it demonstrates nothing")
		}
	})

	t.Run("let refused pass with a mutation", func(t *testing.T) {
		decl := ws.Declaration{
			Ritual:     "probe_refused_leak",
			Verbs:      []ws.Verb{ws.CLI("probe refused-leak")},
			RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"},
			HeadSwitch: true,
			StagePaths: []ws.PathPattern{"owned/*"},
			IndexCarry: ws.CarryRefused,
		}
		// before/after differ in refs, HEAD, and the commit set: a real
		// branch-and-commit happened despite the ritual claiming exit 2.
		before := Snapshot{
			LocalRefs:   map[string]string{"refs/heads/main": sha('a')},
			Head:        Head{Branch: "main", Commit: sha('a')},
			Working:     []WorkingEntry{{Index: 'A', Worktree: ' ', Path: ForeignFile}},
			Commits:     map[string]bool{sha('a'): true},
			CommitFiles: map[string][]string{sha('a'): nil},
		}
		after := Snapshot{
			LocalRefs: map[string]string{
				"refs/heads/main":        sha('a'),
				"refs/heads/ritual/leak": sha('b'),
			},
			Head:    Head{Branch: "ritual/leak", Commit: sha('b')},
			Working: []WorkingEntry{{Index: 'A', Worktree: ' ', Path: ForeignFile}},
			Commits: map[string]bool{sha('a'): true, sha('b'): true},
			CommitFiles: map[string][]string{
				sha('a'): nil,
				sha('b'): {"owned/leak.txt"},
			},
		}
		const exit = 2

		real := verdictFor(t, Result{Verdicts: evaluateWith(decl, exit, before, after, "/repo", CommandLog{OK: true}, defaultEvalFuncs())}, "index_carry", "")
		if real.Status != Outside {
			t.Fatalf("setup: real noMutation gave %s, want outside (a branch and a commit exist despite the claimed refusal)", real.Status)
		}

		mutantNoMutation := func(Snapshot, Snapshot) bool { return true } // BUG: trusts exit 2 blindly
		mutant := verdictFor(t, Result{Verdicts: evaluateWith(decl, exit, before, after, "/repo", CommandLog{OK: true}, evalFuncs{
			classify:   defaultEvalFuncs().classify,
			matchRef:   defaultEvalFuncs().matchRef,
			noMutation: mutantNoMutation,
		})}, "index_carry", "")
		if mutant.Status != Within {
			t.Fatalf("mutant noMutation gave %s, want within (the danger this mutant demonstrates)", mutant.Status)
		}
		if mutant.Status == real.Status {
			t.Fatal("the mutant did not change the verdict; it demonstrates nothing")
		}
	})
}

// sha returns a 40-character hex string of b repeated, a syntactically
// valid (if fake) full object id for hand-built Snapshot fixtures.
func sha(b byte) string {
	out := make([]byte, 40)
	for i := range out {
		out[i] = b
	}
	return string(out)
}

// verdictFor returns the one verdict in res.Verdicts matching field and
// (if non-empty) a Detail substring, failing the test if there is not
// exactly one.
func verdictFor(t *testing.T, res Result, field, substr string) Verdict {
	t.Helper()
	var found []Verdict
	for _, v := range res.Verdicts {
		if v.Field == field && (substr == "" || strings.Contains(v.Detail, substr)) {
			found = append(found, v)
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected exactly one %s verdict (detail containing %q), got %d:\n%s", field, substr, len(found), formatVerdicts(res.Verdicts))
	}
	return found[0]
}

// reEvaluate re-runs Evaluate's logic over res's already-captured before/
// after Snapshots, command log, and store root, against decl, with fns
// substituted — the mechanism the first two mutant demonstrations above use
// to replay a real captured case through a broken decision function.
func reEvaluate(res Result, decl ws.Declaration, fns evalFuncs) Result {
	res.Verdicts = evaluateWith(decl, res.Exit, res.Before, res.After, res.StoreRoot, res.Log, fns)
	return res
}
