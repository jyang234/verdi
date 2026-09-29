package gitx

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// TestUntrackedPaths_Happy proves an untracked file is reported, a staged
// new file is not (git already knows it), and an unstaged modification to
// an already-tracked file is not (still not "never seen").
func TestUntrackedPaths_Happy(t *testing.T) {
	ctx := context.Background()
	repo := buildRepo(t)

	if err := os.WriteFile(filepath.Join(repo.Dir, "untracked.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatalf("writing untracked.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "staged-new.txt"), []byte("also new\n"), 0o644); err != nil {
		t.Fatalf("writing staged-new.txt: %v", err)
	}
	if err := AddPaths(ctx, repo.Dir, "staged-new.txt"); err != nil {
		t.Fatalf("AddPaths: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "a.txt"), []byte("unstaged edit\n"), 0o644); err != nil {
		t.Fatalf("writing a.txt: %v", err)
	}

	got, err := UntrackedPaths(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("UntrackedPaths: %v", err)
	}
	want := []string{"untracked.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("UntrackedPaths = %#v, want %#v", got, want)
	}
}

// TestUntrackedPaths_CleanIsNil proves a clean tree (nothing untracked)
// answers a nil slice, never an error.
func TestUntrackedPaths_CleanIsNil(t *testing.T) {
	repo := buildRepo(t)
	got, err := UntrackedPaths(context.Background(), repo.Dir)
	if err != nil {
		t.Fatalf("UntrackedPaths: %v", err)
	}
	if got != nil {
		t.Fatalf("UntrackedPaths(clean) = %#v, want nil", got)
	}
}

// TestUntrackedPaths_RespectsGitignore proves an ignored untracked file
// never surfaces — --exclude-standard is load-bearing, not incidental.
func TestUntrackedPaths_RespectsGitignore(t *testing.T) {
	ctx := context.Background()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".gitignore": "*.log\n"},
		Message: "seed",
	}})

	if err := os.WriteFile(filepath.Join(repo.Dir, "ignored.log"), []byte("noise\n"), 0o644); err != nil {
		t.Fatalf("writing ignored.log: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "kept.txt"), []byte("kept\n"), 0o644); err != nil {
		t.Fatalf("writing kept.txt: %v", err)
	}

	got, err := UntrackedPaths(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("UntrackedPaths: %v", err)
	}
	want := []string{"kept.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("UntrackedPaths(gitignore) = %#v, want %#v", got, want)
	}
}

// TestUntrackedPaths_Negative proves a directory outside any git repository
// is a surfaced error, never a silent empty result.
func TestUntrackedPaths_Negative(t *testing.T) {
	notARepo := t.TempDir()
	if _, err := UntrackedPaths(context.Background(), notARepo); err == nil {
		t.Fatal("UntrackedPaths outside a repo: want error, got nil")
	}
}
