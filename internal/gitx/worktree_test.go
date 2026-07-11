package gitx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
