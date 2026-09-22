package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

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
// register (ledger contract item 3, freeze=false half): --prepare and bare
// `verdi align` on a HEAD that carries a committed one-behind report leave
// the file byte-identical and report it current under SI-231 — never
// regenerating over it, never calling the judge.
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
