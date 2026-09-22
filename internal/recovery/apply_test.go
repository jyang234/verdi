package recovery

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/reclaim"
	"github.com/jyang234/verdi/internal/store"
)

func TestApply_UnwindEmptyBranchCut(t *testing.T) {
	repo, cfg := fixtureStore(t)
	cut := cutEmptyBranch(t, repo, "close/checkout") // leaves close/checkout checked out; cut == repo.Head

	out, err := Apply(context.Background(), cfg, "spec/checkout", "unwind-branch-cut:close/checkout", io.Discard)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for _, pc := range out.Postconditions {
		if !pc.Held {
			t.Fatalf("postcondition failed: %+v", pc)
		}
	}
	if hasState(out.After, StateEmptyBranchCut) {
		t.Fatal("state still recognized after unwind")
	}
	if out.Journey == nil || out.JourneyErr != nil {
		t.Fatalf("journey not re-derived: %v", out.JourneyErr)
	}
	if cur, err := gitx.CurrentBranch(context.Background(), repo.Dir); err != nil || cur != "main" {
		t.Fatalf("branch = %q, err = %v, want main", cur, err)
	}
	_ = cut
}

// TestApply_RefusesWhenPreconditionMoved proves R-RR3-9's re-proof: a
// single mutation applied BEFORE calling Apply cannot make Apply's own
// two adjacent, synchronous Gather calls disagree with each other (both
// see the identical, already-mutated reality) — confirmed empirically:
// extending close/checkout with its own unshared commit before Apply is
// even called would make it no longer "empty" (R-RR3-5) at all, so
// Apply's OWN FIRST gather would already fail to recognize the state,
// producing ErrUnknownChoice, never ErrPreconditionFailed. The only
// place a fresh, immediately-pre-execution re-gather can ever observe
// something the discovery gather did not is the narrow window R-RR3-9
// itself names between discovery and re-proof — so this test uses
// applyReproveHook (mirroring cmd/verdi/recover.go's own
// recoverObserverHook) to land the mutation exactly there, simulating a
// concurrent writer.
func TestApply_RefusesWhenPreconditionMoved(t *testing.T) {
	repo, cfg := fixtureStore(t)
	cutEmptyBranch(t, repo, "close/checkout")

	var movedTip string
	applyReproveHook = func() {
		ctx := context.Background()
		if err := os.WriteFile(filepath.Join(repo.Dir, "c.txt"), []byte("c\n"), 0o644); err != nil {
			t.Fatalf("writing c.txt: %v", err)
		}
		if err := gitx.AddPaths(ctx, repo.Dir, "c.txt"); err != nil {
			t.Fatalf("AddPaths: %v", err)
		}
		if _, err := gitx.CreateCommit(ctx, repo.Dir, "own commit"); err != nil {
			t.Fatalf("CreateCommit: %v", err)
		}
		tip, err := gitx.RevParse(ctx, repo.Dir, "close/checkout")
		if err != nil {
			t.Fatalf("RevParse(close/checkout): %v", err)
		}
		movedTip = tip
	}
	t.Cleanup(func() { applyReproveHook = nil })

	_, err := Apply(context.Background(), cfg, "spec/checkout", "unwind-branch-cut:close/checkout", io.Discard)
	if !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("err = %v, want ErrPreconditionFailed", err)
	}
	after := gitOutput(t, repo.Dir, "rev-parse", "close/checkout")
	if strings.TrimSpace(after) != movedTip {
		t.Fatalf("close/checkout changed by Apply's own refusal: got %s, want the hook's own commit %s", strings.TrimSpace(after), movedTip)
	}
}

func TestApply_RefusesNoExecutor(t *testing.T) {
	repo, cfg := fixtureStore(t)
	path := writeStaleWriterLock(t, repo)
	p := Derive(mustGather(t, cfg, "spec/checkout"))
	if len(p.States) == 0 || len(p.States[0].Choices) == 0 {
		t.Fatalf("no manual choice recognized: %+v", p.States)
	}
	id := p.States[0].Choices[0].ID

	_, err := Apply(context.Background(), cfg, "spec/checkout", id, io.Discard)
	if !errors.Is(err, ErrNoExecutor) || !strings.Contains(err.Error(), "rm ") {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatal("lock removed by a choice with no executor")
	}
}

