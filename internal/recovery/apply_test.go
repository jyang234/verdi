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
