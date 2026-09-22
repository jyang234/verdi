// cleantree_test.go is the regression suite for the unwind's own
// clean-tree proof (owner risk review F1/F2, spec/readiness-recovery-v2
// ac-9/ac-10, guided-lifecycle-governance-v3 DC-13).
//
// Two ways the old proof answered "clean" without having proved it: it
// read `git status --porcelain`'s single bool, which an ordinary display
// setting (status.showUntrackedFiles=no) makes under-report; and it read
// a FAILED listing's success-shaped zero value as an empty one. Both are
// covered here at derive time and at the execution re-proof, over real
// repositories.
package recovery

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
)

const unwindCheckoutChoice = "unwind-branch-cut:close/checkout"

// TestProveCleanTree is the shared predicate's own table: an empty index
// and an unchanged working tree, BOTH successfully observed, is the only
// combination that proves the unwind's starting point.
func TestProveCleanTree(t *testing.T) {
	for _, tc := range []struct {
		name           string
		facts          func(f *Facts)
		wantProven     bool
		wantSubstrings []string
	}{
		{
			name:       "both listings observed and empty",
			facts:      func(f *Facts) {},
			wantProven: true,
		},
		{
			name: "a non-empty index",
			facts: func(f *Facts) {
				f.StagedPaths = []string{"notes/one.md"}
			},
			wantSubstrings: []string{"the index is not empty", "notes/one.md"},
		},
		{
			name: "a changed working tree",
			facts: func(f *Facts) {
				f.WorktreeChangedPaths = []string{"unfinished.txt"}
			},
			wantSubstrings: []string{"the working tree is not clean", "unfinished.txt"},
		},
		{
			name: "the staged listing was never observed",
			facts: func(f *Facts) {
				f.StagedPathsObserved = false
			},
			wantSubstrings: []string{"staged-path listing could not be observed"},
		},
		{
			name: "the working-tree listing was never observed",
			facts: func(f *Facts) {
				f.WorktreeChangedObserved = false
			},
			wantSubstrings: []string{"changed-path listing could not be observed"},
		},
		{
			name: "neither listing was observed",
			facts: func(f *Facts) {
				f.StagedPathsObserved = false
				f.WorktreeChangedObserved = false
			},
			wantSubstrings: []string{"staged-path listing could not be observed", "changed-path listing could not be observed"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := baseFacts()
			tc.facts(&f)
			proof := proveCleanTree(f)
			if proof.Proven() != tc.wantProven {
				t.Fatalf("proveCleanTree(%+v).Proven() = %v, want %v (%+v)", f, proof.Proven(), tc.wantProven, proof)
			}
			refusal := cleanTreeRefusal(f)
			if tc.wantProven {
				if refusal != "" {
					t.Fatalf("cleanTreeRefusal = %q, want none over a proved clean tree", refusal)
				}
				return
			}
			for _, want := range tc.wantSubstrings {
				if !strings.Contains(refusal, want) {
					t.Fatalf("cleanTreeRefusal = %q, want it to carry %q", refusal, want)
				}
			}
			if strings.Contains(refusal, "no longer") {
				t.Fatalf("cleanTreeRefusal = %q: it must state what it observed, never a transition it did not witness", refusal)
			}
		})
	}
}

// TestUnwind_UntrackedFileHiddenByStatusConfig is F1 over a real
// repository: `status.showUntrackedFiles=no` hides an untracked file
// from `git status --porcelain`, which is what Facts.Dirty was read
// from, while the changed-path listing (which passes
// --untracked-files=all and so overrides the setting) still names it.
// The clean-tree proof must be configuration-independent in both
// places — no executable unwind at derive, and a refusal at the
// execution re-proof.
func TestUnwind_UntrackedFileHiddenByStatusConfig(t *testing.T) {
	repo, cfg := fixtureStore(t)
	cutEmptyBranch(t, repo, "close/checkout")

	// The choice as it is built over a provably clean tree: the input the
	// re-proof is handed.
	clean := mustGather(t, cfg, "spec/checkout")
	choice, _, offered := findChoice(Derive(clean), unwindCheckoutChoice)
	if !offered {
		t.Fatalf("fixture does not offer %s over a clean tree; every assertion below would pass for the wrong reason", unwindCheckoutChoice)
	}

	runGit(t, repo.Dir, "config", "status.showUntrackedFiles", "no")
	if err := os.WriteFile(filepath.Join(repo.Dir, "unfinished.txt"), []byte("operator work\n"), 0o644); err != nil {
		t.Fatalf("writing unfinished.txt: %v", err)
	}

	f := mustGather(t, cfg, "spec/checkout")
	if !containsString(f.WorktreeChangedPaths, "unfinished.txt") {
		t.Fatalf("WorktreeChangedPaths = %v, want the untracked file --untracked-files=all reports", f.WorktreeChangedPaths)
	}

	p := Derive(f)
	mustValidate(t, p)
	s, ok := stateFor(p.States, StateEmptyBranchCut, "close/checkout")
	if !ok {
		t.Fatalf("no empty-branch-cut state for close/checkout: %+v", p.States)
	}
	if len(s.Choices) != 0 {
		t.Fatalf("Choices = %+v, want none: an untracked file a display setting hides is still uncommitted work", s.Choices)
	}
	u, ok := uncleanTreeUncertaintyOf(s)
	if !ok {
		t.Fatalf("Uncertainties = %+v, want the clean-tree uncertainty", s.Uncertainties)
	}
	if !strings.Contains(u.Text, "unfinished.txt") {
		t.Fatalf("uncertainty %q does not name the changed path", u.Text)
	}
	if !strings.Contains(u.Witness, "--untracked-files=all") {
		t.Fatalf("witness %q must name the flag that makes the answer configuration-independent", u.Witness)
	}

	if refusal := reproveUnwind(clean, choice, f); refusal == "" {
		t.Fatal("reproveUnwind returned no refusal: the execution re-proof admitted an executor over a tree it cannot prove clean")
	} else if !strings.Contains(refusal, "the working tree is not clean") {
		t.Fatalf("reproveUnwind refusal = %q, want it to state the working-tree fact", refusal)
	}
	if ok, _ := gitx.HasLocalBranch(context.Background(), repo.Dir, "close/checkout"); !ok {
		t.Fatal("close/checkout no longer exists: a read-only projection mutated the repository")
	}
}