func TestApply_UnknownChoiceListsKnown(t *testing.T) {
	repo, cfg := fixtureStore(t)
	cutEmptyBranch(t, repo, "close/checkout")

	_, err := Apply(context.Background(), cfg, "spec/checkout", "no-such-choice", io.Discard)
	if !errors.Is(err, ErrUnknownChoice) {
		t.Fatalf("err = %v, want ErrUnknownChoice", err)
	}
	p := Derive(mustGather(t, cfg, "spec/checkout"))
	for _, id := range choiceIDs(p) {
		if !strings.Contains(err.Error(), id) {
			t.Fatalf("err = %v, want it to list known choice id %q", err, id)
		}
	}
}

func TestApply_ReclaimDelegation(t *testing.T) {
	repo, cfg := fixtureStore(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	wtPath := cutMergedRitualWorktree(t, repo, "feature/checkout")

	p := Derive(mustGather(t, cfg, "spec/checkout"))
	if _, ok := stateFor(p.States, StateStrandedResidue, "feature/checkout"); !ok {
		t.Fatalf("stranded-residue not recognized for feature/checkout: %+v", p.States)
	}

	var stderr strings.Builder
	out, err := Apply(context.Background(), cfg, "spec/checkout", "reclaim:feature/checkout", &stderr)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for _, pc := range out.Postconditions {
		if !pc.Held {
			t.Fatalf("postcondition failed: %+v", pc)
		}
	}
	if ok, _ := gitx.HasLocalBranch(context.Background(), repo.Dir, "feature/checkout"); ok {
		t.Fatal("feature/checkout still exists after reclaim")
	}
	if _, statErr := os.Stat(wtPath); !os.IsNotExist(statErr) {
		t.Fatalf("worktree %s still exists after reclaim (stat err %v)", wtPath, statErr)
	}
	if !strings.Contains(stderr.String(), "reclaimed:") || !strings.Contains(stderr.String(), "feature/checkout") {
		t.Fatalf("stderr = %q, want reclaim's own Row line verbatim", stderr.String())
	}
	// Wave-review m4: every line this verb writes to the error stream is
	// attributed, the row's own text unchanged after the prefix.
	for _, ln := range strings.Split(strings.TrimRight(stderr.String(), "\n"), "\n") {
		if !strings.HasPrefix(ln, "recover: ") {
			t.Fatalf("stderr line %q does not carry the verb prefix", ln)
		}
	}
}

func TestApply_ReclaimRefusalSurfacesVerbatim(t *testing.T) {
	repo, cfg := fixtureStore(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	wtPath := cutMergedRitualWorktree(t, repo, "feature/checkout")

	applyReproveHook = func() {
		if err := os.WriteFile(filepath.Join(wtPath, "wip.txt"), []byte("wip\n"), 0o644); err != nil {
			t.Fatalf("dirtying worktree %s: %v", wtPath, err)
		}
	}
	t.Cleanup(func() { applyReproveHook = nil })

	_, err := Apply(context.Background(), cfg, "spec/checkout", "reclaim:feature/checkout", io.Discard)
	if !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("err = %v, want ErrPreconditionFailed", err)
	}
	if !strings.Contains(err.Error(), "kept:") || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("err = %v, want reclaim's own kept:dirty Row.Line() verbatim", err)
	}
	if _, statErr := os.Stat(wtPath); statErr != nil {
		t.Fatalf("worktree %s removed by a refused reclaim: %v", wtPath, statErr)
	}
	if ok, _ := gitx.HasLocalBranch(context.Background(), repo.Dir, "feature/checkout"); !ok {
		t.Fatal("branch feature/checkout removed by a refused reclaim")
	}
}

func TestApply_AmbiguityWithholdsUnwind(t *testing.T) {
	repo, cfg := fixtureStore(t)
	cutEmptyBranch(t, repo, "close/checkout")
	moveArchiveUncommitted(t, repo)

	_, err := Apply(context.Background(), cfg, "spec/checkout", "unwind-branch-cut:close/checkout", io.Discard)
	if !errors.Is(err, ErrUnknownChoice) {
		t.Fatalf("err = %v, want ErrUnknownChoice (R-RR3-8 withholds the unwind choice)", err)
	}
	if _, statErr := os.Stat(store.ArchiveSpecPath(repo.Dir, "checkout")); statErr != nil {
		t.Fatal("archive move undone by a refused Apply call")
	}
	if _, statErr := os.Stat(store.ActiveSpecPath(repo.Dir, "checkout")); !os.IsNotExist(statErr) {
		t.Fatal("active spec restored by a refused Apply call")
	}
}

