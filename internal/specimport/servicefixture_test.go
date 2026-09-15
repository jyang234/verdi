package specimport

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// buildImportRepo returns a hermetic, minimally valid store root on branch
// main: a bare verdi.yaml (model.Canonical() resolves) and a .verdi/
// .gitignore excluding data/, so a checkout-cleanliness check never trips
// on verdi's own gitignored runtime state.
func buildImportRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml": "schema: verdi.layout/v1\n",
			".verdi/.gitignore": "data/\n",
		},
		Message: "seed store",
	}})
}

// fakeEngine is a fixed-value EngineIdentity: hermetic tests must never
// depend on the actual test binary's own digest, which changes on every
// rebuild.
type fakeEngine struct {
	digest string
	err    error
}

func (f fakeEngine) Digest(context.Context) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if f.digest == "" {
		return "fixed-engine-digest", nil
	}
	return f.digest, nil
}

// testService returns a Service wired for hermetic tests: a fixed engine
// digest (never the actual running test binary) and a policy source
// resolved to draft-write mode — the common case every non-policy-focused
// test needs a delegated agent to pass. Dedicated policy-matrix tests
// override svc.Policy directly.
func testService(t *testing.T) *Service {
	t.Helper()
	return &Service{Engine: fakeEngine{}, Policy: fakePolicySource{policy: resolvedPolicyFor(t, "draft-write")}}
}

// repoSnapshot is the working-tree/index/refs projection
// snapshotRepository/assertRepositoryEqual compare before and after a
// Preview or a refused/retried Apply — proving no checkout/index change and
// no new ref, per spec-import-contract.md's "No checkout/index change" and
// the plan's own core positive test shape.
type repoSnapshot struct {
	head   string
	refs   string
	status string
}

func snapshotRepository(t *testing.T, dir string) repoSnapshot {
	t.Helper()
	return repoSnapshot{
		head:   runGitCapture(t, dir, "rev-parse", "HEAD"),
		refs:   sortedLines(runGitCapture(t, dir, "for-each-ref", "--format=%(refname) %(objectname)")),
		status: runGitCapture(t, dir, "status", "--porcelain=v1", "-z", "--untracked-files=all"),
	}
}

func assertRepositoryEqual(t *testing.T, before, after repoSnapshot) {
	t.Helper()
	if before != after {
		t.Fatalf("repository changed:\nbefore: %+v\nafter:  %+v", before, after)
	}
}

func sortedLines(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func runGitCapture(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out.String())
	}
	return out.String()
}

// runGitFixture runs a git command against dir, failing the test on error —
// used by tests that need to dirty a fixture checkout (an untracked file, a
// staged change) to exercise the cleanliness gate.
func runGitFixture(t *testing.T, dir string, args ...string) {
	t.Helper()
	runGitCapture(t, dir, args...)
}

func joinPath(dir string, parts ...string) string {
	return filepath.Join(append([]string{dir}, parts...)...)
}
