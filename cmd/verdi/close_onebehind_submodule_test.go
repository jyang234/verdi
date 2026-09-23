package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// buildOneBehindSubmoduleBumpCloseRepo is buildOneBehindCloseRepo's
// production fixture (the waiver removed, ac-1's obligation elaborated at
// H, a CI-shaped `pass` record at R) with one difference, L3b re-review
// RR-8's shape: H also records the submodule lib at commit A under a
// COMMITTED .gitmodules carrying `ignore = all`, and R commits the report
// together with lib bumped to commit B. H carries no report.
func buildOneBehindSubmoduleBumpCloseRepo(t *testing.T, ctx context.Context) (repo *fixturegit.Repo, parent, head, libB string) {
	t.Helper()
	repo = buildCloseExperimentProductionFixtureRepo(t, nil)
	if err := os.Remove(store.WaiverPath(repo.Dir, closeExperimentWaiverSlug, "ac-1")); err != nil {
		t.Fatalf("removing the shared fixture's ac-1 waiver: %v", err)
	}
	libDir, libA, libB := oneBehindLibRepo(t)
	obligationRel := ".verdi/obligations/exp-spike/ac-1--static.md"
	closeExperimentWriteFixtureFile(t, repo.Dir, obligationRel, fixtureElaboratedObligationMD("exp-spike", "ac-1", artifact.EvidenceStatic, "fixture-static", "1", gateFakeFrozenCommit))
	closeExperimentWriteFixtureFile(t, repo.Dir, ".gitmodules", fmt.Sprintf(oneBehindIgnoringGitmodules, libDir))
	runGitCmd(t, repo.Dir, "add", "--", obligationRel, ".gitmodules")
	stageOneBehindGitlink(t, repo.Dir, libA)
	runGitCmd(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "elaborate exp-spike's ac-1 obligation; record lib at commit A")
	parent = strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "HEAD"))

	head = commitOneBehindReportWithGitlinkBump(t, repo.Dir, "exp-spike", oneBehindRenderedReport(t, parent, artifact.FindingFixed, ""), libB)
	repo.Head = head
	writeFixtureVerdicts(t, repo.Dir, "spec/exp-spike", head, featureFixtureEvidenceJSON("ac-1", "static", "pass", head))
	startCloseExperimentCountersignForge(t, repo)
	return repo, parent, head, libB
}

// TestCloseBuiltBinary_OneBehindSubmoduleBumpRefuses is L3b re-review RR-8
// made permanent (N-1, controller ruling R-W1-10): R adds the report AND
// bumps the submodule lib, and a committed .gitmodules `ignore = all` hides
// the bump from plain `git diff`. SI-231 requires HEAD's commit to change
// nothing but the report, so the real binary's close must refuse at closure
// condition 4, naming the path-count clause, and archive nothing, leaving
// HEAD and the working tree as they were. Before gitx passed
// --ignore-submodules=none, the same run exited 0, froze the report "under
// SI-231 … dispositions preserved", and archived covers = H while HEAD's
// code differed.
func TestCloseBuiltBinary_OneBehindSubmoduleBumpRefuses(t *testing.T) {
	ctx := context.Background()
	bin := buildVerdiBinary(t)
	repo, parent, head, libB := buildOneBehindSubmoduleBumpCloseRepo(t, ctx)
	reportRel := store.DeviationReportRelPath(store.ZoneActive, "exp-spike")
	assertOneBehindSubmoduleFixture(t, repo.Dir, parent, head, reportRel, "A\t"+reportRel)

	activePath := store.DeviationReportPath(repo.Dir, store.ZoneActive, "exp-spike")
	before, err := os.ReadFile(activePath)
	if err != nil {
		t.Fatalf("reading the working-tree report: %v", err)
	}
	// .verdi/data/ is Verdi's own runtime state, never committed.
	statusArgs := []string{"status", "--porcelain", "--untracked-files=all", "--ignore-submodules=none", "--", ".", ":(exclude).verdi/data"}
	statusBefore := gitOutput(t, repo.Dir, statusArgs...)
	branchBefore := gitOutput(t, repo.Dir, "rev-parse", "--abbrev-ref", "HEAD")

	stdout, stderr, code := runExperimentBuiltBinary(t, bin, repo.Dir, nil, "close", "--force-local", "spec/exp-spike")
	if code != 1 {
		t.Fatalf("close (R also bumps a submodule hidden by ignore = all) = %d, want 1 (a verdict refusal); stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "[FAIL] closure: 4.") {
		t.Fatalf("stdout = %q, want closure condition 4 to FAIL", stdout)
	}
	if want := parent + ".." + head + " changes 2 path(s), not exactly one"; !strings.Contains(stdout, want) {
		t.Fatalf("stdout = %q, want condition 4 to name the path-count clause %q", stdout, want)
	}
	for _, claim := range []string{"dispositions preserved", "under SI-231 (a committed", "archived spec/exp-spike"} {
		if strings.Contains(stdout, claim) {
			t.Fatalf("stdout = %q, want no %q claim from a refused close", stdout, claim)
		}
	}

	archivedPath := store.DeviationReportPath(repo.Dir, store.ZoneArchive, "exp-spike")
	if _, err := os.Stat(archivedPath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("stat %s = %v, want it absent — a refused close archives nothing", archivedPath, err)
	}
	if _, err := os.Stat(filepath.Dir(archivedPath)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("stat %s = %v, want no archive directory at all", filepath.Dir(archivedPath), err)
	}
	if got := strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "HEAD")); got != head {
		t.Fatalf("HEAD = %s after a refused close, want R %s unchanged", got, head)
	}
	if got := gitOutput(t, repo.Dir, "rev-parse", "--abbrev-ref", "HEAD"); got != branchBefore {
		t.Fatalf("checked-out branch = %q after a refused close, want %q unchanged", got, branchBefore)
	}
	if got, err := gitx.RevParse(ctx, repo.Dir, "HEAD:lib"); err != nil || got != libB {
		t.Fatalf("HEAD:lib = %q, %v, want R's gitlink %s unchanged", got, err, libB)
	}
	after, err := os.ReadFile(activePath)
	if err != nil {
		t.Fatalf("reading the working-tree report after close: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("the refused close rewrote the working-tree report:\nbefore=%s\nafter=%s", before, after)
	}
	if statusAfter := gitOutput(t, repo.Dir, statusArgs...); statusAfter != statusBefore {
		t.Fatalf("working tree after a refused close:\n%s\nwant it unchanged from:\n%s", statusAfter, statusBefore)
	}
}