// declaredChoice derives the projection for ref and returns the choice
// with id — the DECLARED sentences Apply's own postcondition results must
// line up with, one for one (M1).
func declaredChoice(t *testing.T, cfg *store.Config, ref, id string) Choice {
	t.Helper()
	p := Derive(mustGather(t, cfg, ref))
	c, _, ok := findChoice(p, id)
	if !ok {
		t.Fatalf("choice %q not in the derived projection: %v", id, choiceIDs(p))
	}
	return c
}

// assertPostconditionsAligned is M1's own check at the call site: every
// declared postcondition was evaluated, in order, under its own declared
// text.
func assertPostconditionsAligned(t *testing.T, declared Choice, out Outcome) {
	t.Helper()
	if len(out.Postconditions) != len(declared.Postconditions) {
		t.Fatalf("evaluated %d postconditions, declared %d: %+v vs %v", len(out.Postconditions), len(declared.Postconditions), out.Postconditions, declared.Postconditions)
	}
	for i, pc := range out.Postconditions {
		if pc.Text != declared.Postconditions[i] {
			t.Fatalf("postcondition[%d] text = %q, declared %q", i, pc.Text, declared.Postconditions[i])
		}
	}
}

// TestApply_UnwindReturnBranchAheadOfCut is C1/R-RR3-19's own red-first
// proof for the CURRENT-checkout cut mechanism's containing tier: close/
// checkout is cut from main at C0, main then advances to C1 (anyone
// merges), close/checkout is still empty because C1 descends C0, and
// resolveReturnBranch resolves the return branch to main by CONTAINMENT.
// A correct unwind therefore leaves HEAD at main's tip C1, which is NOT
// the cut point — R-RR3-5's own "the return branch may sit ahead of the
// ritual tip". Every postcondition must hold.
func TestApply_UnwindReturnBranchAheadOfCut(t *testing.T) {
	repo, cfg := fixtureStore(t)
	cut := cutEmptyBranch(t, repo, "close/checkout")
	mainTip := advanceBranch(t, repo, "main", "ahead.txt")
	if mainTip == cut {
		t.Fatalf("fixture did not advance main past the cut point %s", cut)
	}

	declared := declaredChoice(t, cfg, "spec/checkout", "unwind-branch-cut:close/checkout")
	out, err := Apply(context.Background(), cfg, "spec/checkout", "unwind-branch-cut:close/checkout", io.Discard)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	assertPostconditionsAligned(t, declared, out)
	for _, pc := range out.Postconditions {
		if !pc.Held {
			t.Fatalf("postcondition failed on a correct ahead-of-cut unwind: %+v (all: %+v)", pc, out.Postconditions)
		}
	}
	if hasState(out.After, StateEmptyBranchCut) {
		t.Fatal("state still recognized after unwind")
	}
	if cur, err := gitx.CurrentBranch(context.Background(), repo.Dir); err != nil || cur != "main" {
		t.Fatalf("branch = %q, err = %v, want main", cur, err)
	}
	head := strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "HEAD"))
	if head != mainTip {
		t.Fatalf("HEAD = %s, want main's tip %s", head, mainTip)
	}
}

// TestApply_UnwindResolvedBaseReturnBranchAheadOfCut is C1's second
// mechanism (R-RR3-5's resolved-base cuts: design start and policy
// adopt): design/checkout's return branch is the freshly RE-RESOLVED
// default branch, whose tip is under no obligation to equal the design
// branch's own cut point.
func TestApply_UnwindResolvedBaseReturnBranchAheadOfCut(t *testing.T) {
	repo, cfg := fixtureStore(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	cut := cutEmptyBranch(t, repo, "design/checkout")
	mainTip := advanceBranch(t, repo, "main", "ahead.txt")
	if mainTip == cut {
		t.Fatalf("fixture did not advance main past the cut point %s", cut)
	}

	declared := declaredChoice(t, cfg, "spec/checkout", "unwind-branch-cut:design/checkout")
	out, err := Apply(context.Background(), cfg, "spec/checkout", "unwind-branch-cut:design/checkout", io.Discard)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	assertPostconditionsAligned(t, declared, out)
	for _, pc := range out.Postconditions {
		if !pc.Held {
			t.Fatalf("postcondition failed on a correct resolved-base unwind: %+v (all: %+v)", pc, out.Postconditions)
		}
	}
	if ok, _ := gitx.HasLocalBranch(context.Background(), repo.Dir, "design/checkout"); ok {
		t.Fatal("design/checkout still exists after unwind")
	}
	head := strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "HEAD"))
	if head != mainTip {
		t.Fatalf("HEAD = %s, want main's tip %s", head, mainTip)
	}
}

