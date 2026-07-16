package gitx

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestIsShallow_Table covers the happy and negative paths: a repo with
// git's shallow-boundary marker present reports shallow; the same repo
// without it reports not-shallow; a non-git directory is a surfaced error,
// never a false "not shallow".
func TestIsShallow_Table(t *testing.T) {
	ctx := context.Background()

	t.Run("marker present is shallow", func(t *testing.T) {
		repo := buildRepo(t)
		// An empty marker is what `git log` itself tolerates and what a
		// depth-limited fetch leaves behind; its existence is the signal.
		if err := os.WriteFile(filepath.Join(repo.Dir, ".git", "shallow"), nil, 0o644); err != nil {
			t.Fatalf("placing shallow marker: %v", err)
		}
		got, err := IsShallow(ctx, repo.Dir)
		if err != nil {
			t.Fatalf("IsShallow(shallow repo): %v", err)
		}
		if !got {
			t.Error("IsShallow = false, want true when git's shallow marker is present")
		}
	})

	t.Run("no marker is not shallow", func(t *testing.T) {
		repo := buildRepo(t)
		got, err := IsShallow(ctx, repo.Dir)
		if err != nil {
			t.Fatalf("IsShallow(full repo): %v", err)
		}
		if got {
			t.Error("IsShallow = true, want false for an ordinary (non-shallow) clone")
		}
	})

	t.Run("non-git dir is an error", func(t *testing.T) {
		notARepo := t.TempDir()
		if _, err := IsShallow(ctx, notARepo); err == nil {
			t.Fatal("IsShallow(non-git dir): want error, got nil (must never guess not-shallow)")
		}
	})
}
