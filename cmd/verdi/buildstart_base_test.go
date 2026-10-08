package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/specstate"
)

// TestRunBuildStart_CutsFromTheResolvedBase (UAT-023): with HEAD on a
// branch whose tree differs from the default branch, build start cuts
// feature/<name> at the default branch's commit and discloses the base; an
// origin whose default branch does not resolve is refused before the cut,
// leaving no branch and HEAD where it was.
func TestRunBuildStart_CutsFromTheResolvedBase(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, dir string)
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"the default branch's commit, not HEAD", func(t *testing.T, dir string) {
			t.Setenv("CI_DEFAULT_BRANCH", "")
			bare := t.TempDir()
			gitTestOutput(t, bare, "init", "-q", "--bare", "--initial-branch=main")
			gitTestOutput(t, dir, "remote", "add", "origin", bare)
			gitTestOutput(t, dir, "push", "-q", "origin", "main")
			gitTestOutput(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
		}, 0, "build start: base origin/main @ ", ""},
		{"an unresolvable default branch is refused before the cut", func(t *testing.T, dir string) {
			t.Setenv("CI_DEFAULT_BRANCH", "")
			gitTestOutput(t, dir, "remote", "add", "origin", t.TempDir())
		}, 2, "", "git remote set-head origin"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := fixturegit.Build(t, []fixturegit.Layer{{Files: buildStartStoreFiles(), Message: "accepted story"}})
			main := strings.TrimSpace(gitTestOutput(t, repo.Dir, "rev-parse", "HEAD"))
			gitTestOutput(t, repo.Dir, "checkout", "-q", "-b", "elsewhere")
			if err := os.WriteFile(filepath.Join(repo.Dir, "elsewhere.txt"), []byte("only on elsewhere\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			gitTestOutput(t, repo.Dir, "add", "--", "elsewhere.txt")
			gitTestOutput(t, repo.Dir, "commit", "-q", "-m", "diverge", "--", "elsewhere.txt")
			elsewhere := strings.TrimSpace(gitTestOutput(t, repo.Dir, "rev-parse", "HEAD"))
			tt.setup(t, repo.Dir)

			var stdout, stderr bytes.Buffer
			resolver := fakeScaffoldResolver{result: specstate.Result{State: specstate.AcceptedPendingBuild}}
			got := runBuildStart(context.Background(), repo.Dir, "spec/widget-story", resolver,
				syncDeps{Runner: nil, GoTest: fakeGoTest{}, Model: phase7Model(t)}, &stdout, &stderr)
			if got != tt.wantCode {
				t.Fatalf("runBuildStart = %d, want %d; stdout=%s stderr=%s", got, tt.wantCode, stdout.String(), stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) || !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Fatalf("stdout = %q, stderr = %q; want %q and %q", stdout.String(), stderr.String(), tt.wantStdout, tt.wantStderr)
			}
			branches := gitTestOutput(t, repo.Dir, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/feature/")
			head := strings.TrimSpace(gitTestOutput(t, repo.Dir, "rev-parse", "HEAD"))
			if tt.wantCode != 0 {
				if branches != "" || head != elsewhere {
					t.Fatalf("a refused build start left branches %q and HEAD %s, want none and %s", branches, head, elsewhere)
				}
				return
			}
			if want := "refs/heads/feature/widget-story " + main + "\n"; branches != want {
				t.Fatalf("feature branches = %q, want %q (cut at main, not HEAD %s)", branches, want, elsewhere)
			}
			// Cut at the commit, not at origin/main's name: git sets up no
			// upstream, a config write build start does not declare.
			if out, err := exec.CommandContext(context.Background(), "git", "-C", repo.Dir, "config", "--local", "--get-regexp", `^branch\.feature/`).CombinedOutput(); err == nil {
				t.Fatalf("the cut wrote branch configuration %q; build start declares no config write", out)
			}
		})
	}
}
