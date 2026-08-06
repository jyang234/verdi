package gitx

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWorktreeAddDetached_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	wtPath := filepath.Join(t.TempDir(), "detached")
	if err := WorktreeAddDetached(ctx, repo.Dir, wtPath, repo.Heads[0]); err != nil {
		t.Fatalf("WorktreeAddDetached: %v", err)
	}

	// Detached at the exact sha: HEAD resolves to it, and there is no
	// checked-out branch name at all.
	head, err := RevParse(ctx, wtPath, "HEAD")
	if err != nil {
		t.Fatalf("RevParse(HEAD) in cut worktree: %v", err)
	}
	if head != repo.Heads[0] {
		t.Fatalf("cut worktree HEAD = %q, want exact sha %q", head, repo.Heads[0])
	}
	branch, err := CurrentBranch(ctx, wtPath)
	if err != nil {
		t.Fatalf("CurrentBranch(cut worktree): %v", err)
	}
	if branch != "" {
		t.Fatalf("WorktreeAddDetached minted a checked-out branch %q, want detached HEAD (empty)", branch)
	}

	// No local branch was minted anywhere in the repo either.
	branches, err := LocalBranches(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("LocalBranches: %v", err)
	}
	if len(branches) != 1 || branches[0] != "main" {
		t.Fatalf("LocalBranches after WorktreeAddDetached = %v, want only [main] (never mints a branch)", branches)
	}

	// The serving checkout's own branch is untouched.
	rootBranch, err := CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("CurrentBranch(repo.Dir): %v", err)
	}
	if rootBranch != "main" {
		t.Fatalf("WorktreeAddDetached changed the serving checkout's own branch to %q", rootBranch)
	}
}

func TestWorktreeAddDetached_Negative_BadSHA(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	wtPath := filepath.Join(t.TempDir(), "bad")
	badSHA := "0000000000000000000000000000000000dead"
	err := WorktreeAddDetached(ctx, repo.Dir, wtPath, badSHA)
	if err == nil {
		t.Fatal("WorktreeAddDetached(bad sha): want error, got nil")
	}
	if _, statErr := os.Stat(wtPath); !os.IsNotExist(statErr) {
		t.Fatalf("WorktreeAddDetached(bad sha) left a directory behind: err=%v", statErr)
	}

	// The serving checkout's own branch is unchanged after a failure too.
	rootBranch, berr := CurrentBranch(ctx, repo.Dir)
	if berr != nil {
		t.Fatalf("CurrentBranch(repo.Dir): %v", berr)
	}
	if rootBranch != "main" {
		t.Fatalf("failed WorktreeAddDetached changed the serving checkout's own branch to %q", rootBranch)
	}
}
