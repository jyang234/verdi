package gitx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildPatch modifies path inside repoDir to newContent, captures the
// resulting unstaged unified diff (`git diff -- path`), restores the
// original on-disk content, and returns the captured patch bytes — so a
// test can apply that same patch to a freshly cut, unmodified worktree.
func buildPatch(t *testing.T, repoDir, path, newContent string) []byte {
	t.Helper()
	full := filepath.Join(repoDir, path)
	original, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("reading %s before patching: %v", path, err)
	}
	if err := os.WriteFile(full, []byte(newContent), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	out, err := exec.Command("git", "-C", repoDir, "diff", "--", path).Output()
	if err != nil {
		t.Fatalf("git diff -- %s: %v", path, err)
	}
	if err := os.WriteFile(full, original, 0o644); err != nil {
		t.Fatalf("restoring %s: %v", path, err)
	}
	if len(out) == 0 {
		t.Fatalf("git diff -- %s produced no output", path)
	}
	return out
}

func TestApplyPatch_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()
	patch := buildPatch(t, repo.Dir, "a.txt", "patched content\n")

	wtPath := filepath.Join(t.TempDir(), "patched")
	if err := WorktreeAddDetached(ctx, repo.Dir, wtPath, repo.Head); err != nil {
		t.Fatalf("WorktreeAddDetached: %v", err)
	}

	if err := ApplyPatch(ctx, wtPath, patch); err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(wtPath, "a.txt"))
	if err != nil {
		t.Fatalf("reading patched file: %v", err)
	}
	if string(got) != "patched content\n" {
		t.Fatalf("a.txt after ApplyPatch = %q, want %q", got, "patched content\n")
	}
}

func TestApplyPatch_Negative_Malformed(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	wtPath := filepath.Join(t.TempDir(), "malformed")
	if err := WorktreeAddDetached(ctx, repo.Dir, wtPath, repo.Head); err != nil {
		t.Fatalf("WorktreeAddDetached: %v", err)
	}

	err := ApplyPatch(ctx, wtPath, []byte("this is not a patch at all\nneither is this\n"))
	if err == nil {
		t.Fatal("ApplyPatch(malformed): want error, got nil")
	}
}

func TestApplyPatch_Negative_Conflict(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()
	patch := buildPatch(t, repo.Dir, "a.txt", "patched content\n")

	wtPath := filepath.Join(t.TempDir(), "conflict")
	if err := WorktreeAddDetached(ctx, repo.Dir, wtPath, repo.Head); err != nil {
		t.Fatalf("WorktreeAddDetached: %v", err)
	}
	// Diverge the target file first so the captured patch's context lines
	// no longer match — a genuine apply conflict, not a malformed patch.
	if err := os.WriteFile(filepath.Join(wtPath, "a.txt"), []byte("already diverged\n"), 0o644); err != nil {
		t.Fatalf("diverging a.txt: %v", err)
	}

	err := ApplyPatch(ctx, wtPath, patch)
	if err == nil {
		t.Fatal("ApplyPatch(conflicting patch): want error, got nil")
	}
	if !strings.Contains(err.Error(), "a.txt") {
		t.Fatalf("ApplyPatch(conflict) error = %v, want it to disclose the failing path (git apply stderr)", err)
	}
}
