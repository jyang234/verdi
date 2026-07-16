package gitx

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAddPathsCommit_Happy proves AddPaths stages exactly the named paths —
// a sibling untracked file in the same working tree is never picked up,
// unlike AddAll's `git add -A` (D6-33: a ritual that commits a frozen stamp
// must stage only the paths it itself modified).
func TestAddPathsCommit_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := os.WriteFile(filepath.Join(repo.Dir, "wanted.txt"), []byte("wanted content\n"), 0o644); err != nil {
		t.Fatalf("writing wanted.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "unrelated.txt"), []byte("unrelated content\n"), 0o644); err != nil {
		t.Fatalf("writing unrelated.txt: %v", err)
	}

	if err := AddPaths(ctx, repo.Dir, filepath.Join(repo.Dir, "wanted.txt")); err != nil {
		t.Fatalf("AddPaths: %v", err)
	}
	sha, err := CreateCommit(ctx, repo.Dir, "add wanted.txt only")
	if err != nil {
		t.Fatalf("CreateCommit: %v", err)
	}
	if sha == repo.Head {
		t.Fatal("Commit did not produce a new HEAD")
	}

	// The new commit's tree contains wanted.txt ...
	show, err := Show(ctx, repo.Dir, sha, "wanted.txt")
	if err != nil {
		t.Fatalf("Show(sha, wanted.txt): %v", err)
	}
	if strings.TrimSpace(string(show)) != "wanted content" {
		t.Fatalf("Show(sha, wanted.txt) = %q, want %q", show, "wanted content")
	}

	// ... but never unrelated.txt: it stays untracked, out of the commit
	// entirely (Show on a path absent from the tree is an error).
	if _, err := Show(ctx, repo.Dir, sha, "unrelated.txt"); err == nil {
		t.Fatal("Show(sha, unrelated.txt): want an error (path must not be in the commit's tree), got nil")
	}
	entries, err := DiffNameStatus(ctx, repo.Dir, repo.Head, sha)
	if err != nil {
		t.Fatalf("DiffNameStatus: %v", err)
	}
	if len(entries) != 1 || entries[0].Path != "wanted.txt" {
		t.Fatalf("DiffNameStatus(repo.Head, sha) = %+v, want exactly one entry for wanted.txt", entries)
	}
}

// TestAddPathsCommit_MultiplePaths proves AddPaths stages every path given,
// not just the first — accept.go's own use needs this when a supersession
// flip modifies a second spec.md alongside the one being accepted.
func TestAddPathsCommit_MultiplePaths(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := os.WriteFile(filepath.Join(repo.Dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatalf("writing a.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatalf("writing b.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "c.txt"), []byte("c\n"), 0o644); err != nil {
		t.Fatalf("writing c.txt: %v", err)
	}

	if err := AddPaths(ctx, repo.Dir, filepath.Join(repo.Dir, "a.txt"), filepath.Join(repo.Dir, "b.txt")); err != nil {
		t.Fatalf("AddPaths: %v", err)
	}
	sha, err := CreateCommit(ctx, repo.Dir, "add a and b, not c")
	if err != nil {
		t.Fatalf("CreateCommit: %v", err)
	}

	entries, err := DiffNameStatus(ctx, repo.Dir, repo.Head, sha)
	if err != nil {
		t.Fatalf("DiffNameStatus: %v", err)
	}
	got := map[string]bool{}
	for _, e := range entries {
		got[e.Path] = true
	}
	if !got["a.txt"] || !got["b.txt"] || got["c.txt"] {
		t.Fatalf("DiffNameStatus entries = %+v, want exactly a.txt and b.txt, never c.txt", entries)
	}
}

// TestAddPaths_Negative covers AddPaths' own operational-error paths: no
// paths given, and a path outside any git repository.
func TestAddPaths_Negative(t *testing.T) {
	ctx := context.Background()

	t.Run("no paths", func(t *testing.T) {
		repo := buildRepo(t)
		if err := AddPaths(ctx, repo.Dir); err == nil {
			t.Fatal("AddPaths(no paths): want error, got nil")
		}
	})

	t.Run("not a repo", func(t *testing.T) {
		notARepo := t.TempDir()
		if err := os.WriteFile(filepath.Join(notARepo, "x.txt"), []byte("x\n"), 0o644); err != nil {
			t.Fatalf("writing x.txt: %v", err)
		}
		if err := AddPaths(ctx, notARepo, filepath.Join(notARepo, "x.txt")); err == nil {
			t.Fatal("AddPaths outside a repo: want error, got nil")
		}
	})

	t.Run("path does not exist and is not a deletion", func(t *testing.T) {
		repo := buildRepo(t)
		if err := AddPaths(ctx, repo.Dir, filepath.Join(repo.Dir, "does-not-exist.txt")); err == nil {
			t.Fatal("AddPaths(nonexistent path): want error, got nil")
		}
	})
}
