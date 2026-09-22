package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

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
