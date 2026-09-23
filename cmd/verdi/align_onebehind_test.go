package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/align"
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/upstream"
)

// oneBehindAlignSpec builds the minimal *artifact.SpecFrontmatter
// runAlignForSpec needs to resolve its own report path (specRef.Name) —
// neither of this file's tests ever reach align.Generate, so nothing else
// on the spec is consulted.
func oneBehindAlignSpec() *artifact.SpecFrontmatter {
	return &artifact.SpecFrontmatter{Base: artifact.Base{ID: "spec/" + oneBehindReportSpecName}}
}

// TestRunAlignForSpec_OneBehind_FreezeTrue is SI-231's runAlignForSpec
// register (ledger contract item 3, freeze=true half): close's freeze step
// on a HEAD that carries a committed one-behind report freezes it in
// place, preserving every disposition and keeping `covers` at HEAD's
// PARENT (the content-final head it audited) rather than overwriting it to
// HEAD — and never regenerates (a FakeRunner with no configured services
// would make any regeneration attempt visibly diverge Covers/Findings from
// what this test asserts).
func TestRunAlignForSpec_OneBehind_FreezeTrue(t *testing.T) {
	ctx := context.Background()
	repo := oneBehindBaseRepo(t)
	parent := repo.Head
	head := commitOneBehindReport(t, ctx, repo.Dir, oneBehindReportSpecName, oneBehindReportContent(parent, oneBehindDispositionedFindingYAML))

	deps := alignDeps{Runner: upstream.NewFakeRunner(), ModelDigest: testResolveModelDigest(t, repo.Dir)}
	var stdout, stderr bytes.Buffer
	got := runAlignForSpec(ctx, repo.Dir, oneBehindAlignSpec(), head, true, deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runAlignForSpec(freeze=true, one-behind) = %d, want 0; stderr=%s", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), "SI-231") {
		t.Fatalf("stdout = %q, want it to name SI-231", stdout.String())
	}

	reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, oneBehindReportSpecName)
	decoded := decodeReportFile(t, reportPath)
	if decoded.Frozen == nil {
		t.Fatal("no Frozen stamp after freeze=true on the one-behind shape")
	}
	if decoded.Covers != parent {
		t.Fatalf("Covers = %q, want HEAD's parent %q (the content-final head it audited), not HEAD %q", decoded.Covers, parent, head)
	}
	if len(decoded.Findings) != 1 || decoded.Findings[0].ID != "f-1" || decoded.Findings[0].Disposition != artifact.FindingFixed {
		t.Fatalf("Findings = %+v, want the single committed f-1/fixed finding preserved verbatim", decoded.Findings)
	}
}

// TestRunAlignForSpec_OneBehind_FreezeFalse is SI-231's runAlignForSpec
// register (ledger contract item 3, freeze=false half): --prepare, the one
// freeze=false caller of runAlignForSpec, on a HEAD that carries a committed
// one-behind report leaves the file byte-identical and reports it current
// under SI-231 — never regenerating over it, never calling the judge. Bare
// `verdi align` is outside SI-231 (R-W1-9): see
// TestRunAlign_BareAlignIgnoresOneBehind.
func TestRunAlignForSpec_OneBehind_FreezeFalse(t *testing.T) {
	ctx := context.Background()
	repo := oneBehindBaseRepo(t)
	parent := repo.Head
	head := commitOneBehindReport(t, ctx, repo.Dir, oneBehindReportSpecName, oneBehindReportContent(parent, oneBehindDispositionedFindingYAML))

	reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, oneBehindReportSpecName)
	before, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("reading committed report before the run: %v", err)
	}

	deps := alignDeps{Runner: upstream.NewFakeRunner(), ModelDigest: testResolveModelDigest(t, repo.Dir)}
	var stdout, stderr bytes.Buffer
	got := runAlignForSpec(ctx, repo.Dir, oneBehindAlignSpec(), head, false, deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runAlignForSpec(freeze=false, one-behind) = %d, want 0; stderr=%s", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), "SI-231") || !strings.Contains(stdout.String(), "current") {
		t.Fatalf("stdout = %q, want it to report the report current under SI-231", stdout.String())
	}

	after, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("reading report after the run: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("report changed across a freeze=false one-behind run:\nbefore=%s\nafter=%s", before, after)
	}
}

