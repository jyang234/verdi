package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/specstate"
)

// TestBuildStart_AdoptedStoreRunsTheGateOnlyAtTheBase is ledger SI-336:
// in an adopted store the conflict gate judges the checkout, so build start
// runs it only when HEAD is the resolved base commit, and otherwise refuses
// with exit 2 before any effect, naming HEAD and the base — even when
// HEAD's tree differs only outside every governed path, or not at all.
// The provider here passes every request, so a gate that ran where it must
// not would cut a branch instead of refusing. An unadopted store runs no
// gate and still cuts at the base while HEAD is elsewhere (UAT-023).
func TestBuildStart_AdoptedStoreRunsTheGateOnlyAtTheBase(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "")
	story := buildStartStoreFiles()
	adopted := contextPolicyStoreFiles(t)
	for rel, content := range story {
		adopted[rel] = content
	}
	tests := []struct {
		name        string
		store       map[string]string
		head        func(t *testing.T, git func(...string) string) // moves HEAD off main, or leaves it
		wantExit    int
		wantCalls   int
		wantRefusal bool
	}{
		{"(a) adopted, HEAD elsewhere with a tree differing only outside the governed paths", adopted, func(t *testing.T, git func(...string) string) {
			git("checkout", "-q", "-b", "elsewhere")
			writeTestFile(t, filepath.Join(strings.TrimSpace(git("rev-parse", "--show-toplevel")), "docs", "notes.md"), []byte("only on elsewhere\n"))
			git("add", "--", "docs/notes.md")
			git("commit", "-q", "-m", "elsewhere", "--", "docs/notes.md")
		}, 2, 0, true},
		{"(b) adopted, HEAD a different commit with the base's tree", adopted, func(_ *testing.T, git func(...string) string) {
			git("checkout", "-q", "-b", "elsewhere")
			git("commit", "-q", "--allow-empty", "-m", "the same tree, another commit")
		}, 2, 0, true},
		{"(c) adopted, HEAD at the base", adopted, func(*testing.T, func(...string) string) {}, 0, 1, false},
		{"(d) unadopted, HEAD elsewhere (UAT-023)", story, func(t *testing.T, git func(...string) string) {
			git("checkout", "-q", "-b", "elsewhere")
			writeTestFile(t, filepath.Join(strings.TrimSpace(git("rev-parse", "--show-toplevel")), "docs", "notes.md"), []byte("only on elsewhere\n"))
			git("add", "--", "docs/notes.md")
			git("commit", "-q", "-m", "elsewhere", "--", "docs/notes.md")
		}, 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := fixturegit.Build(t, []fixturegit.Layer{{Files: tt.store, Message: "accepted story"}})
			git := func(args ...string) string { return gitTestOutput(t, repo.Dir, args...) }
			bare := t.TempDir()
			gitTestOutput(t, bare, "init", "-q", "--bare", "--initial-branch=main")
			git("remote", "add", "origin", bare)
			git("push", "-q", "origin", "main")
			git("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
			base := strings.TrimSpace(git("rev-parse", "origin/main"))
			tt.head(t, git)
			head := strings.TrimSpace(git("rev-parse", "HEAD"))
			requestPath := ""
			if _, isAdopted := tt.store[".verdi/policy/constitution.md"]; isAdopted {
				requestPath = contextLifecycleRequestFile(t, repo.Dir, "build-start-context.json", "spec/widget-story", contextcompile.PhaseBuild, nil)
			}
			before := []string{
				git("for-each-ref", "--format=%(refname) %(objectname)"),
				git("rev-parse", "--symbolic-full-name", "HEAD"),
				git("ls-files", "--stage"),
				git("status", "--porcelain=v1", "--untracked-files=all"),
				git("config", "--local", "--list"),
			}

			calls := 0
			provider := contextConflictProviderFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
				calls++
				return lifecycleConflictResult(policyconflict.VerdictPass), nil
			})
			var stdout, stderr bytes.Buffer
			got := runBuildStartWithConflict(context.Background(), repo.Dir, "spec/widget-story",
				fakeScaffoldResolver{result: specstate.Result{State: specstate.AcceptedPendingBuild}},
				syncDeps{Runner: nil, GoTest: fakeGoTest{}, Model: phase7Model(t)}, requestPath, provider, &stdout, &stderr)
			if got != tt.wantExit || calls != tt.wantCalls {
				t.Fatalf("runBuildStartWithConflict = %d with %d gate call(s), want %d with %d; stdout=%s stderr=%s", got, calls, tt.wantExit, tt.wantCalls, stdout.String(), stderr.String())
			}
			if tt.wantRefusal {
				for _, want := range []string{head, base, "the conflict gate judges the checkout"} {
					if !strings.Contains(stderr.String(), want) {
						t.Errorf("stderr = %q, want it to name %q", stderr.String(), want)
					}
				}
				after := []string{
					git("for-each-ref", "--format=%(refname) %(objectname)"),
					git("rev-parse", "--symbolic-full-name", "HEAD"),
					git("ls-files", "--stage"),
					git("status", "--porcelain=v1", "--untracked-files=all"),
					git("config", "--local", "--list"),
				}
				for i, what := range []string{"refs", "HEAD", "the index", "the working tree", "the configuration"} {
					if after[i] != before[i] {
						t.Errorf("the refusal changed %s:\nbefore %s\nafter  %s", what, before[i], after[i])
					}
				}
				return
			}
			if tip := strings.TrimSpace(git("rev-parse", "refs/heads/feature/widget-story")); tip != base {
				t.Fatalf("feature/widget-story = %s, want the base %s", tip, base)
			}
			if tt.wantCalls == 1 && !strings.Contains(stdout.String(), "constitutional conflict: state: "+string(policyconflict.VerdictPass)) {
				t.Fatalf("stdout = %q, want the gate's pass summary", stdout.String())
			}
		})
	}
}
