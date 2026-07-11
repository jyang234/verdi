package gitx

import (
	"context"
	"testing"

	"github.com/OWNER/verdi/internal/fixturegit"
)

func TestIsAncestor_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	ok, err := IsAncestor(ctx, repo.Dir, repo.Heads[0], repo.Heads[1])
	if err != nil {
		t.Fatalf("IsAncestor(layer1, layer2): %v", err)
	}
	if !ok {
		t.Fatal("IsAncestor(layer1, layer2) = false, want true (layer1 is layer2's parent)")
	}

	// A commit is its own ancestor for this purpose (the fold's "current at
	// C" must include a record produced at C itself).
	ok, err = IsAncestor(ctx, repo.Dir, repo.Head, repo.Head)
	if err != nil {
		t.Fatalf("IsAncestor(HEAD, HEAD): %v", err)
	}
	if !ok {
		t.Fatal("IsAncestor(HEAD, HEAD) = false, want true (a commit is its own ancestor)")
	}
}

func TestIsAncestor_Negative(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	t.Run("later commit is not an ancestor of an earlier one", func(t *testing.T) {
		ok, err := IsAncestor(ctx, repo.Dir, repo.Heads[1], repo.Heads[0])
		if err != nil {
			t.Fatalf("IsAncestor(layer2, layer1): unexpected error: %v", err)
		}
		if ok {
			t.Fatal("IsAncestor(layer2, layer1) = true, want false (layer2 is layer1's child, not ancestor)")
		}
	})

	t.Run("unrelated history is not an ancestor", func(t *testing.T) {
		other := fixturegit.Build(t, []fixturegit.Layer{
			{Files: map[string]string{"x.txt": "x\n"}, Message: "unrelated"},
		})
		// other.Head does not resolve inside repo.Dir at all, so this is an
		// error (unresolvable revision), not a false answer.
		if _, err := IsAncestor(ctx, repo.Dir, other.Head, repo.Head); err == nil {
			t.Fatal("IsAncestor with a commit unresolvable in dir: want error, got nil")
		}
	})

	t.Run("not a repository at all", func(t *testing.T) {
		notARepo := t.TempDir()
		if _, err := IsAncestor(ctx, notARepo, repo.Heads[0], repo.Heads[1]); err == nil {
			t.Fatal("IsAncestor outside a repo: want error, got nil")
		}
	})
}
