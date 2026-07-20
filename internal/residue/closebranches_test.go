package residue

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

func TestCloseBranchName(t *testing.T) {
	cases := []struct {
		branch   string
		wantName string
		wantOK   bool
	}{
		{"close/widget", "widget", true},
		{"close/showcase-corpus-renovation", "showcase-corpus-renovation", true},
		{"close/", "", false},
		{"close", "", false},
		{"design/widget", "", false},
		{"main", "", false},
		{"closewidget", "", false},
	}
	for _, c := range cases {
		t.Run(c.branch, func(t *testing.T) {
			name, ok := closeBranchName(c.branch)
			if name != c.wantName || ok != c.wantOK {
				t.Fatalf("closeBranchName(%q) = (%q, %v), want (%q, %v)", c.branch, name, ok, c.wantName, c.wantOK)
			}
		})
	}
}

func TestCloseClassification_String(t *testing.T) {
	if got := RitualIncomplete.String(); got != "ritual-incomplete" {
		t.Fatalf("RitualIncomplete.String() = %q, want ritual-incomplete", got)
	}
	if got := SupersededElsewhere.String(); got != "superseded-elsewhere" {
		t.Fatalf("SupersededElsewhere.String() = %q, want superseded-elsewhere", got)
	}
	if got := CloseClassification(99).String(); got != "unknown" {
		t.Fatalf("CloseClassification(99).String() = %q, want unknown", got)
	}
}

func buildArchiveExistsAtRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore":                     "data/\n",
			".verdi/specs/archive/foo/spec.md":      storySpecMD("foo", "closed", "feature-x"),
			".verdi/specs/active/untouched/spec.md": storySpecMD("untouched", "accepted-pending-build", "feature-x"),
		},
		Message: "seed one archived and one active spec",
	}})
}

func TestArchiveExistsAt_Happy(t *testing.T) {
	repo := buildArchiveExistsAtRepo(t)
	ctx := context.Background()

	got, err := archiveExistsAt(ctx, repo.Dir, repo.Head, "foo")
	if err != nil {
		t.Fatalf("archiveExistsAt(foo): %v", err)
	}
	if !got {
		t.Fatal("archiveExistsAt(foo) = false, want true (archive/foo/spec.md exists at HEAD)")
	}
}

func TestArchiveExistsAt_NegativePath_NotAnError(t *testing.T) {
	repo := buildArchiveExistsAtRepo(t)
	ctx := context.Background()

	got, err := archiveExistsAt(ctx, repo.Dir, repo.Head, "bar")
	if err != nil {
		t.Fatalf("archiveExistsAt(bar): unexpected error: %v", err)
	}
	if got {
		t.Fatal("archiveExistsAt(bar) = true, want false (no archive/bar/spec.md exists)")
	}

	// "untouched" is only ACTIVE, never archived — archiveExistsAt must
	// not be fooled by the active-zone copy existing.
	got, err = archiveExistsAt(ctx, repo.Dir, repo.Head, "untouched")
	if err != nil {
		t.Fatalf("archiveExistsAt(untouched): unexpected error: %v", err)
	}
	if got {
		t.Fatal("archiveExistsAt(untouched) = true, want false (only the active-zone copy exists)")
	}
}

func TestArchiveExistsAt_Negative_BogusRef(t *testing.T) {
	repo := buildArchiveExistsAtRepo(t)
	if _, err := archiveExistsAt(context.Background(), repo.Dir, "not-a-real-ref", "foo"); err == nil {
		t.Fatal("archiveExistsAt(bogus ref): want error, got nil")
	}
}

