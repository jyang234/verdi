// diagnosisonly_test.go is the regression suite for R-RR3-26: a state
// the projection can still fully RECOGNIZE is never withheld because a
// fact it needs only to DESCRIBE or DISAMBIGUATE could not be observed.
//
// The fix for the owner risk review's F2 made every recognizer touching
// the two working-tree listings fail closed, which was right where the
// listing answers the recognition question itself
// (recognizeScaffoldUnstaged, stagedClosureSpecName) and wrong for
// archive-move-uncommitted, whose state is read off disk and HEAD alone:
// there the unobserved index only stops it being told apart from
// artifacts-staged-uncommitted, and the unobserved store prefix only
// stops its manual commands being named in git's coordinates. Silence
// turned a described residue into `exit 0, nothing recognized`
// (readiness-recovery-v2 ac-8, co-6; guided-lifecycle-governance-v3
// DC-13 — "diagnosis only", not nothing).
package recovery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// breakIndex truncates repo's .git/index so every status/index read git
// makes against it fails — the owner risk review's own F2 reproduction.
func breakIndex(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".git", "index"), []byte("broken index\n"), 0o644); err != nil {
		t.Fatalf("truncating .git/index: %v", err)
	}
}

// stateByCode returns p's one state carrying code, whatever its target.
func stateByCode(t *testing.T, p Projection, code StateCode) RecognizedState {
	t.Helper()
	var found []RecognizedState
	for _, s := range p.States {
		if s.Code == code {
			found = append(found, s)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%d %s states in the projection, want exactly one: %+v", len(found), code, p.States)
	}
	return found[0]
}

// uncertaintyNaming returns s's first uncertainty whose text carries
// want.
func uncertaintyNaming(t *testing.T, s RecognizedState, want string) Uncertainty {
	t.Helper()
	for _, u := range s.Uncertainties {
		if strings.Contains(u.Text, want) {
			return u
		}
	}
	t.Fatalf("no uncertainty of %s names %q: %+v", s.Code, want, s.Uncertainties)
	return Uncertainty{}
}

// TestDerive_ArchiveMoveUncommitted_UnobservedIndexIsDiagnosedNotWithheld
// is the re-review's own reproduction: a real interrupted close whose
// spec directory was moved to the archive zone on disk and never staged,
// in a repository whose index git cannot read, with NO close/<name>
// branch to carry the diagnosis instead. Every fact the state is
// recognized from (disk, and HEAD's two zone trees, which read no index)
// is available, so the state is EMITTED — with the index named as the
// unavailable fact, the witness that would settle it, and no choice at
// all, because the disambiguation from artifacts-staged-uncommitted is
// exactly what is missing.
func TestDerive_ArchiveMoveUncommitted_UnobservedIndexIsDiagnosedNotWithheld(t *testing.T) {
	repo, cfg := fixtureStore(t)
	moveArchiveUncommitted(t, repo)

	// The same fixture with a healthy index: the state fires and carries
	// its executable-free choice. Without this, every assertion below
	// could pass for the wrong reason.
	healthy := Derive(mustGather(t, cfg, "spec/checkout"))
	if len(stateByCode(t, healthy, StateArchiveMoveUncommitted).Choices) != 1 {
		t.Fatalf("fixture does not offer the archive-move advice over a healthy index: %+v", healthy.States)
	}

	breakIndex(t, repo.Dir)

	f := mustGather(t, cfg, "spec/checkout") // co-6: a failed fact is disclosed, never fatal
	if f.StagedPathsObserved {
		t.Fatal("StagedPathsObserved = true over an index git cannot read")
	}
	if !f.RepoPrefixObserved {
		t.Fatal("RepoPrefixObserved = false: this fixture's prefix is readable, so the index guard is what is under test")
	}

	p := Derive(f)
	mustValidate(t, p)
	if !p.Recognized() {
		t.Fatalf("Recognized() = false: a fully observed residue reports nothing (exit 0): %+v", p)
	}
	s := stateByCode(t, p, StateArchiveMoveUncommitted)
	if len(s.Choices) != 0 {
		t.Fatalf("Choices = %+v, want none: no command can be named correctly without the index", s.Choices)
	}
	u := uncertaintyNaming(t, s, "staged-path listing could not be observed")
	if !strings.Contains(u.Text, string(StateArtifactsStagedUncommitted)) {
		t.Fatalf("uncertainty %q does not say which state this cannot be told apart from", u.Text)
	}
	if u.Witness == "" {
		t.Fatalf("uncertainty %+v carries no witness that would settle the unavailable fact", u)
	}
	for _, fact := range s.Facts {
		if !strings.Contains(fact, ".verdi/specs/") {
			t.Fatalf("fact %q does not name the zone path it was observed at", fact)
		}
	}
}

// TestDerive_ArchiveMoveUncommitted_UnobservedIndex_AmbiguityPointerResolves
// is N1a: recognizeEmptyBranchCut tells the operator to "resolve the
// archive-move-uncommitted state first", so that state must actually be
// in the projection it points into (2B-F2's reciprocal-naming contract).
// With the index unobserved it used to name a state nothing emitted.
func TestDerive_ArchiveMoveUncommitted_UnobservedIndex_AmbiguityPointerResolves(t *testing.T) {
	repo, cfg := fixtureStore(t)
	cutEmptyBranch(t, repo, "close/checkout")
	moveArchiveUncommitted(t, repo)
	breakIndex(t, repo.Dir)

	p := Derive(mustGather(t, cfg, "spec/checkout"))
	mustValidate(t, p)

	cut, ok := stateFor(p.States, StateEmptyBranchCut, "close/checkout")
	if !ok {
		t.Fatalf("no empty-branch-cut state for close/checkout: %+v", p.States)
	}
	pointer := uncertaintyNaming(t, cut, string(StateArchiveMoveUncommitted))
	if pointer.Witness == "" {
		t.Fatalf("ambiguity uncertainty %+v carries no witness", pointer)
	}
	move := stateByCode(t, p, StateArchiveMoveUncommitted)
	if len(move.Choices) != 0 {
		t.Fatalf("Choices = %+v, want none over an unobserved index", move.Choices)
	}
	// The reciprocal half: the archive-move state names the empty cut
	// back, so an operator reading either one alone learns of the other.
	uncertaintyNaming(t, move, string(StateEmptyBranchCut))
}