// TestApply_RefusesWhenEmptinessLostInReproveWindow is I1's own red-first
// proof of ac-9's literal third clause ("has no commits of its own" is
// re-proved AT EXECUTION TIME) for the resolved-base mechanism, where the
// return branch does NOT derive from the emptiness witnesses at all:
// design/checkout sits at C1 with main at C2; in the re-prove window a
// concurrent writer rewinds refs/heads/main to C0, so design/checkout now
// carries a commit nothing else has. R-RR3-9: exit 1, NOTHING changed —
// no checkout, no delete.
func TestApply_RefusesWhenEmptinessLostInReproveWindow(t *testing.T) {
	repo, cfg := fixtureStore(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	c0 := strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "main"))
	advanceBranch(t, repo, "main", "c1.txt")
	designTip := cutEmptyBranch(t, repo, "design/checkout")
	advanceBranch(t, repo, "main", "c2.txt")

	applyReproveHook = func() {
		runGit(t, repo.Dir, "update-ref", "refs/heads/main", c0)
	}
	t.Cleanup(func() { applyReproveHook = nil })

	_, err := Apply(context.Background(), cfg, "spec/checkout", "unwind-branch-cut:design/checkout", io.Discard)
	if !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("err = %v, want ErrPreconditionFailed", err)
	}
	if cur, cerr := gitx.CurrentBranch(context.Background(), repo.Dir); cerr != nil || cur != "design/checkout" {
		t.Fatalf("current branch = %q, err = %v: a refused run switched the working tree", cur, cerr)
	}
	if ok, _ := gitx.HasLocalBranch(context.Background(), repo.Dir, "design/checkout"); !ok {
		t.Fatal("design/checkout deleted by a refused run")
	}
	if got := strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "design/checkout")); got != designTip {
		t.Fatalf("design/checkout = %s, want %s untouched", got, designTip)
	}
}

// TestApply_UnwindLeftDeleteFailedIsViolated is I2's own reachable
// VIOLATED case (and the giving-up-outcome proof): close/checkout is held
// by a linked worktree, so git's own safe `branch -d` REFUSES. The switch
// back succeeds, so exactly the first postcondition comes up VIOLATED
// with the branch's own observed tip — reported, never swallowed, and
// Apply itself returns no error (the verdict is the postcondition).
func TestApply_UnwindLeftDeleteFailedIsViolated(t *testing.T) {
	repo, cfg := fixtureStore(t)
	cut := cutEmptyBranch(t, repo, "close/checkout")
	if err := gitx.CheckoutExisting(context.Background(), repo.Dir, "main"); err != nil {
		t.Fatalf("CheckoutExisting(main): %v", err)
	}
	holdBranchInLinkedWorktree(t, repo, "close/checkout")

	declared := declaredChoice(t, cfg, "spec/checkout", "unwind-branch-cut:close/checkout")
	out, err := Apply(context.Background(), cfg, "spec/checkout", "unwind-branch-cut:close/checkout", io.Discard)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	assertPostconditionsAligned(t, declared, out)
	if out.Postconditions[0].Held {
		t.Fatalf("postcondition[0] held although `git branch -d` refused: %+v", out.Postconditions)
	}
	if want := "exists at " + cut; out.Postconditions[0].Observed != want {
		t.Fatalf("observed = %q, want %q", out.Postconditions[0].Observed, want)
	}
	for _, pc := range out.Postconditions[1:] {
		if !pc.Held {
			t.Fatalf("postcondition %+v should still hold (the switch back succeeded)", pc)
		}
	}
}

