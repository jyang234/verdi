package reclaim

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// runGit runs git in dir with a fixed author/committer identity (for any
// invocation that creates a commit) and fails the test on a non-zero exit
// — mirrors internal/wtmanager's and internal/residue's own per-package
// test helper of the same name/shape (CLAUDE.md precedent: each package
// that needs raw git beyond gitx's own wrapped primitives defines its own
// tiny copy, never a shared production dependency for test-only plumbing).
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Verdi Fixture", "GIT_AUTHOR_EMAIL=fixture@verdi.invalid",
		"GIT_COMMITTER_NAME=Verdi Fixture", "GIT_COMMITTER_EMAIL=fixture@verdi.invalid",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// checkoutMain returns root to its main branch.
func checkoutMain(t *testing.T, root string) {
	t.Helper()
	if err := gitx.Checkout(context.Background(), root, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}
}

// newReclaimTestRepo builds a fresh, minimal fixturegit repo on "main" —
// the shared starting point every test in this file cuts branches and
// worktrees against afterward.
func newReclaimTestRepo(t *testing.T) string {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/.gitignore": "data/\n"},
		Message: "root",
	}})
	return repo.Dir
}

// eligiblePair is one merged, clean, unmanaged branch+worktree unit cut
// against an existing repo root.
type eligiblePair struct {
	branch string
	path   string
	tip    string
}

// cutEligiblePair cuts a new design/<name> branch off main's current tip,
// commits one file on it, merges it (--no-ff) into main, cuts an UNMANAGED
// worktree for it (outside .verdi/data/worktrees, so it never reads
// Managed true), and returns to main — an AC-1-eligible worktree+branch
// unit, real, hermetic, no mocking (co-1). Reusable multiple times against
// the same root: each call starts from main's THEN-current tip.
func cutEligiblePair(t *testing.T, root, name string) eligiblePair {
	t.Helper()
	ctx := context.Background()
	branch := "design/" + name

	if err := gitx.CheckoutNewBranch(ctx, root, branch); err != nil {
		t.Fatalf("CheckoutNewBranch(%s): %v", branch, err)
	}
	if err := os.WriteFile(filepath.Join(root, name+".txt"), []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "add "+name)
	tip, err := gitx.RevParse(ctx, root, branch)
	if err != nil {
		t.Fatal(err)
	}
	checkoutMain(t, root)
	runGit(t, root, "merge", "--quiet", "--no-ff", "-m", "merge "+branch, branch)

	path := filepath.Join(t.TempDir(), name+"-wt")
	if err := gitx.WorktreeAdd(ctx, root, path, branch); err != nil {
		t.Fatalf("WorktreeAdd(%s): %v", branch, err)
	}
	return eligiblePair{branch: branch, path: path, tip: tip}
}

// planItemFor finds the PlanItem for branch, failing the test if absent.
func planItemFor(t *testing.T, plan Plan, branch string) PlanItem {
	t.Helper()
	for _, item := range plan.Items {
		if item.Unit.Branch == branch {
			return item
		}
	}
	t.Fatalf("no plan item for branch %q among %d items: %+v", branch, len(plan.Items), plan.Items)
	return PlanItem{}
}

// realOrSelf resolves symlinks in path, falling back to path unchanged if
// it cannot be resolved — mirrors internal/residue/survey_test.go's own
// realOrSelfSurvey helper (each package defines its own tiny test-only
// copy, CLAUDE.md's no-copy-paste rule binding production code, not this
// per-package test idiom already established at two sibling call sites).
// Needed wherever a test compares a path THIS test built (as passed to
// gitx.WorktreeAdd, unresolved) against one residue.Scan reports (git's
// own, already-resolved form) — the same macOS /var-vs-/private/var parity
// class internal/reclaim's own canonicalPath (predicate.go) exists to
// survive in production.
func realOrSelf(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}
