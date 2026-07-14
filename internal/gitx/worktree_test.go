package gitx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusDirty(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	dirty, err := StatusDirty(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("StatusDirty: %v", err)
	}
	if dirty {
		t.Fatal("fresh fixture repo reported dirty")
	}

	if err := os.WriteFile(filepath.Join(repo.Dir, "a.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirty, err = StatusDirty(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("StatusDirty: %v", err)
	}
	if !dirty {
		t.Fatal("modified tree reported clean")
	}
}

func TestStatusDirty_Negative(t *testing.T) {
	if _, err := StatusDirty(context.Background(), t.TempDir()); err == nil {
		t.Fatal("StatusDirty outside a repo: want error")
	}
}

func TestLocalBranches(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := CheckoutNewBranch(ctx, repo.Dir, "design/x"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}
	branches, err := LocalBranches(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("LocalBranches: %v", err)
	}
	want := map[string]bool{"main": true, "design/x": true}
	if len(branches) != 2 || !want[branches[0]] || !want[branches[1]] {
		t.Fatalf("LocalBranches = %v, want main + design/x", branches)
	}
}

func TestLocalBranches_Negative(t *testing.T) {
	if _, err := LocalBranches(context.Background(), t.TempDir()); err == nil {
		t.Fatal("LocalBranches outside a repo: want error")
	}
}

func TestCheckout_GuardsDirtyTree(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()
	if err := CheckoutNewBranch(ctx, repo.Dir, "design/x"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}

	// Clean: the switch works.
	if err := Checkout(ctx, repo.Dir, "main"); err != nil {
		t.Fatalf("Checkout (clean): %v", err)
	}
	got, _ := CurrentBranch(ctx, repo.Dir)
	if got != "main" {
		t.Fatalf("CurrentBranch after checkout = %q, want main", got)
	}

	// Dirty: the guard blocks before git runs.
	if err := os.WriteFile(filepath.Join(repo.Dir, "a.txt"), []byte("mid-edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Checkout(ctx, repo.Dir, "design/x"); err == nil {
		t.Fatal("Checkout with a dirty tree succeeded, want guard error")
	}
	got, _ = CurrentBranch(ctx, repo.Dir)
	if got != "main" {
		t.Fatalf("guarded checkout still switched branch to %q", got)
	}
}

func TestCheckout_Negative_UnknownBranch(t *testing.T) {
	repo := buildRepo(t)
	if err := Checkout(context.Background(), repo.Dir, "no-such-branch"); err == nil {
		t.Fatal("Checkout of a missing branch succeeded")
	}
}

func TestWorktreeAdd_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := CheckoutNewBranch(ctx, repo.Dir, "design/x"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}
	if err := Checkout(ctx, repo.Dir, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}

	wtPath := filepath.Join(t.TempDir(), "x")
	if err := WorktreeAdd(ctx, repo.Dir, wtPath, "design/x"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wtPath, "a.txt")); err != nil {
		t.Fatalf("cut worktree missing expected file: %v", err)
	}
	got, err := CurrentBranch(ctx, wtPath)
	if err != nil {
		t.Fatalf("CurrentBranch(cut worktree): %v", err)
	}
	if got != "design/x" {
		t.Fatalf("cut worktree's branch = %q, want design/x", got)
	}

	// The serving checkout's own branch is untouched.
	rootBranch, _ := CurrentBranch(ctx, repo.Dir)
	if rootBranch != "main" {
		t.Fatalf("WorktreeAdd changed the serving checkout's own branch to %q", rootBranch)
	}
}

func TestWorktreeAdd_Negative(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	t.Run("nonexistent branch", func(t *testing.T) {
		wtPath := filepath.Join(t.TempDir(), "nope")
		if err := WorktreeAdd(ctx, repo.Dir, wtPath, "design/nope"); err == nil {
			t.Fatal("WorktreeAdd(nonexistent branch): want error, got nil")
		}
		if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
			t.Fatalf("WorktreeAdd(nonexistent branch) left a directory behind: err=%v", err)
		}
	})

	t.Run("branch already checked out at dir itself", func(t *testing.T) {
		if err := CheckoutNewBranch(ctx, repo.Dir, "design/here"); err != nil {
			t.Fatalf("CheckoutNewBranch: %v", err)
		}
		wtPath := filepath.Join(t.TempDir(), "here")
		err := WorktreeAdd(ctx, repo.Dir, wtPath, "design/here")
		if err == nil {
			t.Fatal("WorktreeAdd(branch checked out at dir): want error, got nil")
		}
		if !strings.Contains(err.Error(), "already checked out") {
			t.Fatalf("WorktreeAdd error = %v, want it to mention \"already checked out\"", err)
		}
	})
}

func TestWorktreeRemove_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := CheckoutNewBranch(ctx, repo.Dir, "design/x"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}
	if err := Checkout(ctx, repo.Dir, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}
	wtPath := filepath.Join(t.TempDir(), "x")
	if err := WorktreeAdd(ctx, repo.Dir, wtPath, "design/x"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}

	if err := WorktreeRemove(ctx, repo.Dir, wtPath); err != nil {
		t.Fatalf("WorktreeRemove: %v", err)
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Fatalf("worktree directory still present after WorktreeRemove: err=%v", err)
	}
}

func TestWorktreeRemove_Negative_DirtyRefusedWithoutForce(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := CheckoutNewBranch(ctx, repo.Dir, "design/x"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}
	if err := Checkout(ctx, repo.Dir, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}
	wtPath := filepath.Join(t.TempDir(), "x")
	if err := WorktreeAdd(ctx, repo.Dir, wtPath, "design/x"); err != nil {
		t.Fatalf("WorktreeAdd: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "a.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := WorktreeRemove(ctx, repo.Dir, wtPath); err == nil {
		t.Fatal("WorktreeRemove(dirty worktree, no --force): want error, got nil")
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("dirty worktree removed despite refusal: %v", err)
	}
}

func TestWorktreeRemove_Negative_NoSuchWorktree(t *testing.T) {
	repo := buildRepo(t)
	if err := WorktreeRemove(context.Background(), repo.Dir, filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("WorktreeRemove(never-added path): want error, got nil")
	}
}

func TestPushAndHasRemote(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	// No remote yet.
	has, err := HasRemote(ctx, repo.Dir, "origin")
	if err != nil {
		t.Fatalf("HasRemote: %v", err)
	}
	if has {
		t.Fatal("HasRemote true with no remotes")
	}
	if err := Push(ctx, repo.Dir); err == nil {
		t.Fatal("Push with no origin succeeded, want error")
	}

	// A local bare origin: push round-trips (hermetic — no network).
	bare := t.TempDir()
	if err := exec.Command("git", "init", "--bare", "--quiet", "--initial-branch=main", bare).Run(); err != nil {
		t.Fatalf("git init --bare: %v", err)
	}
	if err := exec.Command("git", "-C", repo.Dir, "remote", "add", "origin", bare).Run(); err != nil {
		t.Fatalf("git remote add: %v", err)
	}
	has, err = HasRemote(ctx, repo.Dir, "origin")
	if err != nil {
		t.Fatalf("HasRemote: %v", err)
	}
	if !has {
		t.Fatal("HasRemote false after remote add")
	}
	if err := Push(ctx, repo.Dir); err != nil {
		t.Fatalf("Push: %v", err)
	}
	out, err := exec.Command("git", "-C", bare, "rev-parse", "main").Output()
	if err != nil || len(out) == 0 {
		t.Fatalf("bare origin has no main after push: %v", err)
	}
}