// TestApply_JourneyFailureAfterExecutionIsReported is I2's own proof of
// R-RR3-9's operational branch: the executor ran, the projection still
// carries what it observed, and the journey re-derivation's own failure
// is surfaced on Outcome.JourneyErr (never swallowed, never an error that
// hides the executed work).
func TestApply_JourneyFailureAfterExecutionIsReported(t *testing.T) {
	repo, cfg := fixtureStore(t)
	cutEmptyBranch(t, repo, "close/checkout")

	applyJourneyHook = func() {
		if err := os.RemoveAll(store.ActiveSpecDir(repo.Dir, "checkout")); err != nil {
			t.Fatalf("removing the active spec dir: %v", err)
		}
	}
	t.Cleanup(func() { applyJourneyHook = nil })

	out, err := Apply(context.Background(), cfg, "spec/checkout", "unwind-branch-cut:close/checkout", io.Discard)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if out.JourneyErr == nil || out.Journey != nil {
		t.Fatalf("JourneyErr = %v, Journey = %+v, want a reported journey failure", out.JourneyErr, out.Journey)
	}
	if len(out.Postconditions) == 0 {
		t.Fatal("no postconditions reported although the executor ran")
	}
	if ok, _ := gitx.HasLocalBranch(context.Background(), repo.Dir, "close/checkout"); ok {
		t.Fatal("close/checkout still exists: the executor did not run")
	}
}

