package residue

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/wtmanager"
)

func buildMergedBranchesFixture(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/.gitignore": "data/\n"},
		Message: "root",
	}})
	ctx := context.Background()

	if err := gitx.CheckoutNewBranch(ctx, repo.Dir, "merged-a"); err != nil {
		t.Fatalf("CheckoutNewBranch(merged-a): %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo.Dir, "add", "-A")
	runGit(t, repo.Dir, "commit", "--quiet", "-m", "merged-a work")
	checkoutMain(t, repo.Dir)
	runGit(t, repo.Dir, "merge", "--quiet", "--no-ff", "-m", "merge merged-a", "merged-a")

	if err := gitx.CheckoutNewBranch(ctx, repo.Dir, "unmerged-b"); err != nil {
		t.Fatalf("CheckoutNewBranch(unmerged-b): %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo.Dir, "add", "-A")
	runGit(t, repo.Dir, "commit", "--quiet", "-m", "unmerged-b work")
	checkoutMain(t, repo.Dir)

	return repo
}

func TestScanMergedBranches_Happy(t *testing.T) {
	repo := buildMergedBranchesFixture(t)
	ctx := context.Background()

	// repo.Head is BEFORE the merge commit (fixturegit.Build only stamps
	// the layers it built) — resolve main's REAL current tip instead.
	mainTip, err := gitx.RevParse(ctx, repo.Dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	got, err := scanMergedBranches(ctx, repo.Dir, "main", mainTip)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "merged-a" {
		t.Fatalf("scanMergedBranches = %v, want exactly [merged-a] (main excluded, unmerged-b excluded)", got)
	}
}

func TestScanMergedBranches_Negative_NotARepo(t *testing.T) {
	if _, err := scanMergedBranches(context.Background(), t.TempDir(), "main", "HEAD"); err == nil {
		t.Fatal("scanMergedBranches outside a repo: want error, got nil")
	}
}

func TestScanMergedBranches_Negative_BogusDefaultTip(t *testing.T) {
	repo := buildMergedBranchesFixture(t)
	if _, err := scanMergedBranches(context.Background(), repo.Dir, "main", "not-a-real-commit"); err == nil {
		t.Fatal("scanMergedBranches(bogus default tip): want error, got nil")
	}
}

func TestLooksLikePrimaryWorktree(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/.gitignore": "data/\n"},
		Message: "root",
	}})

	if !looksLikePrimaryWorktree(gitx.WorktreeEntry{Path: repo.Dir}) {
		t.Fatal("looksLikePrimaryWorktree(primary checkout) = false, want true (.git is a directory there)")
	}
	if looksLikePrimaryWorktree(gitx.WorktreeEntry{Path: t.TempDir()}) {
		t.Fatal("looksLikePrimaryWorktree(no .git at all) = true, want false")
	}
}

func TestIsUnderRoot(t *testing.T) {
	cases := []struct {
		path, root string
		want       bool
	}{
		{"/store/.verdi/data/worktrees/x", "/store/.verdi/data/worktrees", true},
		{"/store/.verdi/data/worktrees", "/store/.verdi/data/worktrees", false}, // equal, not a descendant
		{"/elsewhere/x", "/store/.verdi/data/worktrees", false},
		{"/store/.verdi/data/worktrees-other/x", "/store/.verdi/data/worktrees", false},
	}
	for _, c := range cases {
		if got := isUnderRoot(c.path, c.root); got != c.want {
			t.Errorf("isUnderRoot(%q, %q) = %v, want %v", c.path, c.root, got, c.want)
		}
	}
}

// buildWorktreeSurveyFixture builds a fixturegit repo with one managed
// worktree (via the real wtmanager.EnsureWorktree path, on a design
// branch), one unmanaged worktree on a MERGED branch, one unmanaged
// worktree on an UNMERGED branch (left dirty, proving the dirty signal is
// live), and one unmanaged worktree with a detached HEAD at a commit that
// IS an ancestor of main's tip — the exact four-shape mix ac-3's
// behavioral obligation asks for.
func buildWorktreeSurveyFixture(t *testing.T) (root string, unmergedWTPath string) {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/.gitignore": "data/\n"},
		Message: "root",
	}})
	root = repo.Dir
	ctx := context.Background()
	rootCommit := repo.Head

	// Managed: a real design/x worktree cut via the production entry point.
	if err := gitx.CheckoutNewBranch(ctx, root, "design/x"); err != nil {
		t.Fatalf("CheckoutNewBranch(design/x): %v", err)
	}
	checkoutMain(t, root)
	if _, err := wtmanager.EnsureWorktree(ctx, root, "design/x"); err != nil {
		t.Fatalf("EnsureWorktree(design/x): %v", err)
	}

	// Unmanaged + merged: a branch merged into main, worktree OUTSIDE the
	// managed root entirely (a sibling temp dir, mirroring how real
	// verdi-wt/ orchestration worktrees sit outside the repo altogether).
	if err := gitx.CheckoutNewBranch(ctx, root, "merged-elsewhere"); err != nil {
		t.Fatalf("CheckoutNewBranch(merged-elsewhere): %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "merged.txt"), []byte("m\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "merged-elsewhere work")
	checkoutMain(t, root)
	runGit(t, root, "merge", "--quiet", "--no-ff", "-m", "merge merged-elsewhere", "merged-elsewhere")

	mergedWTPath := filepath.Join(t.TempDir(), "merged-wt")
	if err := gitx.WorktreeAdd(ctx, root, mergedWTPath, "merged-elsewhere"); err != nil {
		t.Fatalf("WorktreeAdd(merged-elsewhere): %v", err)
	}

	// Unmanaged + unmerged, left dirty.
	if err := gitx.CheckoutNewBranch(ctx, root, "unmerged-elsewhere"); err != nil {
		t.Fatalf("CheckoutNewBranch(unmerged-elsewhere): %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "unmerged.txt"), []byte("u\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "unmerged-elsewhere work")
	checkoutMain(t, root)

	unmergedWTPath = filepath.Join(t.TempDir(), "unmerged-wt")
	if err := gitx.WorktreeAdd(ctx, root, unmergedWTPath, "unmerged-elsewhere"); err != nil {
		t.Fatalf("WorktreeAdd(unmerged-elsewhere): %v", err)
	}
	if err := os.WriteFile(filepath.Join(unmergedWTPath, "wip.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Unmanaged + detached HEAD, at the ROOT commit (an ancestor of main's
	// later, moved-on tip) — no branch name at all.
	detachedWTPath := filepath.Join(t.TempDir(), "detached-wt")
	if _, err := exec.Command("git", "-C", root, "worktree", "add", "--detach", "--quiet", detachedWTPath, rootCommit).CombinedOutput(); err != nil {
		t.Fatalf("git worktree add --detach: %v", err)
	}

	return root, unmergedWTPath
}

func TestScanWorktrees_Happy(t *testing.T) {
	root, unmergedWTPath := buildWorktreeSurveyFixture(t)
	ctx := context.Background()

	defaultTip, err := gitx.RevParse(ctx, root, "main")
	if err != nil {
		t.Fatal(err)
	}

	got, err := scanWorktrees(ctx, root, defaultTip)
	if err != nil {
		t.Fatalf("scanWorktrees: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("scanWorktrees = %+v, want exactly 4 (primary excluded)", got)
	}

	byBranch := map[string]Worktree{}
	var detached *Worktree
	for i := range got {
		if got[i].Branch == "" {
			detached = &got[i]
			continue
		}
		byBranch[got[i].Branch] = got[i]
	}

	managed := byBranch["design/x"]
	if managed.Path == "" {
		t.Fatalf("scanWorktrees missing design/x entry: %+v", got)
	}
	if !managed.Managed {
		t.Fatal("design/x worktree Managed = false, want true (cut under wtmanager.WorktreesRoot)")
	}
	if managed.Dirty {
		t.Fatal("design/x worktree Dirty = true, want false (freshly cut)")
	}

	mergedEntry := byBranch["merged-elsewhere"]
	if mergedEntry.Path == "" {
		t.Fatalf("scanWorktrees missing merged-elsewhere entry: %+v", got)
	}
	if mergedEntry.Managed {
		t.Fatal("merged-elsewhere worktree Managed = true, want false (outside the managed root)")
	}
	if !mergedEntry.Merged {
		t.Fatal("merged-elsewhere worktree Merged = false, want true")
	}

	unmergedEntry := byBranch["unmerged-elsewhere"]
	if unmergedEntry.Path == "" {
		t.Fatalf("scanWorktrees missing unmerged-elsewhere entry: %+v", got)
	}
	if unmergedEntry.Managed {
		t.Fatal("unmerged-elsewhere worktree Managed = true, want false")
	}
	if unmergedEntry.Merged {
		t.Fatal("unmerged-elsewhere worktree Merged = true, want false")
	}
	if !unmergedEntry.Dirty {
		t.Fatal("unmerged-elsewhere worktree Dirty = false, want true (an uncommitted edit was made)")
	}
	if realOrSelfSurvey(unmergedEntry.Path) != realOrSelfSurvey(unmergedWTPath) {
		t.Fatalf("unmerged-elsewhere worktree Path = %q, want %q", unmergedEntry.Path, unmergedWTPath)
	}

	if detached == nil {
		t.Fatalf("scanWorktrees missing the detached-HEAD entry: %+v", got)
	}
	if detached.Branch != "" {
		t.Fatalf("detached entry Branch = %q, want empty", detached.Branch)
	}
	if detached.Commit == "" {
		t.Fatal("detached entry Commit is empty; want the checked-out commit disclosed")
	}
	if !detached.Merged {
		t.Fatal("detached entry Merged = false, want true (checked out at an ancestor of main's tip, resolved at the commit level)")
	}
	if detached.Managed {
		t.Fatal("detached entry Managed = true, want false")
	}
}

func TestScanWorktrees_NoWorktreesBeyondPrimary(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/.gitignore": "data/\n"},
		Message: "root",
	}})
	ctx := context.Background()

	got, err := scanWorktrees(ctx, repo.Dir, repo.Head)
	if err != nil {
		t.Fatalf("scanWorktrees: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("scanWorktrees = %+v, want empty (only the primary checkout exists)", got)
	}
}

func TestScanWorktrees_Negative_NotARepo(t *testing.T) {
	if _, err := scanWorktrees(context.Background(), t.TempDir(), "HEAD"); err == nil {
		t.Fatal("scanWorktrees outside a repo: want error, got nil")
	}
}

// TestScanWorktrees_StaleWorktreeDisclosedNotAborted is Defect 1's
// unit-level witness: a worktree registered then deleted from disk WITHOUT
// `git worktree remove` (git still lists it, marked prunable) is DISCLOSED
// with its clean state unresolvable — never an aborting operational error
// (AC-3(b): "disclosed rather than guessed when a worktree's state cannot
// be resolved"). Its merge state stays resolvable (checked in root against
// the porcelain-reported HEAD, which git still provides for a prunable
// entry), and git's own prunable reason is surfaced.
func TestScanWorktrees_StaleWorktreeDisclosedNotAborted(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/.gitignore": "data/\n"},
		Message: "root",
	}})
	root := repo.Dir
	ctx := context.Background()

	if err := gitx.CheckoutNewBranch(ctx, root, "stale-branch"); err != nil {
		t.Fatalf("CheckoutNewBranch(stale-branch): %v", err)
	}
	checkoutMain(t, root)
	staleWT := filepath.Join(t.TempDir(), "stale-wt")
	if err := gitx.WorktreeAdd(ctx, root, staleWT, "stale-branch"); err != nil {
		t.Fatalf("WorktreeAdd(stale-branch): %v", err)
	}
	if err := os.RemoveAll(staleWT); err != nil {
		t.Fatalf("RemoveAll(%s): %v", staleWT, err)
	}

	defaultTip, err := gitx.RevParse(ctx, root, "main")
	if err != nil {
		t.Fatal(err)
	}

	got, err := scanWorktrees(ctx, root, defaultTip)
	if err != nil {
		t.Fatalf("scanWorktrees aborted on a stale worktree: %v (want a disclosed entry, no error)", err)
	}
	if len(got) != 1 {
		t.Fatalf("scanWorktrees = %+v, want exactly 1 (the stale worktree, primary excluded)", got)
	}
	wt := got[0]
	if !wt.DirtyUnresolved {
		t.Error("stale worktree DirtyUnresolved = false, want true (`git status` cannot run in a deleted directory)")
	}
	if wt.Dirty {
		t.Error("stale worktree Dirty = true; an unresolvable clean state must not be asserted as an answer")
	}
	if wt.Reason == "" {
		t.Error("stale worktree Reason is empty; want git's prunable reason disclosed")
	}
	if !strings.Contains(wt.Reason, "prunable") {
		t.Errorf("stale worktree Reason = %q, want it to name the prunable cause", wt.Reason)
	}
	// Merge state stays resolvable: git still reports the entry's HEAD.
	if wt.MergedUnresolved {
		t.Error("stale worktree MergedUnresolved = true, want false (merge state is checked in root against the reported HEAD)")
	}
}

