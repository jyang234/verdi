package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/recovery"
)

// recoverDesignStartBound bounds every built-binary run below.
const recoverDesignStartBound = 2 * time.Minute

// runVerdiBounded runs bin as runVerdiBinary does, killed when ctx ends,
// so a hung child fails its own test instead of the package timeout.
func runVerdiBounded(ctx context.Context, t *testing.T, bin, dir string, extraEnv []string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return outBuf.String(), errBuf.String(), 0
	case ctx.Err() != nil:
		t.Fatalf("verdi %s did not finish within its bound: %v", strings.Join(args, " "), ctx.Err())
	case errors.As(err, &exitErr):
		return outBuf.String(), errBuf.String(), exitErr.ExitCode()
	default:
		t.Fatalf("running verdi %s: %v", strings.Join(args, " "), err)
	}
	return "", "", -1
}

// TestRecoverE2E_AfterResolvedBaseDesignStart pins the recovery half of
// ledger SI-341 (6): design start now cuts design/<name> at the resolved
// default branch's commit, so the cut carries no upstream (BL-141), and
// recovery's semantics are unchanged (parent co-4).
//
// The state is an interrupted design start: design/<name> at its cut,
// checked out, with nothing committed on it and whatever configuration the
// cut wrote. Recovery reads a spec's class only where a spec.md exists (the
// store's zones on disk, the ritual branches, the resolved base), and design
// start's cut carries none, so a bare cut is not recoverable by name. Here
// the spec lands on the default branch after the cut: design start runs to
// completion through the built binary, its scaffold commit is pushed to
// origin/main and fetched (a colleague landing the same spec), and git
// reset moves design/<name> back to its cut point, writing no
// configuration.
//
// Recovery still classifies the empty cut resolved-base: it offers the
// return to the re-resolved default branch, main, never to B, the
// colleague's branch at the cut that witnesses its emptiness. Where local
// main is current the unwind completes. Where local main is behind the cut,
// BL-143's case now applies to design start as it does to build start: the
// switch to main succeeds, `git branch -d` refuses (design/<name> has no
// upstream, and main does not contain its cut), and recover exits 1 with
// that postcondition VIOLATED and nothing lost: design/<name> still exists
// at its cut. While the cut set an upstream to origin/main, `git branch -d`
// succeeded there.
func TestRecoverE2E_AfterResolvedBaseDesignStart(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	for _, tt := range []struct {
		name      string
		staleMain bool
	}{
		{"local main behind the cut", true},
		{"local main at origin/main", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), recoverDesignStartBound)
			defer cancel()
			repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{
				".verdi/verdi.yaml": minimalManifestYAML,
			}, Message: "scaffold"}})
			git := func(args ...string) string { return gitTestOutput(t, repo.Dir, args...) }
			commit := func(file string) {
				if err := os.WriteFile(filepath.Join(repo.Dir, file), []byte(file+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				git("add", "--", file)
				git("commit", "-q", "-m", file, "--", file)
			}
			bare := t.TempDir()
			gitTestOutput(t, bare, "init", "-q", "--bare", "--initial-branch=main")
			git("remote", "add", "origin", bare)
			git("push", "-q", "origin", "main")
			git("checkout", "-q", "-b", "B", "main")
			commit("b.txt")
			git("push", "-q", "origin", "B:main")
			git("fetch", "-q", "origin")
			git("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
			git("checkout", "-q", "-b", "A", "main")
			commit("a.txt")
			originMain := strings.TrimSpace(git("rev-parse", "origin/main"))
			env := []string{"CI_DEFAULT_BRANCH="}

			const branch = "design/checkout"
			if stdout, stderr, code := runVerdiBounded(ctx, t, bin, repo.Dir, env, "design", "start", "--kind", "feature", "--name", "checkout", "--defer-statements"); code != 0 {
				t.Fatalf("design start: exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
			}
			if parent := strings.TrimSpace(git("rev-parse", branch+"^")); parent != originMain {
				t.Fatalf("%s's scaffold commit sits on %s, want the resolved base origin/main %s", branch, parent, originMain)
			}
			if out, err := exec.CommandContext(ctx, "git", "-C", repo.Dir, "config", "--local", "--get-regexp", `^branch\.design/`).CombinedOutput(); err == nil {
				t.Fatalf("design start's cut wrote branch configuration %q (BL-141)", out)
			}
			// The spec lands on the default branch, so recovery can read its
			// class; then design/<name> goes back to its cut point, the state
			// an interruption between the cut and the scaffold commit leaves.
			scaffold := strings.TrimSpace(git("rev-parse", branch))
			git("push", "-q", "origin", scaffold+":refs/heads/main")
			git("fetch", "-q", "origin")
			if !tt.staleMain {
				git("branch", "-f", "main", "origin/main")
			}
			git("reset", "-q", "--hard", originMain)

			stdout, stderr, code := runVerdiBounded(ctx, t, bin, repo.Dir, env, "recover", "--json", "spec/checkout")
			if code != 1 {
				t.Fatalf("verdi recover: exit %d, want 1\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
			}
			proj, err := recovery.Decode([]byte(strings.TrimRight(stdout, "\n")))
			if err != nil {
				t.Fatalf("recovery.Decode(stdout): %v\nstdout: %s", err, stdout)
			}
			var cut *recovery.RecognizedState
			for i, s := range proj.States {
				if s.Code == recovery.StateEmptyBranchCut && s.Target == branch {
					cut = &proj.States[i]
				}
			}
			if cut == nil {
				t.Fatalf("no empty-branch-cut state for %s: %+v", branch, proj.States)
			}
			if len(cut.Choices) != 1 {
				t.Fatalf("Choices = %+v, uncertainties %+v; want the one unwind", cut.Choices, cut.Uncertainties)
			}
			choice := cut.Choices[0]
			if want := "switch back to main|delete " + branch + " with git branch -d"; strings.Join(choice.Effects, "|") != want {
				t.Fatalf("Effects = %v, want %q: a resolved-base cut returns to the re-resolved default branch, never to B", choice.Effects, want)
			}

			stdout, stderr, code = runVerdiBounded(ctx, t, bin, repo.Dir, env, "recover", "spec/checkout", "--apply", choice.ID)
			if !tt.staleMain {
				if code != 0 {
					t.Fatalf("verdi recover --apply %s: exit %d\nstdout:\n%s\nstderr:\n%s", choice.ID, code, stdout, stderr)
				}
				for _, want := range []string{"postcondition: " + branch + " does not exist: held", "postcondition: current branch is main: held"} {
					if !strings.Contains(stdout, want) {
						t.Fatalf("stdout = %q, want %q", stdout, want)
					}
				}
				return
			}
			if code != 1 {
				t.Fatalf("verdi recover --apply %s with local main behind the cut: exit %d, want 1 (BL-143)\nstdout:\n%s\nstderr:\n%s", choice.ID, code, stdout, stderr)
			}
			for _, want := range []string{"postcondition: " + branch + " does not exist: VIOLATED", "postcondition: current branch is main: held"} {
				if !strings.Contains(stdout, want) {
					t.Fatalf("stdout = %q, want %q", stdout, want)
				}
			}
			if tip := strings.TrimSpace(git("rev-parse", "refs/heads/"+branch)); tip != originMain {
				t.Fatalf("%s = %s after the refused delete, want it kept at its cut %s (nothing lost)", branch, tip, originMain)
			}
		})
	}
}
