package gitx

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckoutNewBranch_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := CheckoutNewBranch(ctx, repo.Dir, "design/my-feature"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}
	got, err := CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if got != "design/my-feature" {
		t.Fatalf("CurrentBranch = %q, want %q", got, "design/my-feature")
	}
	// The new branch starts at the same commit as its parent.
	head, err := RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatalf("RevParse(HEAD): %v", err)
	}
	if head != repo.Head {
		t.Fatalf("HEAD after CheckoutNewBranch = %q, want unchanged %q", head, repo.Head)
	}
}

func TestCheckoutNewBranch_Negative(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := CheckoutNewBranch(ctx, repo.Dir, "dup"); err != nil {
		t.Fatalf("first CheckoutNewBranch: %v", err)
	}
	// Back on main so the second attempt is a real "branch already exists"
	// collision, not a no-op re-checkout of the branch we're already on.
	if _, err := run(ctx, repo.Dir, "checkout", "main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	if err := CheckoutNewBranch(ctx, repo.Dir, "dup"); err == nil {
		t.Fatal("CheckoutNewBranch(existing branch name): want error, got nil")
	}

	notARepo := t.TempDir()
	if err := CheckoutNewBranch(ctx, notARepo, "whatever"); err == nil {
		t.Fatal("CheckoutNewBranch outside a repo: want error, got nil")
	}
}

func TestAddAllCommit_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := os.WriteFile(filepath.Join(repo.Dir, "new.txt"), []byte("new content\n"), 0o644); err != nil {
		t.Fatalf("writing new.txt: %v", err)
	}
	if err := AddAll(ctx, repo.Dir); err != nil {
		t.Fatalf("AddAll: %v", err)
	}
	sha, err := CreateCommit(ctx, repo.Dir, "add new.txt")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if sha == repo.Head {
		t.Fatal("Commit did not produce a new HEAD")
	}
	got, err := RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatalf("RevParse(HEAD): %v", err)
	}
	if got != sha {
		t.Fatalf("RevParse(HEAD) = %q, want the just-created commit %q", got, sha)
	}

	// The new commit's tree really contains new.txt (AddAll actually staged
	// it, not just a message-only commit).
	show, err := Show(ctx, repo.Dir, sha, "new.txt")
	if err != nil {
		t.Fatalf("Show(sha, new.txt): %v", err)
	}
	if strings.TrimSpace(string(show)) != "new content" {
		t.Fatalf("Show(sha, new.txt) = %q, want %q", show, "new content")
	}
}

func TestCommit_Negative(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if _, err := CreateCommit(ctx, repo.Dir, ""); err == nil {
		t.Fatal("Commit(empty message): want error, got nil")
	}

	// Nothing staged: a plain `git commit` with no changes fails.
	if _, err := CreateCommit(ctx, repo.Dir, "empty commit attempt"); err == nil {
		t.Fatal("Commit with nothing staged: want error, got nil")
	}
}