// TestApply_ReclaimPostconditionsMatchDeclaredTexts is M1 for the second
// executor.
func TestApply_ReclaimPostconditionsMatchDeclaredTexts(t *testing.T) {
	repo, cfg := fixtureStore(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	cutMergedRitualWorktree(t, repo, "feature/checkout")

	declared := declaredChoice(t, cfg, "spec/checkout", "reclaim:feature/checkout")
	out, err := Apply(context.Background(), cfg, "spec/checkout", "reclaim:feature/checkout", io.Discard)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	assertPostconditionsAligned(t, declared, out)
}

// TestCheckReclaimPostconditions_FailsClosedWithoutRows is M4: with no
// row from the actual execution there is no evidence at all, so every
// declared postcondition is VIOLATED — never "held" from nothing.
func TestCheckReclaimPostconditions_FailsClosedWithoutRows(t *testing.T) {
	choice := Choice{
		ID:             "reclaim:feature/checkout",
		Postconditions: []string{"feature/checkout does not exist", "its worktree does not exist"},
	}
	state := RecognizedState{Target: "feature/checkout"}
	for _, rows := range [][]reclaim.Row{nil, {}} {
		results := checkReclaimPostconditions(choice, state, Facts{}, rows)
		if len(results) != len(choice.Postconditions) {
			t.Fatalf("results = %+v, want one per declared postcondition", results)
		}
		for i, r := range results {
			if r.Held {
				t.Fatalf("result[%d] held with no reclaim row at all: %+v", i, r)
			}
			if r.Observed != "no reclaim row returned" {
				t.Fatalf("result[%d].Observed = %q, want %q", i, r.Observed, "no reclaim row returned")
			}
		}
	}
}

// TestAssertPostconditionsDeclared is M1's own negative path: a declared
// sentence that was never evaluated, and an evaluated result whose text
// drifted from its declaration, are each this verb's own operational
// failure naming the offending index — never a silently short report.
func TestAssertPostconditionsDeclared(t *testing.T) {
	declared := Choice{ID: "unwind-branch-cut:close/checkout", Postconditions: []string{"a", "b", "c"}}
	cases := []struct {
		name    string
		results []PostconditionResult
		wantErr string
	}{
		{"aligned", []PostconditionResult{{Text: "a"}, {Text: "b"}, {Text: "c"}}, ""},
		{"one declared sentence unevaluated", []PostconditionResult{{Text: "a"}, {Text: "b"}}, "declares 3 postcondition(s) but 2 were evaluated"},
		{"an extra evaluated result", []PostconditionResult{{Text: "a"}, {Text: "b"}, {Text: "c"}, {Text: "d"}}, "declares 3 postcondition(s) but 4 were evaluated"},
		{"text drifted at index 1", []PostconditionResult{{Text: "a"}, {Text: "elsewhere"}, {Text: "c"}}, `postcondition[1] was evaluated as "elsewhere" but declared as "b"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := assertPostconditionsDeclared(declared, tc.results)
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("err = %v, want nil", err)
			case tc.wantErr != "" && err == nil:
				t.Fatalf("err = nil, want one naming %q", tc.wantErr)
			case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
				t.Fatalf("err = %v, want it to name %q", err, tc.wantErr)
			}
			if err != nil {
				if errors.Is(err, ErrPreconditionFailed) || errors.Is(err, ErrUnknownChoice) || errors.Is(err, ErrNoExecutor) {
					t.Fatalf("err = %v wraps a verdict sentinel; a postcondition-table mismatch is operational (exit 2)", err)
				}
				if !strings.Contains(err.Error(), declared.ID) {
					t.Fatalf("err = %v, want it to name the choice %q", err, declared.ID)
				}
			}
		})
	}
}

// TestBranchTip covers checkUnwindPostconditions's own lookup of the
// return branch's post-execution tip (R-RR3-19), both paths.
func TestBranchTip(t *testing.T) {
	f := Facts{LocalBranches: []BranchTip{{Name: "main", Tip: "aaa"}, {Name: "design/x", Tip: "bbb"}}}
	if tip, ok := branchTip(f, "design/x"); !ok || tip != "bbb" {
		t.Fatalf("branchTip(design/x) = %q, %v, want bbb, true", tip, ok)
	}
	if tip, ok := branchTip(f, "gone"); ok || tip != "" {
		t.Fatalf("branchTip(gone) = %q, %v, want \"\", false", tip, ok)
	}
}

// TestCheckUnwindPostconditions_ReturnBranchVanished is R-RR3-19's own
// fail-closed path: with no post-execution tip for the return branch
// there is nothing to compare HEAD against, so the postcondition is
// VIOLATED, never "held" from no evidence.
func TestCheckUnwindPostconditions_ReturnBranchVanished(t *testing.T) {
	facts := Facts{
		Close:         RitualBranch{Name: "close/checkout", Exists: true, Tip: "c0", EmptyWitnesses: []string{"main"}},
		LocalBranches: []BranchTip{{Name: "main", Tip: "c0"}, {Name: "close/checkout", Tip: "c0"}},
	}
	choice := Choice{
		ID:             "unwind-branch-cut:close/checkout",
		Postconditions: []string{"close/checkout does not exist", "current branch is main", "HEAD is the tip of main"},
	}
	state := RecognizedState{Code: StateEmptyBranchCut, Target: "close/checkout"}
	after := Facts{CurrentBranch: "main", Head: "c0"} // main is gone from LocalBranches
	results := checkUnwindPostconditions(facts, choice, state, after)
	if len(results) != 3 {
		t.Fatalf("results = %+v, want three", results)
	}
	if results[2].Held {
		t.Fatalf("results[2] held although the return branch is not a local branch any more: %+v", results[2])
	}
	if !strings.Contains(results[2].Observed, "not a local branch any more") {
		t.Fatalf("results[2].Observed = %q, want it to say so", results[2].Observed)
	}
}

// TestApply_RefusesWhenEmptinessLostInReproveWindow_CutFromCurrent is I1
// for the other cut mechanism ("for EVERY cut mechanism", plan amendment
// Task 4/I1): close/checkout's return branch DOES derive from the
// emptiness witnesses, so losing emptiness was already caught — but by
// the return-branch clause, which names the wrong thing. The refusal must
// now name ac-9's own third clause.
func TestApply_RefusesWhenEmptinessLostInReproveWindow_CutFromCurrent(t *testing.T) {
	repo, cfg := fixtureStore(t)
	c0 := strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "main"))
	advanceBranch(t, repo, "main", "c1.txt")
	closeTip := cutEmptyBranch(t, repo, "close/checkout")
	advanceBranch(t, repo, "main", "c2.txt")

	applyReproveHook = func() { runGit(t, repo.Dir, "update-ref", "refs/heads/main", c0) }
	t.Cleanup(func() { applyReproveHook = nil })

	_, err := Apply(context.Background(), cfg, "spec/checkout", "unwind-branch-cut:close/checkout", io.Discard)
	if !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("err = %v, want ErrPreconditionFailed", err)
	}
	if !strings.Contains(err.Error(), "carries commit(s) of its own") {
		t.Fatalf("err = %v, want it to name ac-9's own \"has no commits of its own\" clause", err)
	}
	if cur, cerr := gitx.CurrentBranch(context.Background(), repo.Dir); cerr != nil || cur != "close/checkout" {
		t.Fatalf("current branch = %q, err = %v: a refused run switched the working tree", cur, cerr)
	}
	if got := strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "close/checkout")); got != closeTip {
		t.Fatalf("close/checkout = %s, want %s untouched", got, closeTip)
	}
}

// TestApply_UnwindRefusesOnATreeThatWentUnclean covers the window
// R-RR3-21's derive-time guard cannot close: the projection was derived
// over a clean tree, and the tree went unclean before the executor ran.
// The re-proof still refuses (R-RR3-9, nothing changed) — and its
// sentence states the FACT it can actually prove ("the working tree is
// not clean"), never a transition ("no longer"), which it has no
// observation of: derive and re-prove both read the same fresh Gather
// here, and the guard means the offered choice's own start was clean.
func TestApply_UnwindRefusesOnATreeThatWentUnclean(t *testing.T) {
	repo, cfg := fixtureStore(t)
	cutEmptyBranch(t, repo, "close/checkout")

	applyReproveHook = func() {
		if err := os.WriteFile(filepath.Join(repo.Dir, "leftover.txt"), []byte("uncommitted\n"), 0o644); err != nil {
			t.Fatalf("dirtying %s: %v", repo.Dir, err)
		}
	}
	t.Cleanup(func() { applyReproveHook = nil })

	_, err := Apply(context.Background(), cfg, "spec/checkout", "unwind-branch-cut:close/checkout", io.Discard)
	if !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("err = %v, want ErrPreconditionFailed", err)
	}
	if !strings.Contains(err.Error(), "the working tree is not clean") {
		t.Fatalf("err = %v, want it to state the working-tree fact", err)
	}
	// ErrPreconditionFailed's own sentinel ("precondition no longer
	// holds") IS a transition this path observed — the choice was offered,
	// so its start was proved clean. The re-proof's own SENTENCE is what
	// must not invent one.
	if strings.Contains(err.Error(), "no longer clean") {
		t.Fatalf("err = %v, must not assert a transition the tree check never observed", err)
	}
	if ok, _ := gitx.HasLocalBranch(context.Background(), repo.Dir, "close/checkout"); !ok {
		t.Fatal("close/checkout deleted by a refused Apply call")
	}
}

// TestApply_UnwindRefusesOnAnIndexThatWentNonEmpty is the same window for
// the index half.
func TestApply_UnwindRefusesOnAnIndexThatWentNonEmpty(t *testing.T) {
	repo, cfg := fixtureStore(t)
	cutEmptyBranch(t, repo, "close/checkout")

	applyReproveHook = func() {
		path := filepath.Join(repo.Dir, "staged.txt")
		if err := os.WriteFile(path, []byte("staged\n"), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
		if err := gitx.AddPaths(context.Background(), repo.Dir, "staged.txt"); err != nil {
			t.Fatalf("AddPaths: %v", err)
		}
	}
	t.Cleanup(func() { applyReproveHook = nil })

	_, err := Apply(context.Background(), cfg, "spec/checkout", "unwind-branch-cut:close/checkout", io.Discard)
	if !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("err = %v, want ErrPreconditionFailed", err)
	}
	if !strings.Contains(err.Error(), "the index is not empty") {
		t.Fatalf("err = %v, want it to state the index fact", err)
	}
	if strings.Contains(err.Error(), "no longer empty") {
		t.Fatalf("err = %v, must not assert a transition the index check never observed", err)
	}
	if ok, _ := gitx.HasLocalBranch(context.Background(), repo.Dir, "close/checkout"); !ok {
		t.Fatal("close/checkout deleted by a refused Apply call")
	}
}

// TestApply_UnknownChoiceWithNoChoicesOffered is wave-review m1: a
// projection that offers nothing renders that fact, never the label
// "known choices:" with an empty list after it.
func TestApply_UnknownChoiceWithNoChoicesOffered(t *testing.T) {
	_, cfg := fixtureStore(t) // a healthy store: no ritual branch, no lock, nothing recognized
	p := Derive(mustGather(t, cfg, "spec/checkout"))
	if len(choiceIDs(p)) != 0 {
		t.Fatalf("fixture offers choices %v; this case needs a projection with none", choiceIDs(p))
	}

	_, err := Apply(context.Background(), cfg, "spec/checkout", "bogus:x", io.Discard)
	if !errors.Is(err, ErrUnknownChoice) {
		t.Fatalf("err = %v, want ErrUnknownChoice", err)
	}
	if !strings.Contains(err.Error(), "no choices are offered for spec/checkout") {
		t.Fatalf("err = %v, want it to state that nothing is offered", err)
	}
	if strings.Contains(err.Error(), "known choices") {
		t.Fatalf("err = %v, want no empty known-choices tail", err)
	}
}
