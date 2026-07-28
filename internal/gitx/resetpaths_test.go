package gitx

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// indexHasStagedChanges reports whether dir's index differs from HEAD —
// `git diff --cached --quiet` exits non-zero exactly when something is
// staged. Uses this package's own run helper (an in-package test).
func indexHasStagedChanges(t *testing.T, dir string) bool {
	t.Helper()
	_, err := run(context.Background(), dir, "diff", "--cached", "--quiet")
	return err != nil
}

// TestResetPaths_UnstagesExactlyTheGivenPath proves a staged modification is
// unstaged by ResetPaths while the working-tree change itself is left intact
// (a mixed reset, never --hard).
func TestResetPaths_UnstagesExactlyTheGivenPath(t *testing.T) {
	repo := buildLsTreeRepo(t)
	ctx := context.Background()
	rel := ".verdi/specs/active/foo/spec.md"
	abs := filepath.Join(repo.Dir, rel)

	if err := os.WriteFile(abs, []byte("foo modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AddPaths(ctx, repo.Dir, rel); err != nil {
		t.Fatalf("AddPaths: %v", err)
	}
	if !indexHasStagedChanges(t, repo.Dir) {
		t.Fatal("precondition: expected a staged change after AddPaths")
	}

	if err := ResetPaths(ctx, repo.Dir, rel); err != nil {
		t.Fatalf("ResetPaths: %v", err)
	}
	if indexHasStagedChanges(t, repo.Dir) {
		t.Fatal("ResetPaths did not unstage the modification")
	}

	// The working-tree edit itself survives (mixed reset, not --hard).
	got, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "foo modified\n" {
		t.Fatalf("ResetPaths touched the working tree: %q", got)
	}
}

// TestResetPaths_Negative pins the empty-paths caller-bug guard, mirroring
// AddPaths' own.
func TestResetPaths_Negative(t *testing.T) {
	repo := buildLsTreeRepo(t)
	if err := ResetPaths(context.Background(), repo.Dir); err == nil {
		t.Fatal("ResetPaths(no paths): want error, got nil")
	}
}