// TestScanWorktrees_BogusDefaultTip_DisclosedPerWorktreeNotAborted proves
// the resilience is not limited to the clean-state check: when the
// merge-state check itself cannot resolve (here, an invalid default tip),
// every worktree's merge state is DISCLOSED as unresolvable rather than
// the whole survey aborting. In production Scan resolves the default tip
// via gitx.RevParse before this is ever reached (and errors loudly on a
// bad default-branch ref — TestScan_Negative_UnresolvableDefaultBranchRef_
// IsARealError), so this exercises the per-worktree disclosure path
// directly.
func TestScanWorktrees_BogusDefaultTip_DisclosedPerWorktreeNotAborted(t *testing.T) {
	root, _ := buildWorktreeSurveyFixture(t)

	got, err := scanWorktrees(context.Background(), root, "not-a-real-commit")
	if err != nil {
		t.Fatalf("scanWorktrees(bogus default tip) aborted: %v (want per-worktree disclosure, no error)", err)
	}
	if len(got) == 0 {
		t.Fatal("scanWorktrees(bogus default tip) = empty, want the worktrees disclosed")
	}
	for _, wt := range got {
		if !wt.MergedUnresolved {
			t.Errorf("worktree %s MergedUnresolved = false, want true (default tip did not resolve)", wt.Path)
		}
		if wt.Merged {
			t.Errorf("worktree %s Merged = true; an unresolvable merge state must not be asserted", wt.Path)
		}
		if wt.Reason == "" {
			t.Errorf("worktree %s Reason is empty; want the unresolvable cause disclosed", wt.Path)
		}
	}
}

// realOrSelfSurvey resolves symlinks in path, falling back to path
// unchanged if it cannot be resolved — the same D6-8-class macOS
// /var/folders parity helper internal/wtmanager's and internal/gitx's own
// tests define locally (git itself reports worktree paths already
// realpath'd).
func realOrSelfSurvey(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}
