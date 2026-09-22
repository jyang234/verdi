package main

import (
	"context"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/store"
)

// TestCloseBuiltBinary_OneBehindCommittedReport is SI-231's built-binary
// register (ledger contract item 4, "built-binary close"): a story whose
// close branch HEAD is a committed one-behind report commit — exactly the
// shape a CI close checkout produces, since it holds only committed state
// — with fresh CI-shaped evidence recorded at that exact HEAD, passes
// closure condition 4 and freezes the committed report verbatim, keeping
// `covers` at HEAD's parent. Reuses the existing close e2e harness
// (closeexperiment_test.go's production fixture, forge countersign server,
// and built-binary driver) rather than a bespoke one, isolating this test
// to the one thing SI-231 changes: the deviation report's own commit shape.
func TestCloseBuiltBinary_OneBehindCommittedReport(t *testing.T) {
	ctx := context.Background()
	bin := buildVerdiBinary(t)

	repo := buildCloseExperimentProductionFixtureRepo(t, nil)
	parent := repo.Head

	// Commit R: the ONLY change from parent is the dispositioned
	// deviation-report.md — exactly 03 §Closure ritual step 1's "a
	// manually triggered CI job" shape, and SI-231's own accepted commit
	// topology. The candidate branch's tip becomes R; repo.Head is updated
	// to match, mirroring buildCloseExperimentProductionFixtureRepo's own
	// convention for keeping the field in sync with the branch tip.
	head := commitOneBehindReport(t, ctx, repo.Dir, "exp-spike", oneBehindReportContent(parent, oneBehindDispositionedFindingYAML))
	repo.Head = head

	// Fresh CI-shaped evidence recorded for R itself — the exact commit
	// being closed — not for R's parent: proving the closure gate's
	// eligibility fold reads evidence at HEAD, whose code is identical to
	// the content-final parent the report audited (SI-231 ledger option
	// (a): "the fold's evidence is evaluated at HEAD").
	writeFixtureVerdicts(t, repo.Dir, "spec/exp-spike", head, featureFixtureEvidenceJSON("ac-1", "static", "pass", head))

	startCloseExperimentCountersignForge(t, repo)

	stdout, stderr, code := runExperimentBuiltBinary(t, bin, repo.Dir, nil, "close", "--force-local", "spec/exp-spike")
	if code != 0 {
		t.Fatalf("close (one-behind committed report) = %d, want 0; stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "[PASS] closure: 4.") {
		t.Fatalf("stdout = %q, want closure condition 4 to PASS", stdout)
	}

	archivedPath := store.DeviationReportPath(repo.Dir, store.ZoneArchive, "exp-spike")
	archived := decodeReportFile(t, archivedPath)
	if archived.Frozen == nil {
		t.Fatalf("archived report at %s carries no Frozen stamp", archivedPath)
	}
	if archived.Covers != parent {
		t.Fatalf("archived report Covers = %q, want HEAD's parent %q (the content-final head it audited), not HEAD %q", archived.Covers, parent, head)
	}
	if len(archived.Findings) != 1 || archived.Findings[0].ID != "f-1" || !archived.Findings[0].Dispositioned() {
		t.Fatalf("archived Findings = %+v, want the single committed, dispositioned f-1 finding preserved verbatim", archived.Findings)
	}
}
