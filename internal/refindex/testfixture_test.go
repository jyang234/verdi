package refindex

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
)

// runGit runs git in dir with the process's inherited environment (identity
// is already configured on the fixturegit-built repo), failing the test on a
// non-zero exit — the same convention gitx's own tests use for setup steps
// fixturegit itself does not cover (branch_test.go's runFor).
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v (dir %s): %v\n%s", args, dir, err, out)
	}
	return string(out)
}

// checkoutNewBranch cuts and switches to a new branch at the current HEAD.
func checkoutNewBranch(t *testing.T, dir, name string) {
	t.Helper()
	runGit(t, dir, "checkout", "--quiet", "-b", name)
}

// checkoutExisting switches to an already-existing branch.
func checkoutExisting(t *testing.T, dir, name string) {
	t.Helper()
	runGit(t, dir, "checkout", "--quiet", name)
}

// writeAndCommit writes files (repo-relative paths) into dir's working tree
// and commits them, returning the new HEAD sha.
func writeAndCommit(t *testing.T, dir string, files map[string]string, message string) string {
	t.Helper()
	for path, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", path, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "--quiet", "--no-verify", "-m", message)
	return strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
}

// setDefaultBranchSymref points refs/remotes/origin/HEAD at
// refs/remotes/origin/<branch> directly — a real symbolic ref,
// hermetically constructed with no remote, clone, or fetch at all (a
// stronger hermeticity than the clone-based convention
// gitx/branch_test.go's TestDefaultBranch_RemoteHEADConfigured already uses
// in this codebase) so gitx.DefaultBranch resolves branch as the default
// branch (CLAUDE.md: "no network in any test").
func setDefaultBranchSymref(t *testing.T, dir, branch string) {
	t.Helper()
	runGit(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/"+branch)
}

// createRemoteDesignRef creates a simulated refs/remotes/origin/design/<name>
// ref pointing at commit, via gitx.UpdateRef (create-only plumbing that
// never touches HEAD or the working tree) — CO-2's "directly-created
// refs/remotes/origin/design/* refs" hermetic alternative to a real fetch.
func createRemoteDesignRef(t *testing.T, dir, name, commit string) {
	t.Helper()
	if err := gitx.UpdateRef(context.Background(), dir, "refs/remotes/origin/design/"+name, commit); err != nil {
		t.Fatalf("createRemoteDesignRef(%s): %v", name, err)
	}
}

// deleteLocalBranch removes a local branch ref (used to simulate a
// "remote-only" branch: authored on a throwaway local branch so its commit
// exists, then the local ref is deleted, leaving only the remote-tracking
// ref createRemoteDesignRef created at the same commit).
func deleteLocalBranch(t *testing.T, dir, name string) {
	t.Helper()
	runGit(t, dir, "branch", "-D", name)
}

func componentSpecMD(name, status string) string {
	return fmt.Sprintf(`---
id: spec/%s
kind: spec
class: component
title: "%s"
status: %s
owners: [platform-team]
---
# %s
`, name, name, status, name)
}

func storySpecAcceptedMD(name string) string {
	return fmt.Sprintf(`---
id: spec/%s
kind: spec
class: story
title: "%s"
status: accepted-pending-build
owners: [platform-team]
story: jira:TEST-1
problem: { text: "a problem", anchor: "#problem" }
outcome: { text: "an outcome", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
frozen: { at: 2026-01-01, commit: 1234567 }
---
# %s

## Problem

a problem

## Outcome

an outcome
`, name, name, name)
}

// entryByRef finds the entry named ref in entries, failing the test if it
// is not present exactly once.
func entryByRef(t *testing.T, entries []Entry, ref string) Entry {
	t.Helper()
	var found []Entry
	for _, e := range entries {
		if e.Ref == ref {
			found = append(found, e)
		}
	}
	if len(found) != 1 {
		t.Fatalf("entries for %q = %d, want exactly 1 (entries: %+v)", ref, len(found), entries)
	}
	return found[0]
}

func refs(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Ref
	}
	sort.Strings(out)
	return out
}

// hashWorkingTree walks dir (excluding .git) and returns a combined sha256
// of every tracked file's path and content — a content-addressed proof the
// working tree is byte-identical across two snapshots, independent of git's
// own reported status.
func hashWorkingTree(t *testing.T, dir string) string {
	t.Helper()
	h := sha256.New()
	var paths []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("hashWorkingTree: walking %s: %v", dir, err)
	}
	sort.Strings(paths)
	for _, rel := range paths {
		content, readErr := os.ReadFile(filepath.Join(dir, rel))
		if readErr != nil {
			t.Fatalf("hashWorkingTree: reading %s: %v", rel, readErr)
		}
		h.Write([]byte(rel))
		h.Write(content)
	}
	return hex.EncodeToString(h.Sum(nil))
}