// buildScanCloseBranchesFixture builds a repo exercising every reachable
// scanCloseBranches outcome in one pass: close/widget (ritual-incomplete
// via AC-1 pattern (a)'s own shape — archived on its own tip, default
// branch does not have it yet), close/already-done (superseded-elsewhere
// — archived on both its own tip AND, separately, on main through an
// independent commit), close/fresh (ritual-incomplete — never even ran
// the archive step), and close/merged-in (merged into main — must be
// excluded from the result entirely).
func buildScanCloseBranchesFixture(t *testing.T) (root string) {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore":                        "data/\n",
			".verdi/specs/active/widget/spec.md":       storySpecMD("widget", "accepted-pending-build", "feature-x"),
			".verdi/specs/active/already-done/spec.md": storySpecMD("already-done", "accepted-pending-build", "feature-x"),
			".verdi/specs/active/fresh/spec.md":        storySpecMD("fresh", "accepted-pending-build", "feature-x"),
			".verdi/specs/active/merged-in/spec.md":    storySpecMD("merged-in", "accepted-pending-build", "feature-x"),
		},
		Message: "seed four active stories",
	}})
	root = repo.Dir

	cutCloseBranch(t, root, "widget")
	runCloseRitualArchiveCommit(t, root, "widget", "close: archive spec/widget")
	checkoutMain(t, root)

	cutCloseBranch(t, root, "already-done")
	runCloseRitualArchiveCommit(t, root, "already-done", "close: archive spec/already-done")
	checkoutMain(t, root)

	// main's OWN, independent archive of "already-done" — a DIFFERENT
	// commit (distinguished by message) than close/already-done's own
	// (the "superseded elsewhere" shape: someone else already closed it
	// through a different commit history).
	runCloseRitualArchiveCommit(t, root, "already-done", "close: archive spec/already-done (independent, main-side)")

	cutCloseBranch(t, root, "merged-in")
	runCloseRitualArchiveCommit(t, root, "merged-in", "close: archive spec/merged-in")
	checkoutMain(t, root)
	runGit(t, root, "merge", "--quiet", "--no-ff", "-m", "merge close/merged-in", "close/merged-in")

	cutCloseBranch(t, root, "fresh")
	// A trivial, spec-unrelated commit so close/fresh's tip is provably
	// distinct from (and unmerged into) main — never touches the archive
	// move at all (this branch never even ran that step).
	if err := os.WriteFile(filepath.Join(root, "NOTES.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "wip on close/fresh")
	checkoutMain(t, root)

	return root
}

func TestScanCloseBranches_Happy(t *testing.T) {
	root := buildScanCloseBranchesFixture(t)
	ctx := context.Background()

	defaultTip, err := gitx.RevParse(ctx, root, "main")
	if err != nil {
		t.Fatalf("RevParse(main): %v", err)
	}

	got, err := scanCloseBranches(ctx, root, defaultTip, nil)
	if err != nil {
		t.Fatalf("scanCloseBranches: %v", err)
	}

	byName := make(map[string]CloseBranch, len(got))
	for _, cb := range got {
		byName[cb.Name] = cb
	}
	if _, ok := byName["merged-in"]; ok {
		t.Fatalf("scanCloseBranches included merged-in, which is MERGED and must be excluded entirely: %+v", got)
	}
	if len(got) != 3 {
		t.Fatalf("scanCloseBranches = %+v, want exactly 3 unmerged close/* branches", got)
	}

	widget := byName["widget"]
	if !widget.ArchivedOnOwnTip {
		t.Fatalf("widget.ArchivedOnOwnTip = false, want true (its own tip performed the archive move)")
	}
	if widget.Class != RitualIncomplete {
		t.Fatalf("widget.Class = %v, want RitualIncomplete (AC-1 pattern (a)'s own shape)", widget.Class)
	}
	if widget.Branch != "close/widget" {
		t.Fatalf("widget.Branch = %q, want close/widget", widget.Branch)
	}

	already := byName["already-done"]
	if !already.ArchivedOnOwnTip {
		t.Fatal("already-done.ArchivedOnOwnTip = false, want true")
	}
	if already.Class != SupersededElsewhere {
		t.Fatalf("already-done.Class = %v, want SupersededElsewhere (also archived on main, independently)", already.Class)
	}

	fresh := byName["fresh"]
	if fresh.ArchivedOnOwnTip {
		t.Fatal("fresh.ArchivedOnOwnTip = true, want false (never ran the archive step)")
	}
	if fresh.Class != RitualIncomplete {
		t.Fatalf("fresh.Class = %v, want RitualIncomplete (default branch does not have archive/fresh either)", fresh.Class)
	}
}

func TestScanCloseBranches_Negative_NotARepo(t *testing.T) {
	if _, err := scanCloseBranches(context.Background(), t.TempDir(), "HEAD", nil); err == nil {
		t.Fatal("scanCloseBranches outside a repo: want error, got nil")
	}
}

func TestScanCloseBranches_Negative_BogusDefaultTip(t *testing.T) {
	root := buildScanCloseBranchesFixture(t)
	if _, err := scanCloseBranches(context.Background(), root, "not-a-real-commit", nil); err == nil {
		t.Fatal("scanCloseBranches(bogus default tip): want error, got nil")
	}
}

// TestScanCloseBranches_DC2_SupersededNameExcluded proves dc-2's own
// "AC-1/AC-2" grouping: a close/<name> branch whose <name> is passed in
// supersededNames is excluded from the result entirely — neither
// ritual-incomplete nor superseded-elsewhere — even though its own tip
// and the default branch's tip would otherwise make it classify exactly
// like the "widget" (ritual-incomplete) case.
func TestScanCloseBranches_DC2_SupersededNameExcluded(t *testing.T) {
	root := buildScanCloseBranchesFixture(t)
	ctx := context.Background()
	defaultTip, err := gitx.RevParse(ctx, root, "main")
	if err != nil {
		t.Fatal(err)
	}

	got, err := scanCloseBranches(ctx, root, defaultTip, map[string]bool{"widget": true})
	if err != nil {
		t.Fatalf("scanCloseBranches: %v", err)
	}
	for _, cb := range got {
		if cb.Name == "widget" {
			t.Fatalf("scanCloseBranches still classified widget despite supersededNames excluding it: %+v", got)
		}
	}
	if len(got) != 2 {
		t.Fatalf("scanCloseBranches = %+v, want exactly 2 (already-done, fresh — widget excluded)", got)
	}
}

func TestScanCloseBranches_Green_NoCloseBranches(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore":                  "data/\n",
			".verdi/specs/active/widget/spec.md": storySpecMD("widget", "accepted-pending-build", "feature-x"),
		},
		Message: "no close/* branches at all",
	}})
	ctx := context.Background()

	got, err := scanCloseBranches(ctx, repo.Dir, repo.Head, nil)
	if err != nil {
		t.Fatalf("scanCloseBranches: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("scanCloseBranches = %+v, want empty (no close/* branches)", got)
	}
}
