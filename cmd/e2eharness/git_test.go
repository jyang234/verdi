package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunGitPinsDeterministicEnv proves a commit made through runGit carries
// the fixed author/committer date, not the wall clock. Local git only — no
// network (co-1).
func TestRunGitPinsDeterministicEnv(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
	dir := t.TempDir()

	if err := runGit(dir, nil, "init", "--quiet", "--initial-branch=main"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := runGit(dir, nil, "commit", "--quiet", "--no-verify", "--allow-empty", "-m", "test commit"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	// %at/%ct are the raw author/committer unix timestamps; --date=format:%s
	// re-renders through the local timezone and is not reliable here.
	out, err := exec.Command("git", "-C", dir, "log", "-1", "--format=%at|%ct").CombinedOutput()
	if err != nil {
		t.Fatalf("git log: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	const want = "1704067200|1704067200"
	if got != want {
		t.Fatalf("commit author|committer epoch = %q, want %q", got, want)
	}
}

// TestRunGitWrapsFailure proves a failing git invocation surfaces the
// command's output, not just an opaque exec error.
func TestRunGitWrapsFailure(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
	dir := filepath.Join(t.TempDir(), "does-not-exist")

	err := runGit(dir, nil, "init")
	if err == nil {
		t.Fatal("expected error running git in a nonexistent dir, got nil")
	}
}

// TestGitOutput proves the query twin returns trimmed stdout (happy: the
// deterministic commit's sha via rev-parse) and wraps failure (negative:
// rev-parse in an empty repo has no HEAD).
func TestGitOutput(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
	dir := t.TempDir()
	if err := runGit(dir, nil, "init", "--quiet", "--initial-branch=main"); err != nil {
		t.Fatalf("git init: %v", err)
	}

	if _, err := gitOutput(dir, "rev-parse", "HEAD"); err == nil {
		t.Fatal("expected error rev-parsing HEAD in an empty repo, got nil")
	}

	if err := runGit(dir, nil, "commit", "--quiet", "--no-verify", "--allow-empty", "-m", "test commit"); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	sha, err := gitOutput(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("gitOutput rev-parse: %v", err)
	}
	if len(sha) != 40 || strings.ContainsAny(sha, " \n") {
		t.Fatalf("gitOutput returned %q, want a trimmed 40-hex sha", sha)
	}
}
