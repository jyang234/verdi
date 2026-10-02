package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/recovery"
)

// TestRecoverE2E_AfterResolvedBaseBuildStart pins ledger SI-334 (1) through
// the built binary: build start cuts feature/<name> at the resolved
// default branch's commit (UAT-023), so recovery classifies that cut as
// resolved-base, exactly as design start's, and offers the return to the
// re-resolved default branch. The operator works on A; a colleague's B
// sits at origin/main's tip, where the build branch is cut. A
// cut-from-current reading offers "switch back to B" (a branch the
// operator never had checked out) when local main is behind origin/main
// (the reviewer's probe P6), and withholds the unwind as a tie between B
// and main when local main is current.
//
// The unwind is executed only where local main is current. Where it is
// behind, feature/<name> carries no upstream (SI-333 (1)) and local main
// does not contain its cut, so the executor's `git branch -d` refuses after
// the switch: recover reports that postcondition violated, unchanged
// recovery semantics this test does not pin.
func TestRecoverE2E_AfterResolvedBaseBuildStart(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	for _, tt := range []struct {
		name       string
		staleMain  bool
		wantUnwind bool
	}{
		{"local main behind origin/main", true, false},
		{"local main at origin/main", false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{
				".verdi/verdi.yaml":                    minimalManifestYAML,
				".verdi/specs/active/checkout/spec.md": recoverE2ESpecMD,
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
			if !tt.staleMain {
				git("branch", "-f", "main", "origin/main")
			}
			git("checkout", "-q", "-b", "A", "main")
			commit("a.txt")
			originMain := strings.TrimSpace(git("rev-parse", "origin/main"))
			env := []string{"CI_DEFAULT_BRANCH="}

			if stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "build", "start", "spec/checkout"); code != 0 {
				t.Fatalf("build start: exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
			}
			if tip := strings.TrimSpace(git("rev-parse", "feature/checkout")); tip != originMain {
				t.Fatalf("feature/checkout = %s, want the resolved base origin/main %s", tip, originMain)
			}

			stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "recover", "--json", "spec/checkout")
			if code != 1 {
				t.Fatalf("verdi recover: exit %d, want 1\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
			}
			proj, err := recovery.Decode([]byte(strings.TrimRight(stdout, "\n")))
			if err != nil {
				t.Fatalf("recovery.Decode(stdout): %v\nstdout: %s", err, stdout)
			}
			var cut *recovery.RecognizedState
			for i, s := range proj.States {
				if s.Code == recovery.StateEmptyBranchCut && s.Target == "feature/checkout" {
					cut = &proj.States[i]
				}
			}
			if cut == nil {
				t.Fatalf("no empty-branch-cut state for feature/checkout: %+v", proj.States)
			}
			if len(cut.Choices) != 1 {
				t.Fatalf("Choices = %+v, uncertainties %+v; want the one unwind", cut.Choices, cut.Uncertainties)
			}
			choice := cut.Choices[0]
			if want := "switch back to main|delete feature/checkout with git branch -d"; strings.Join(choice.Effects, "|") != want {
				t.Fatalf("Effects = %v, want %q: a resolved-base cut returns to the re-resolved default branch, never to B", choice.Effects, want)
			}
			if !tt.wantUnwind {
				return
			}

			stdout, stderr, code = runVerdiBinary(t, bin, repo.Dir, env, "recover", "spec/checkout", "--apply", choice.ID)
			if code != 0 {
				t.Fatalf("verdi recover --apply %s: exit %d\nstdout:\n%s\nstderr:\n%s", choice.ID, code, stdout, stderr)
			}
			for _, want := range []string{"postcondition: feature/checkout does not exist: held", "postcondition: current branch is main: held"} {
				if !strings.Contains(stdout, want) {
					t.Fatalf("stdout = %q, want %q", stdout, want)
				}
			}
		})
	}
}
