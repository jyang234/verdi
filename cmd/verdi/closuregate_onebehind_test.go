package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
)

// TestCheckDispositionCompleteCondition_OneBehind is SI-231's closure-gate
// register (ledger contract item 2): condition 4 passes when either today's
// rule holds (a living report covering HEAD, all dispositioned — proven
// unaffected by TestRunClosureGate_DispositionCompleteCondition's own
// "fresh, fully dispositioned" case) OR evaluateOneBehindReport accepts;
// every refusal names a failed clause.
func TestCheckDispositionCompleteCondition_OneBehind(t *testing.T) {
	ctx := context.Background()
	specOneBehind := &artifact.SpecFrontmatter{Base: artifact.Base{ID: "spec/" + oneBehindReportSpecName}}

	t.Run("accepted one-behind shape passes with no living report on disk", func(t *testing.T) {
		repo := oneBehindBaseRepo(t)
		parent := repo.Head
		head := commitOneBehindReport(t, ctx, repo.Dir, oneBehindReportSpecName, oneBehindReportContent(parent, oneBehindDispositionedFindingYAML))

		cond, err := checkDispositionCompleteCondition(ctx, repo.Dir, specOneBehind, head)
		if err != nil {
			t.Fatalf("checkDispositionCompleteCondition: %v", err)
		}
		if !cond.OK {
			t.Fatalf("cond.OK = false, want true; Reason=%q", cond.Reason)
		}
	})

	t.Run("merge-commit HEAD refuses naming both clauses", func(t *testing.T) {
		repo := oneBehindBaseRepo(t)
		checkoutBranch(t, repo.Dir, "side")
		if err := os.WriteFile(filepath.Join(repo.Dir, "side.txt"), []byte("side\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		commitAllOnCurrentBranch(t, repo.Dir, "side change")
		if err := gitx.Checkout(ctx, repo.Dir, "main"); err != nil {
			t.Fatalf("Checkout(main): %v", err)
		}
		if err := os.WriteFile(filepath.Join(repo.Dir, "main.txt"), []byte("main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		commitAllOnCurrentBranch(t, repo.Dir, "main change")
		runGitCmd(t, repo.Dir, "merge", "--quiet", "--no-ff", "-m", "merge side", "side")
		mergeHead, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
		if err != nil {
			t.Fatalf("RevParse(HEAD): %v", err)
		}

		cond, err := checkDispositionCompleteCondition(ctx, repo.Dir, specOneBehind, mergeHead)
		if err != nil {
			t.Fatalf("checkDispositionCompleteCondition: %v", err)
		}
		if cond.OK {
			t.Fatal("cond.OK = true, want false (no living report AND HEAD is a merge commit)")
		}
		if !strings.Contains(cond.Reason, "no deviation-report.md found at") {
			t.Fatalf("Reason = %q, want today's-rule clause named", cond.Reason)
		}
		if !strings.Contains(cond.Reason, "single-parent commit") {
			t.Fatalf("Reason = %q, want the one-behind clause named too", cond.Reason)
		}
	})

	// X-16 (ledger, closuregate.go's top doc comment): dispositions
	// committed TOGETHER WITH another file. The one-behind predicate's own
	// clause-2 refusal (proven directly in onebehind_test.go) must also
	// surface here, at the gate consumers actually call before `verdi
	// close` ever attempts to freeze anything — proving the X-16 trap is
	// caught before any freeze is reached, never silently regenerated.
	t.Run("X-16 shape (report committed together with another file) refuses naming the path-count clause", func(t *testing.T) {
		repo := oneBehindBaseRepo(t)
		parent := repo.Head
		dir := filepath.Join(repo.Dir, ".verdi", "specs", "active", oneBehindReportSpecName)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "deviation-report.md"), []byte(oneBehindReportContent(parent, oneBehindDispositionedFindingYAML)), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo.Dir, "other.txt"), []byte("other\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		head := commitAllOnCurrentBranch(t, repo.Dir, "report + unrelated file (X-16 shape)")

		cond, err := checkDispositionCompleteCondition(ctx, repo.Dir, specOneBehind, head)
		if err != nil {
			t.Fatalf("checkDispositionCompleteCondition: %v", err)
		}
		if cond.OK {
			t.Fatal("cond.OK = true, want false (two paths committed together)")
		}
		if !strings.Contains(cond.Reason, "2 path") {
			t.Fatalf("Reason = %q, want the path-count clause named", cond.Reason)
		}
	})
}