// TestUnwind_UnreadableIndexIsNotProofOfCleanliness is F2 (DC-13): an
// index git itself cannot read fails both listings. Gather still
// succeeds and discloses — but the failure must be carried as its own
// "unobserved" validity, never as the success-shaped empty listing that
// reads as proof of an empty index and a clean tree.
func TestUnwind_UnreadableIndexIsNotProofOfCleanliness(t *testing.T) {
	repo, cfg := fixtureStore(t)
	cutEmptyBranch(t, repo, "close/checkout")

	healthy := mustGather(t, cfg, "spec/checkout")
	choice, _, offered := findChoice(Derive(healthy), unwindCheckoutChoice)
	if !offered {
		t.Fatalf("fixture does not offer %s over a healthy index; every assertion below would pass for the wrong reason", unwindCheckoutChoice)
	}

	if err := os.WriteFile(filepath.Join(repo.Dir, ".git", "index"), []byte("broken index\n"), 0o644); err != nil {
		t.Fatalf("truncating .git/index: %v", err)
	}

	f := mustGather(t, cfg, "spec/checkout") // co-6: a failed fact is disclosed, never fatal
	if f.StagedPathsObserved {
		t.Fatal("StagedPathsObserved = true over an index git cannot read")
	}
	if f.WorktreeChangedObserved {
		t.Fatal("WorktreeChangedObserved = true over an index git cannot read")
	}
	if len(f.StagedPaths) != 0 || len(f.WorktreeChangedPaths) != 0 {
		t.Fatalf("StagedPaths = %v, WorktreeChangedPaths = %v: a failed read must leave no values behind", f.StagedPaths, f.WorktreeChangedPaths)
	}
	if !containsSubstring(f.Disclosures, "could not list staged paths") {
		t.Fatalf("Disclosures = %v, want the failed staged-listing disclosure", f.Disclosures)
	}

	p := Derive(f)
	mustValidate(t, p)
	s, ok := stateFor(p.States, StateEmptyBranchCut, "close/checkout")
	if !ok {
		t.Fatalf("no empty-branch-cut state for close/checkout: %+v", p.States)
	}
	if len(s.Choices) != 0 {
		t.Fatalf("Choices = %+v, want none: an unobserved listing is not proof of an empty one", s.Choices)
	}
	u, ok := uncleanTreeUncertaintyOf(s)
	if !ok {
		t.Fatalf("Uncertainties = %+v, want the clean-tree uncertainty", s.Uncertainties)
	}
	for _, want := range []string{"staged-path listing could not be observed", "changed-path listing could not be observed"} {
		if !strings.Contains(u.Text, want) {
			t.Fatalf("uncertainty %q does not name the unavailable fact %q", u.Text, want)
		}
	}
	if u.Witness == "" {
		t.Fatalf("uncertainty %+v carries no witness that would settle the unavailable fact", u)
	}

	refusal := reproveUnwind(healthy, choice, f)
	if refusal == "" {
		t.Fatal("reproveUnwind returned no refusal: the execution re-proof admitted an executor over listings it never observed")
	}
	if !strings.Contains(refusal, "could not be observed") {
		t.Fatalf("reproveUnwind refusal = %q, want it to name the unobserved fact", refusal)
	}
	if ok, _ := gitx.HasLocalBranch(context.Background(), repo.Dir, "close/checkout"); !ok {
		t.Fatal("close/checkout no longer exists: a refused re-proof mutated the repository")
	}
}
