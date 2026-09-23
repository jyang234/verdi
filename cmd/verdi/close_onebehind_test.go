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
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// buildOneBehindCloseRepo is the built-binary SI-231 fixture: the close
// e2e harness's production fixture (closeexperiment_test.go: accepted
// profile, countersign forge, the jira fake), then the candidate commit H,
// then commit R on top — R's ONLY change from H is exp-spike's
// deviation-report.md, covering H with every finding dispositioned —
// exactly 03 §Closure ritual step 1's "a manually triggered CI job" shape.
// repo.Head is kept at the branch tip R.
//
// The shared fixture's ac-1 waiver is REMOVED, so closure condition 1 can
// hold only through evidence. H commits ac-1's elaborated obligation
// (fixtureElaboratedObligationMD), binding exactly the producer and CI job
// featureFixtureEvidenceJSON stamps — without it the fold reads ac-1 as
// obligation-quality "missing", and no record could ever evidence it.
// ac1Verdict then records one CI-shaped ac-1 record for R itself ("" records
// none), the record a close-branch push produces, which the fold at HEAD
// must read (SI-231 option (a): "the fold's evidence is evaluated at HEAD").
func buildOneBehindCloseRepo(t *testing.T, ctx context.Context, ac1Verdict string) (repo *fixturegit.Repo, parent, head string) {
	t.Helper()
	repo = buildCloseExperimentProductionFixtureRepo(t, nil)
	if err := os.Remove(store.WaiverPath(repo.Dir, closeExperimentWaiverSlug, "ac-1")); err != nil {
		t.Fatalf("removing the shared fixture's ac-1 waiver: %v", err)
	}
	obligationRel := ".verdi/obligations/exp-spike/ac-1--static.md"
	closeExperimentWriteFixtureFile(t, repo.Dir, obligationRel, fixtureElaboratedObligationMD("exp-spike", "ac-1", artifact.EvidenceStatic, "fixture-static", "1", gateFakeFrozenCommit))
	if err := gitx.AddPaths(ctx, repo.Dir, obligationRel); err != nil {
		t.Fatalf("AddPaths(%s): %v", obligationRel, err)
	}
	parent, err := gitx.CreateCommitPaths(ctx, repo.Dir, "elaborate exp-spike's ac-1 obligation", obligationRel)
	if err != nil {
		t.Fatalf("CreateCommitPaths(%s): %v", obligationRel, err)
	}
	head = commitOneBehindReport(t, ctx, repo.Dir, "exp-spike", oneBehindRenderedReport(t, parent, artifact.FindingFixed, ""))
	repo.Head = head
	if ac1Verdict != "" {
		writeFixtureVerdicts(t, repo.Dir, "spec/exp-spike", head, featureFixtureEvidenceJSON("ac-1", "static", ac1Verdict, head))
	}
	startCloseExperimentCountersignForge(t, repo)
	return repo, parent, head
}

// TestCloseBuiltBinary_OneBehindCommittedReport is SI-231's built-binary
// register (ledger contract item 4, "built-binary close"): a story whose
// close branch HEAD is a committed one-behind report commit — exactly the
// shape a CI close checkout produces, since it holds only committed state
// — passes closure condition 1 on the fresh CI-shaped evidence recorded at
// that exact HEAD (no waiver: see buildOneBehindCloseRepo), passes
// condition 4, and freezes the committed report verbatim, keeping `covers`
// at HEAD's parent. Its negative twin,
// TestCloseBuiltBinary_OneBehindEvidenceIsLoadBearing, proves condition 1
// really rests on that evidence.
func TestCloseBuiltBinary_OneBehindCommittedReport(t *testing.T) {
	ctx := context.Background()
	bin := buildVerdiBinary(t)
	repo, parent, head := buildOneBehindCloseRepo(t, ctx, "pass")

	stdout, stderr, code := runExperimentBuiltBinary(t, bin, repo.Dir, nil, "close", "--force-local", "spec/exp-spike")
	if code != 0 {
		t.Fatalf("close (one-behind committed report) = %d, want 0; stdout=%s stderr=%s", code, stdout, stderr)
	}
	for _, want := range []string{"[PASS] closure: 1.", "[PASS] closure: 4."} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout = %q, want %q", stdout, want)
		}
	}
	if strings.Contains(stdout, "waivers/") {
		t.Fatalf("stdout = %q, want no waiver folded — condition 1 must rest on the evidence at HEAD alone", stdout)
	}

	archivedPath := store.DeviationReportPath(repo.Dir, store.ZoneArchive, "exp-spike")
	archived := decodeReportFile(t, archivedPath)
	if archived.Frozen == nil {
		t.Fatalf("archived report at %s carries no Frozen stamp", archivedPath)
	}
	if archived.Covers != parent {
		t.Fatalf("archived report Covers = %q, want HEAD's parent %q (the content-final head it audited), not HEAD %q", archived.Covers, parent, head)
	}
	if len(archived.Findings) != 1 || archived.Findings[0].ID != "f-1" || archived.Findings[0].Disposition != artifact.FindingFixed {
		t.Fatalf("archived Findings = %+v, want the single committed f-1/fixed finding preserved verbatim", archived.Findings)
	}
}

// TestCloseBuiltBinary_OneBehindEvidenceIsLoadBearing is the proof's
// negative twin (L3b review I-2): with the same accepted one-behind report,
// a `fail` record at HEAD, or no record at all, leaves ac-1 unevidenced, so
// close refuses at condition 1 — while condition 4 still passes, isolating
// the refusal to the evidence — and archives nothing.
func TestCloseBuiltBinary_OneBehindEvidenceIsLoadBearing(t *testing.T) {
	ctx := context.Background()
	bin := buildVerdiBinary(t)
	for _, tc := range []struct {
		name       string
		ac1Verdict string
	}{
		{name: "a fail record at HEAD", ac1Verdict: "fail"},
		{name: "no evidence record at all", ac1Verdict: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, _, head := buildOneBehindCloseRepo(t, ctx, tc.ac1Verdict)

			stdout, stderr, code := runExperimentBuiltBinary(t, bin, repo.Dir, nil, "close", "--force-local", "spec/exp-spike")
			if code != 1 {
				t.Fatalf("close = %d, want 1 (a verdict refusal on the evidence); stdout=%s stderr=%s", code, stdout, stderr)
			}
			for _, want := range []string{"[FAIL] closure: 1.", "[PASS] closure: 4."} {
				if !strings.Contains(stdout, want) {
					t.Fatalf("stdout = %q, want %q", stdout, want)
				}
			}
			archivedPath := store.DeviationReportPath(repo.Dir, store.ZoneArchive, "exp-spike")
			if _, err := os.Stat(archivedPath); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("stat %s = %v, want it absent — a refused close archives nothing", archivedPath, err)
			}
			if got := strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "HEAD")); got != head {
				t.Fatalf("HEAD = %s after a refused close, want R %s unchanged", got, head)
			}
		})
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
			repo, parent, head := buildOneBehindCloseRepo(t, ctx, "pass")

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
