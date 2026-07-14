package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forbiddenDiagramSweepReferences is spec/judged-sweep ac-1/co-1's own
// wording: verdi gate's source (and internal/lint's) must contain NO
// reference to sweep-report.md, DiagramSweepFrontmatter, or
// DecodeDiagramSweep anywhere — an absence demonstrated, not merely
// asserted (obligation/judged-sweep--ac-1--static).
var forbiddenDiagramSweepReferences = []string{
	"sweep-report.md",
	"DiagramSweepFrontmatter",
	"DecodeDiagramSweep",
}

// TestDiagramSweepStatic_GateSourceNeverReferencesTheSweepReport is
// spec/judged-sweep ac-1's STATIC obligation, gate half: cmd/verdi/gate.go
// (runGate) names none of the diagram-sweep report's identifying strings.
// Mirrors internal/workbench/badgesstatic_test.go's own
// os.ReadFile+strings.Contains source-inspection pattern (feature/
// badge-computes, the one precedent this codebase has for a static claim
// with no compiler-enforced witness).
func TestDiagramSweepStatic_GateSourceNeverReferencesTheSweepReport(t *testing.T) {
	data, err := os.ReadFile("gate.go")
	if err != nil {
		t.Fatalf("reading gate.go: %v", err)
	}
	src := string(data)
	for _, forbidden := range forbiddenDiagramSweepReferences {
		if strings.Contains(src, forbidden) {
			t.Errorf("gate.go references %q — the diagram sweep must never enter verdi gate's deterministic path (spec/judged-sweep co-1)", forbidden)
		}
	}
}

// TestDiagramSweepStatic_LintSourceNeverReferencesTheSweepReport is
// spec/judged-sweep ac-1's STATIC obligation, lint half: every source file
// under internal/lint carries no reference to the sweep report either.
func TestDiagramSweepStatic_LintSourceNeverReferencesTheSweepReport(t *testing.T) {
	const lintDir = "../../internal/lint"
	entries, err := os.ReadDir(lintDir)
	if err != nil {
		t.Fatalf("reading %s: %v", lintDir, err)
	}
	checked := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		path := filepath.Join(lintDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		src := string(data)
		for _, forbidden := range forbiddenDiagramSweepReferences {
			if strings.Contains(src, forbidden) {
				t.Errorf("%s references %q — internal/lint must never reference the diagram sweep report (spec/judged-sweep co-1)", path, forbidden)
			}
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no .go files found under internal/lint — the absence check exercised nothing")
	}
}

// TestDiagramSweepStatic_AlignDispatchesANewMode is spec/judged-sweep ac-1's
// STATIC obligation, dispatch half: align.go declares the --diagram-sweep
// flag and routes it to runDiagramSweepAlign — a code path distinct from
// both the existing build-branch (runAlign) and design-branch
// (runDesignAlign) modes.
func TestDiagramSweepStatic_AlignDispatchesANewMode(t *testing.T) {
	data, err := os.ReadFile("align.go")
	if err != nil {
		t.Fatalf("reading align.go: %v", err)
	}
	src := string(data)
	if !strings.Contains(src, `"--diagram-sweep"`) {
		t.Error("align.go does not declare a --diagram-sweep flag")
	}
	if !strings.Contains(src, "runDiagramSweepAlign(") {
		t.Error("align.go does not dispatch to runDiagramSweepAlign")
	}
	if !strings.Contains(src, "runAlign(") {
		t.Error("align.go no longer dispatches to the existing build-branch runAlign — the new mode must be additive, not a replacement")
	}
	if !strings.Contains(src, "runDesignAlign(") {
		t.Error("align.go no longer dispatches to the existing design-branch runDesignAlign — the new mode must be additive, not a replacement")
	}
}
