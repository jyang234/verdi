package gitx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initRepoForCommonDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGitCmd(t, dir, "init", "--quiet", "-b", "main")
	runGitCmd(t, dir, "-c", "user.name=Verdi Fixture", "-c", "user.email=fixture@verdi.invalid",
		"commit", "--quiet", "--allow-empty", "-m", "root")
	return dir
}

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestCommonDir_MainWorktree proves the ordinary case: run inside the main
// worktree, git prints a path relative to dir (almost always ".git"), which
// resolves to dir/.git — a real directory.
func TestCommonDir_MainWorktree(t *testing.T) {
	repo := initRepoForCommonDir(t)

	got, err := CommonDir(context.Background(), repo)
	if err != nil {
		t.Fatalf("CommonDir: %v", err)
	}
	if got == "" {
		t.Fatal("CommonDir returned empty string")
	}

	resolved := got
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(repo, resolved)
	}
	if fi, err := os.Stat(resolved); err != nil || !fi.IsDir() {
		t.Fatalf("resolved common dir %q is not a directory: %v", resolved, err)
	}
	if filepath.Clean(resolved) != filepath.Join(repo, ".git") {
		t.Fatalf("resolved common dir = %q, want %q", resolved, filepath.Join(repo, ".git"))
	}
}

// TestCommonDir_LinkedWorktree proves the linked-worktree case: run inside
// a `git worktree add`-cut linked worktree, git prints the MAIN worktree's
// .git directory — the shared administrative directory — not a path under
// the linked worktree itself.
func TestCommonDir_LinkedWorktree(t *testing.T) {
	repo := initRepoForCommonDir(t)
	ctx := context.Background()

	linkedPath := filepath.Join(t.TempDir(), "linked")
	if err := WorktreeAddDetached(ctx, repo, linkedPath, "HEAD"); err != nil {
		t.Fatalf("WorktreeAddDetached: %v", err)
	}

	got, err := CommonDir(ctx, linkedPath)
	if err != nil {
		t.Fatalf("CommonDir(linked): %v", err)
	}
	resolved := got
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(linkedPath, resolved)
	}
	want := filepath.Join(repo, ".git")
	real, rerr := filepath.EvalSymlinks(resolved)
	if rerr != nil {
		real = filepath.Clean(resolved)
	}
	wantReal, werr := filepath.EvalSymlinks(want)
	if werr != nil {
		wantReal = filepath.Clean(want)
	}
	if real != wantReal {
		t.Fatalf("resolved common dir = %q (%q), want %q (%q)", resolved, real, want, wantReal)
	}
}

// TestCommonDir_NotAGitRepo proves this wrapper surfaces git's own failure
// rather than a silent empty result when dir is not inside any repository.
func TestCommonDir_NotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	if _, err := CommonDir(context.Background(), dir); err == nil {
		t.Fatal("CommonDir(non-repo dir): want error, got nil")
	}
}
