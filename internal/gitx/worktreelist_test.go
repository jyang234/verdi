package gitx

import (
	"context"
	"path/filepath"
	"testing"
)

func TestWorktreeList_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := CheckoutNewBranch(ctx, repo.Dir, "design/x"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}
	if err := Checkout(ctx, repo.Dir, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}

	branchWT := filepath.Join(t.TempDir(), "branch-wt")
	if err := WorktreeAdd(ctx, repo.Dir, branchWT, "design/x"); err != nil {
		t.Fatalf("WorktreeAdd(branch): %v", err)
	}

	detachedWT := filepath.Join(t.TempDir(), "detached-wt")
	if _, err := run(ctx, repo.Dir, "worktree", "add", "--detach", "--quiet", detachedWT, "main"); err != nil {
		t.Fatalf("worktree add --detach: %v", err)
	}

	entries, err := WorktreeList(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("WorktreeList: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("WorktreeList = %+v, want 3 entries (primary + branch + detached)", entries)
	}

	// git's own documented ordering: the primary checkout is always first.
	primary := entries[0]
	if realOrSelfForTest(primary.Path) != realOrSelfForTest(repo.Dir) {
		t.Fatalf("WorktreeList[0].Path = %q, want the primary checkout %q", primary.Path, repo.Dir)
	}
	if primary.Branch != "main" {
		t.Fatalf("primary entry Branch = %q, want main", primary.Branch)
	}
	if primary.Head == "" {
		t.Fatal("primary entry Head is empty")
	}
	if primary.Bare {
		t.Fatal("primary entry Bare = true, want false (an ordinary checkout)")
	}

	var branchEntry, detachedEntry *WorktreeEntry
	for i := range entries {
		switch realOrSelfForTest(entries[i].Path) {
		case realOrSelfForTest(branchWT):
			branchEntry = &entries[i]
		case realOrSelfForTest(detachedWT):
			detachedEntry = &entries[i]
		}
	}
	if branchEntry == nil {
		t.Fatalf("WorktreeList missing the branch worktree entry: %+v", entries)
	}
	if branchEntry.Branch != "design/x" {
		t.Fatalf("branch entry Branch = %q, want design/x", branchEntry.Branch)
	}
	if detachedEntry == nil {
		t.Fatalf("WorktreeList missing the detached worktree entry: %+v", entries)
	}
	if detachedEntry.Branch != "" {
		t.Fatalf("detached entry Branch = %q, want empty (no branch, dc-4)", detachedEntry.Branch)
	}
	if detachedEntry.Head == "" {
		t.Fatal("detached entry Head is empty; want the checked-out commit disclosed even with no branch")
	}
}

func TestWorktreeList_Negative_NotARepo(t *testing.T) {
	if _, err := WorktreeList(context.Background(), t.TempDir()); err == nil {
		t.Fatal("WorktreeList outside a repo: want error, got nil")
	}
}

func TestParseWorktreeList_CapturesPrunableAndIgnoresOtherUnknownLines(t *testing.T) {
	// A hand-built porcelain transcript exercising a "locked" line (still
	// ignored — a newer git adding a field this parser does not need must
	// never break it) and a "prunable" line (now captured, so a worktree
	// survey can disclose WHY a stale worktree's live state is unresolvable
	// — spec/closure-hygiene AC-3(b)), plus a trailing newline after the
	// last block (git's real output shape, verified against a real checkout
	// in TestWorktreeList_Happy).
	out := "worktree /repo\n" +
		"HEAD aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n" +
		"branch refs/heads/main\n" +
		"\n" +
		"worktree /repo/stale-wt\n" +
		"HEAD bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n" +
		"detached\n" +
		"locked custom reason\n" +
		"prunable gitdir file points to non-existent location\n"

	got := parseWorktreeList(out)
	if len(got) != 2 {
		t.Fatalf("parseWorktreeList = %+v, want 2 entries", got)
	}
	if got[0].Path != "/repo" || got[0].Branch != "main" {
		t.Fatalf("parseWorktreeList[0] = %+v, want Path=/repo Branch=main", got[0])
	}
	if got[0].Prunable {
		t.Fatalf("parseWorktreeList[0].Prunable = true, want false (the primary is not prunable)")
	}
	if got[1].Path != "/repo/stale-wt" || got[1].Branch != "" || got[1].Head == "" {
		t.Fatalf("parseWorktreeList[1] = %+v, want Path=/repo/stale-wt, detached (Branch empty), Head set", got[1])
	}
	if !got[1].Prunable {
		t.Fatalf("parseWorktreeList[1].Prunable = false, want true (a prunable line was present)")
	}
	if got[1].PrunableReason != "gitdir file points to non-existent location" {
		t.Fatalf("parseWorktreeList[1].PrunableReason = %q, want git's own reason text", got[1].PrunableReason)
	}
}

func TestParseWorktreeList_Bare(t *testing.T) {
	out := "worktree /bare-repo.git\n" +
		"bare\n"
	got := parseWorktreeList(out)
	if len(got) != 1 || !got[0].Bare {
		t.Fatalf("parseWorktreeList(bare) = %+v, want one Bare entry", got)
	}
}

// realOrSelfForTest resolves symlinks in path, falling back to path
// unchanged if it cannot be resolved (e.g. it does not exist) — mirrors
// internal/wtmanager's own test helper of the same shape (D6-8-class
// macOS /var/folders symlink parity), needed because git itself reports
// worktree paths already realpath'd.
func realOrSelfForTest(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}
