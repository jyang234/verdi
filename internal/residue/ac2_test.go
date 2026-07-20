package residue

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// TestScan_AC2_RitualIncompleteAndSupersededElsewhere is
// obligation/closure-hygiene--ac-2--behavioral's own fixture: two unmerged
// close/* branches, close/alpha (tip does NOT yet carry the archive move
// — ritual-incomplete) and close/beta (tip DOES carry it, and the default
// branch, separately, ALSO already carries it through its own independent
// commit history — superseded-elsewhere). Asserts both classifications
// appear and the run is flagged; then, with close/alpha removed so only
// the superseded-elsewhere branch remains, asserts the run is unflagged
// despite close/beta's entry still being present.
func TestScan_AC2_RitualIncompleteAndSupersededElsewhere(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore":                 "data/\n",
			".verdi/specs/active/alpha/spec.md": storySpecMD("alpha", "accepted-pending-build", "feature-x"),
			".verdi/specs/active/beta/spec.md":  storySpecMD("beta", "accepted-pending-build", "feature-x"),
		},
		Message: "seed alpha and beta stories",
	}})
	root := repo.Dir

	// close/alpha: cut, but NEVER runs the archive move — ritual-incomplete.
	cutCloseBranch(t, root, "alpha")
	runGit(t, root, "commit", "--quiet", "--allow-empty", "-m", "close/alpha: cut, but the ritual never finished")
	checkoutMain(t, root)

	// close/beta: its own tip DOES perform the archive move...
	cutCloseBranch(t, root, "beta")
	runCloseRitualArchiveCommit(t, root, "beta", "close: archive spec/beta")
	checkoutMain(t, root)
	// ...and, SEPARATELY, main independently ALSO reaches archive/beta,
	// through a DIFFERENT commit (superseded-elsewhere's own shape).
	runCloseRitualArchiveCommit(t, root, "beta", "close: archive spec/beta (independent, main-side)")

	got, err := Scan(context.Background(), root, "main")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	byName := make(map[string]CloseBranch, len(got.CloseBranches))
	for _, cb := range got.CloseBranches {
		byName[cb.Name] = cb
	}
	alpha, ok := byName["alpha"]
	if !ok {
		t.Fatalf("Scan.CloseBranches missing close/alpha: %+v", got.CloseBranches)
	}
	if alpha.Class != RitualIncomplete {
		t.Errorf("close/alpha Class = %v, want RitualIncomplete", alpha.Class)
	}
	beta, ok := byName["beta"]
	if !ok {
		t.Fatalf("Scan.CloseBranches missing close/beta: %+v", got.CloseBranches)
	}
	if beta.Class != SupersededElsewhere {
		t.Errorf("close/beta Class = %v, want SupersededElsewhere", beta.Class)
	}
	if !got.Flagged() {
		t.Fatal("Scan.Flagged() = false, want true (close/alpha is ritual-incomplete)")
	}

	// Remove close/alpha; only the superseded-elsewhere branch remains.
	ctx := context.Background()
	runGit(t, root, "branch", "-D", "close/alpha")

	got2, err := Scan(ctx, root, "main")
	if err != nil {
		t.Fatalf("Scan (after removing close/alpha): %v", err)
	}
	found := false
	for _, cb := range got2.CloseBranches {
		if cb.Name == "alpha" {
			t.Fatalf("Scan.CloseBranches still contains alpha after its branch was deleted: %+v", got2.CloseBranches)
		}
		if cb.Name == "beta" {
			found = true
			if cb.Class != SupersededElsewhere {
				t.Errorf("close/beta Class = %v, want SupersededElsewhere (unchanged)", cb.Class)
			}
		}
	}
	if !found {
		t.Fatal("Scan.CloseBranches missing close/beta after removing close/alpha")
	}
	if got2.Flagged() {
		t.Fatal("Scan.Flagged() = true, want false (only a superseded-elsewhere branch remains)")
	}
}

// TestScan_AC2_GREEN_NoUnmergedCloseBranches asserts a fixture with NO
// unmerged close/* branches at all reports a clean CloseBranches list.
func TestScan_AC2_GREEN_NoUnmergedCloseBranches(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/.gitignore": "data/\n"},
		Message: "no close/* branches whatsoever",
	}})

	got, err := Scan(context.Background(), repo.Dir, "main")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(got.CloseBranches) != 0 {
		t.Fatalf("Scan.CloseBranches = %+v, want empty", got.CloseBranches)
	}
	if got.Flagged() {
		t.Fatal("Scan.Flagged() = true, want false")
	}
}

// TestScan_AC2_MergedCloseBranchNeverClassified proves a MERGED close/*
// branch is excluded from AC-2's report entirely, matching AC-2's own
// "restricts classification to the subset unmerged" scope.
func TestScan_AC2_MergedCloseBranchNeverClassified(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore":                 "data/\n",
			".verdi/specs/active/gamma/spec.md": storySpecMD("gamma", "accepted-pending-build", "feature-x"),
		},
		Message: "seed gamma story",
	}})
	root := repo.Dir
	ctx := context.Background()

	cutCloseBranch(t, root, "gamma")
	runCloseRitualArchiveCommit(t, root, "gamma", "close: archive spec/gamma")
	checkoutMain(t, root)
	runGit(t, root, "merge", "--quiet", "--no-ff", "-m", "merge close/gamma", "close/gamma")

	got, err := Scan(ctx, root, "main")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	for _, cb := range got.CloseBranches {
		if cb.Name == "gamma" {
			t.Fatalf("Scan.CloseBranches contains gamma, a MERGED close/* branch that must be excluded: %+v", got.CloseBranches)
		}
	}
}
