package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
)

const diagramSweepProposalMermaid = `---
id: diagram/loansvc-future
kind: diagram
class: proposal
title: "LoanSvc future topology (fixture)"
status: proposed
owners: [platform-team]
---
graph TD
  loansvc --> notification-svc
  loansvc --> new-outbox-consumer
`

// buildDiagramSweepRepo builds a one-layer fixturegit repo carrying a
// class: proposal diagram at .verdi/diagrams/loansvc-future.mermaid — the
// --diagram-sweep flag's own target.
func buildDiagramSweepRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                      "schema: verdi.layout/v1\nforge: gitlab\n",
				".verdi/diagrams/loansvc-future.mermaid": diagramSweepProposalMermaid,
			},
			Message: "scaffold + a class: proposal diagram",
		},
	})
}

// TestRunDiagramSweepAlign_WritesReport is spec/judged-sweep ac-1's
// behavioral obligation, first half: a CLI invocation over a fixture class:
// proposal diagram writes .verdi/diagrams/<name>.sweep-report.md with a
// well-formed verdi.diagramsweep/v1 frontmatter.
func TestRunDiagramSweepAlign_WritesReport(t *testing.T) {
	repo := buildDiagramSweepRepo(t)

	var stdout, stderr bytes.Buffer
	got := runDiagramSweepAlign(context.Background(), repo.Dir, "diagram/loansvc-future", alignDeps{ModelDigest: testResolveModelDigest(t, repo.Dir)}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runDiagramSweepAlign = %d, want 0; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}

	reportPath := filepath.Join(repo.Dir, ".verdi", "diagrams", "loansvc-future.sweep-report.md")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("reading %s: %v", reportPath, err)
	}
	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	decoded, err := artifact.DecodeDiagramSweep(fm)
	if err != nil {
		t.Fatalf("DecodeDiagramSweep: %v", err)
	}
	if decoded.Covers != repo.Head {
		t.Fatalf("Covers = %q, want %q", decoded.Covers, repo.Head)
	}
	// No judge configured: the synthetic absence finding, undispositioned.
	if len(decoded.Findings) != 1 {
		t.Fatalf("Findings = %+v, want exactly 1", decoded.Findings)
	}
	if !strings.Contains(stdout.String(), "loansvc-future.sweep-report.md") {
		t.Fatalf("stdout = %q, want it to name the written report path", stdout.String())
	}
}

// TestRunDiagramSweepAlign_NotAProposal_Refused proves a non-proposal
// diagram (class absent, the incumbent authored-living shape) is refused
// rather than swept — the sweep's own stated subject is a class: proposal
// diagram (spec/judged-sweep's outcome text).
func TestRunDiagramSweepAlign_NotAProposal_Refused(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                        "schema: verdi.layout/v1\nforge: gitlab\n",
				".verdi/diagrams/loansvc-topology.mermaid": "---\nid: diagram/loansvc-topology\nkind: diagram\ntitle: \"t\"\nstatus: active\nowners: [platform-team]\n---\ngraph TD\n  a --> b\n",
			},
			Message: "scaffold + an incumbent (non-proposal) diagram",
		},
	})

	var stdout, stderr bytes.Buffer
	got := runDiagramSweepAlign(context.Background(), repo.Dir, "diagram/loansvc-topology", alignDeps{}, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runDiagramSweepAlign = %d, want 1 (refused: not a proposal); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "diagrams", "loansvc-topology.sweep-report.md")); !os.IsNotExist(err) {
		t.Fatal("a refused sweep must not write a report file")
	}
}

// TestRunDiagramSweepAlign_ByteIdentity is spec/judged-sweep ac-4's
// behavioral obligation, first half: SHA-256 the target diagram file's full
// bytes before a real sweep run, run it, re-read, and assert byte-identity
// — the sweep never touched the diagram it read.
func TestRunDiagramSweepAlign_ByteIdentity(t *testing.T) {
	repo := buildDiagramSweepRepo(t)
	diagPath := filepath.Join(repo.Dir, ".verdi", "diagrams", "loansvc-future.mermaid")

	before, err := os.ReadFile(diagPath)
	if err != nil {
		t.Fatalf("reading diagram before sweep: %v", err)
	}
	beforeSHA := sha256.Sum256(before)

	var stdout, stderr bytes.Buffer
	got := runDiagramSweepAlign(context.Background(), repo.Dir, "diagram/loansvc-future", alignDeps{ModelDigest: testResolveModelDigest(t, repo.Dir)}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runDiagramSweepAlign = %d, want 0; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}

	after, err := os.ReadFile(diagPath)
	if err != nil {
		t.Fatalf("reading diagram after sweep: %v", err)
	}
	afterSHA := sha256.Sum256(after)

	if hex.EncodeToString(beforeSHA[:]) != hex.EncodeToString(afterSHA[:]) {
		t.Fatalf("diagram bytes changed after a sweep run: before=%x after=%x", beforeSHA, afterSHA)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("diagram bytes changed after a sweep run (byte comparison)")
	}
}

// TestGate_UnaffectedByDiagramSweepFinding is spec/judged-sweep ac-1's
// behavioral obligation, second half: verdi gate's pass/fail outcome is
// completely unaffected by a sweep-report.md carrying an undispositioned
// finding — proving co-1's "never in any gate's deterministic path"
// behaviorally, not just by source absence.
func TestGate_UnaffectedByDiagramSweepFinding(t *testing.T) {
	repo := buildGateRepo(t, "accepted-pending-build")
	spec := mustResolveBuildSpec(t, repo.Dir)
	writeGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)

	// Baseline: gate passes with no sweep report at all.
	var baseStdout, baseStderr bytes.Buffer
	baseGot := runGate(context.Background(), repo.Dir, spec, repo.Head, "main", nil, &baseStdout, &baseStderr)
	if baseGot != 0 {
		t.Fatalf("baseline runGate = %d, want 0; stdout=%s stderr=%s", baseGot, baseStdout.String(), baseStderr.String())
	}

	// Now write a diagram-sweep report carrying an UNDISPOSITIONED finding
	// into the corpus — the gate's outcome must not move at all.
	dir := filepath.Join(repo.Dir, ".verdi", "diagrams")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	sweepReport := "---\nschema: verdi.diagramsweep/v1\ncovers: " + repo.Head + "\nfindings:\n" +
		"  - { id: judged-1, kind: judged, text: \"undispositioned finding, present on purpose\" }\n" +
		"---\n# Diagram sweep report\n"
	if err := os.WriteFile(filepath.Join(dir, "loansvc-future.sweep-report.md"), []byte(sweepReport), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	got := runGate(context.Background(), repo.Dir, spec, repo.Head, "main", nil, &stdout, &stderr)
	if got != baseGot {
		t.Fatalf("runGate with an undispositioned diagram-sweep finding present = %d, want unchanged from baseline %d; stdout=%s stderr=%s", got, baseGot, stdout.String(), stderr.String())
	}
	if got != 0 {
		t.Fatalf("runGate = %d, want 0 (an undispositioned diagram-sweep finding must never block the merge gate)", got)
	}
}