// TestRunAlignForSpec_OneBehind_WorkingTreeDivergence is SI-231's
// working-tree clause at the shared align fork (ledger row as amended at
// L3b review I-1, ruling R-W1-9): when the operator's deviation-report.md
// differs from the report HEAD commits, neither half of the fork may claim
// SI-231 over it — freeze=true must never stamp HEAD's bytes over the
// operator's and print "dispositions preserved", and freeze=false must
// never call the file "left byte-identical" while describing HEAD's
// content. Both fall through to the fork's pre-SI-231 behavior instead.
func TestRunAlignForSpec_OneBehind_WorkingTreeDivergence(t *testing.T) {
	ctx := context.Background()
	for _, freeze := range []bool{true, false} {
		t.Run(map[bool]string{true: "freeze=true", false: "freeze=false"}[freeze], func(t *testing.T) {
			repo := oneBehindBaseRepo(t)
			parent := repo.Head
			head := commitOneBehindReport(t, ctx, repo.Dir, oneBehindReportSpecName, oneBehindRenderedReport(t, parent, artifact.FindingFixed, ""))
			reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, oneBehindReportSpecName)
			writeOneBehindFile(t, reportPath, oneBehindRenderedReport(t, parent, artifact.FindingAcceptedDeviation, "not fixed after all"))

			deps := alignDeps{Runner: upstream.NewFakeRunner(), ModelDigest: testResolveModelDigest(t, repo.Dir)}
			var stdout, stderr bytes.Buffer
			rc := runAlignForSpec(ctx, repo.Dir, oneBehindAlignSpec(), head, freeze, deps, &stdout, &stderr)
			t.Logf("runAlignForSpec(freeze=%v) = %d; stdout=%q stderr=%q", freeze, rc, stdout.String(), stderr.String())
			for _, claim := range []string{"SI-231", "dispositions preserved", "left byte-identical"} {
				if strings.Contains(stdout.String(), claim) {
					t.Fatalf("stdout = %q, want no %q claim over a working-tree report that differs from HEAD's", stdout.String(), claim)
				}
			}
			after := decodeReportFile(t, reportPath)
			if len(after.Findings) == 1 && after.Findings[0].ID == "f-1" && after.Findings[0].Disposition == artifact.FindingFixed {
				t.Fatalf("the operator's working-tree disposition (accepted-deviation) was replaced by HEAD's committed one (fixed): %+v", after.Findings)
			}
		})
	}
}

// TestRunAlign_BareAlignIgnoresOneBehind is SI-231's scope boundary (ledger
// row as narrowed at L3b review m-2, ruling R-W1-9): the one-behind
// recognition belongs to the closure ritual — close's freeze and `verdi
// close --prepare` — never to bare `verdi align`, whose merge-gate role (03
// §Gates: gate-then-commit) keeps its behavior at base 3e844905. At a HEAD
// that IS SI-231's accepted shape, bare align therefore still regenerates:
// without --freeze it rewrites the report to cover HEAD (so the merge gate's
// own "run `verdi align` again" remedy converges), and with --freeze it
// freezes a regenerated report at HEAD. Neither run claims SI-231.
func TestRunAlign_BareAlignIgnoresOneBehind(t *testing.T) {
	ctx := context.Background()
	for _, freeze := range []bool{false, true} {
		t.Run(map[bool]string{false: "align", true: "align --freeze"}[freeze], func(t *testing.T) {
			repo := buildAlignRepo(t)
			svcDir := filepath.Join(repo.Dir, "loansvc")
			reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, "stale-decline")

			living := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeOK(t), ModelDigest: testResolveModelDigest(t, repo.Dir)}
			var out, errb bytes.Buffer
			if got := runAlign(ctx, repo.Dir, false, living, &out, &errb); got != 0 {
				t.Fatalf("runAlign (living) = %d, want 0; stderr=%s", got, errb.String())
			}
			fm := decodeReportFile(t, reportPath)
			for i := range fm.Findings {
				fm.Findings[i].Disposition = artifact.FindingFixed
			}
			raw, err := os.ReadFile(reportPath)
			if err != nil {
				t.Fatal(err)
			}
			_, body, err := artifact.SplitFrontmatter(raw)
			if err != nil {
				t.Fatalf("SplitFrontmatter: %v", err)
			}
			head := commitOneBehindReport(t, ctx, repo.Dir, "stale-decline", string(align.RenderMarkdown(fm, string(body))))
			if accepted, err := evaluateOneBehindReport(ctx, repo.Dir, "stale-decline", head); err != nil || !accepted.Accepted {
				t.Fatalf("fixture: evaluateOneBehindReport = %+v, %v — want SI-231's accepted shape, so this test proves bare align ignores it", accepted, err)
			}

			deps := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeOK(t), ModelDigest: testResolveModelDigest(t, repo.Dir)}
			var stdout, stderr bytes.Buffer
			if got := runAlign(ctx, repo.Dir, freeze, deps, &stdout, &stderr); got != 0 {
				t.Fatalf("runAlign(freeze=%v) at a one-behind HEAD = %d, want 0; stderr=%s", freeze, got, stderr.String())
			}
			if strings.Contains(stdout.String(), "SI-231") {
				t.Fatalf("stdout = %q, want no SI-231 claim from bare align", stdout.String())
			}
			if !strings.Contains(stdout.String(), "judged finding(s)") {
				t.Fatalf("stdout = %q, want the regenerate path's tally line (bare align regenerates, as at base)", stdout.String())
			}
			after := decodeReportFile(t, reportPath)
			if after.Covers != head {
				t.Fatalf("Covers = %q, want HEAD %q — bare align regenerates to cover HEAD, as at base", after.Covers, head)
			}
			if freeze && (after.Frozen == nil || after.Frozen.Commit != head) {
				t.Fatalf("Frozen = %+v, want a regenerated report frozen at HEAD %s", after.Frozen, head)
			}
			if !freeze && after.Frozen != nil {
				t.Fatalf("Frozen = %+v, want no freeze without --freeze", after.Frozen)
			}
			if !freeze {
				// The reviewer's loop witness, closed: the merge gate's condition
				// 3 says "run `verdi align` again", and doing so now converges.
				cond3, err := checkFreshFullyDispositioned(repo.Dir, "stale-decline", head)
				if err != nil {
					t.Fatalf("checkFreshFullyDispositioned: %v", err)
				}
				if !cond3.OK {
					t.Fatalf("merge gate condition 3 after bare align = %q, want it satisfied (bare align regenerated to cover HEAD, carrying the dispositions)", cond3.Reason)
				}
			}
		})
	}
}
