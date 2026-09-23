package main

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
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

// TestCloseBuiltBinary_OneBehindWorkingTreeDivergenceRefuses is SI-231's
// working-tree clause end to end (ledger row as amended at L3b review I-1,
// ruling R-W1-9; the reviewer's built-binary witness made permanent): after
// commit R, an operator's uncommitted change to the report — through the
// sanctioned `verdi disposition --amend` verb, or a hand retraction — makes
// the real binary's close refuse at closure condition 4, naming the
// divergence, and archive nothing. Before the clause, the same run passed
// condition 4, froze R's bytes over the operator's change, printed
// "dispositions preserved", and archived the discarded disposition.
func TestCloseBuiltBinary_OneBehindWorkingTreeDivergenceRefuses(t *testing.T) {
	ctx := context.Background()
	bin := buildVerdiBinary(t)

	cases := []struct {
		name    string
		diverge func(t *testing.T, repoDir, reportPath, parent string)
	}{
		{
			name: "an uncommitted verdi disposition --amend",
			diverge: func(t *testing.T, repoDir, _, _ string) {
				stdout, stderr, code := runExperimentBuiltBinary(t, bin, repoDir, nil, "disposition", "spec/exp-spike", "f-1", "accepted-deviation", "--rationale", "not fixed after all", "--amend")
				if code != 0 {
					t.Fatalf("disposition --amend = %d, want 0; stdout=%s stderr=%s", code, stdout, stderr)
				}
			},
		},
		{
			name: "a working-tree retraction",
			diverge: func(t *testing.T, _, reportPath, parent string) {
				writeOneBehindFile(t, reportPath, oneBehindRenderedReport(t, parent, "", ""))
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := buildCloseExperimentProductionFixtureRepo(t, nil)
			parent := repo.Head
			head := commitOneBehindReport(t, ctx, repo.Dir, "exp-spike", oneBehindRenderedReport(t, parent, artifact.FindingFixed, ""))
			repo.Head = head
			writeFixtureVerdicts(t, repo.Dir, "spec/exp-spike", head, featureFixtureEvidenceJSON("ac-1", "static", "pass", head))
			startCloseExperimentCountersignForge(t, repo)

			activePath := store.DeviationReportPath(repo.Dir, store.ZoneActive, "exp-spike")
			tc.diverge(t, repo.Dir, activePath, parent)
			before, err := os.ReadFile(activePath)
			if err != nil {
				t.Fatalf("reading the diverged working-tree report: %v", err)
			}

			stdout, stderr, code := runExperimentBuiltBinary(t, bin, repo.Dir, nil, "close", "--force-local", "spec/exp-spike")
			if code != 1 {
				t.Fatalf("close (working tree differs from R) = %d, want 1 (a verdict refusal); stdout=%s stderr=%s", code, stdout, stderr)
			}
			if !strings.Contains(stdout, "[FAIL] closure: 4.") {
				t.Fatalf("stdout = %q, want closure condition 4 to FAIL", stdout)
			}
			if !strings.Contains(stdout, oneBehindWorkingTreeDivergence) {
				t.Fatalf("stdout = %q, want condition 4 to name the working-tree clause %q", stdout, oneBehindWorkingTreeDivergence)
			}
			if strings.Contains(stdout, "dispositions preserved") {
				t.Fatalf("stdout = %q, want no freeze claim at all", stdout)
			}

			archivedPath := store.DeviationReportPath(repo.Dir, store.ZoneArchive, "exp-spike")
			if _, err := os.Stat(archivedPath); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("stat %s = %v, want it absent — a refused close archives nothing", archivedPath, err)
			}
			after, err := os.ReadFile(activePath)
			if err != nil {
				t.Fatalf("reading the working-tree report after close: %v", err)
			}
			if !bytes.Equal(before, after) {
				t.Fatalf("the refused close rewrote the operator's working-tree report:\nbefore=%s\nafter=%s", before, after)
			}
			if got := strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "HEAD")); got != head {
				t.Fatalf("HEAD = %s after a refused close, want R %s unchanged", got, head)
			}
		})
	}
}
