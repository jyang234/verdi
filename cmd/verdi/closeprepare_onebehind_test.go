package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	forgefake "github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/upstream"
)

// TestRunPrepare_OneBehindCommittedReport is item 4 of the ledger contract's
// census: `verdi close --prepare` is one of the three named callers of the
// shared align fork (runAlignForSpec), and its OWN disclosure/message logic
// reads report.Covers != head independently of that fork. Before this fix,
// runPrepare unconditionally disclosed "regenerated dispositions" and
// printed "ALIGNMENT REQUIRED ... the existing align engine refreshed it"
// whenever the on-disk report's covers didn't equal HEAD — both FALSE for a
// committed one-behind report, which runAlignForSpec's own SI-231 branch
// leaves byte-identical and never regenerates.
func TestRunPrepare_OneBehindCommittedReport(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := buildCloseFixtureRepo(t)
	parent := repo.Head
	commitOneBehindReport(t, ctx, repo.Dir, "close-fixture", oneBehindReportContent(parent, oneBehindDispositionedFindingYAML))

	deps := closeDeps{Runner: upstream.NewFakeRunner(), Forge: forgefake.New(), JudgeCmd: alignFakeJudgeOK(t)}
	var stdout, stderr bytes.Buffer
	runPrepare(ctx, repo.Dir, "spec/close-fixture", &store.Manifest{}, deps, true, &stdout, &stderr)

	if strings.Contains(stdout.String(), "ALIGNMENT REQUIRED") {
		t.Fatalf("stdout = %q, want no ALIGNMENT REQUIRED — a committed one-behind report needs no alignment", stdout.String())
	}
	if !strings.Contains(stdout.String(), "SI-231") {
		t.Fatalf("stdout = %q, want it to report the committed report current under SI-231", stdout.String())
	}
	if strings.Contains(stdout.String(), "regenerated") || strings.Contains(stdout.String(), "reversed to") {
		t.Fatalf("stdout = %q, want no regenerated-dispositions disclosure — nothing was regenerated", stdout.String())
	}
}

// TestRunPrepare_OneBehindWorkingTreeDivergence is SI-231's working-tree
// clause at --prepare (ledger row as amended at L3b review I-1, ruling
// R-W1-9): after commit R, a working-tree report that differs from R's is
// not the one-behind shape, so --prepare never calls it current under
// SI-231 or "left byte-identical"; it takes the stale-report path it takes
// at base, disclosing what the refresh may cost.
func TestRunPrepare_OneBehindWorkingTreeDivergence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := buildCloseFixtureRepo(t)
	parent := repo.Head
	commitOneBehindReport(t, ctx, repo.Dir, "close-fixture", oneBehindRenderedReport(t, parent, artifact.FindingFixed, ""))
	writeOneBehindFile(t, store.DeviationReportPath(repo.Dir, store.ZoneActive, "close-fixture"), oneBehindRenderedReport(t, parent, artifact.FindingAcceptedDeviation, "not fixed after all"))

	deps := closeDeps{Runner: upstream.NewFakeRunner(), Forge: forgefake.New(), JudgeCmd: alignFakeJudgeOK(t)}
	var stdout, stderr bytes.Buffer
	runPrepare(ctx, repo.Dir, "spec/close-fixture", &store.Manifest{}, deps, true, &stdout, &stderr)

	for _, claim := range []string{"SI-231", "left byte-identical"} {
		if strings.Contains(stdout.String(), claim) {
			t.Fatalf("stdout = %q, want no %q claim over a working-tree report that differs from R's", stdout.String(), claim)
		}
	}
	if !strings.Contains(stdout.String(), "ALIGNMENT REQUIRED") {
		t.Fatalf("stdout = %q, want the stale-report path's ALIGNMENT REQUIRED line", stdout.String())
	}
}
