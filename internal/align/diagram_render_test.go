package align

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/diagramverify"
)

// TestRenderDiagramAlignment_MixedFixture_GoldenText is obligation
// ac-3--behavioral's golden-text test: a fixture with one full-coverage
// realized proposal, one divergent proposal (named witness), one
// partial-coverage realized proposal, and one illustrative diagram — the
// rendered "### Diagram alignment" subsection's exact text, byte for byte,
// with the full- and partial-coverage realized lines rendering
// distinguishably rather than identically.
func TestRenderDiagramAlignment_MixedFixture_GoldenText(t *testing.T) {
	proposals := []DiagramAlignmentEntry{
		{Name: "loan-flow-clean", Coverage: diagramverify.CoverageFull, Divergent: false},
		{Name: "loan-flow-target", Coverage: diagramverify.CoverageFull, Divergent: true, Deltas: []string{
			`node "LegacyStep": contradicted — truth no longer has it (candidate witness deadbeefcafebabe)`,
		}},
		{Name: "loan-flow-unbuilt", Coverage: diagramverify.CoverageFull, Divergent: true, Deltas: []string{
			`node "NewThing": unrealized — proposed-new, not in truth`,
		}},
		{Name: "loan-flow-partial", Coverage: diagramverify.CoveragePartial, ExcludedCount: 2, Divergent: false},
	}
	illustrative := []IllustrativeFigure{{Name: "figure 1"}}

	var b strings.Builder
	renderDiagramAlignment(&b, proposals, illustrative)
	got := b.String()

	want := "" +
		"- loan-flow-clean: realized (full coverage)\n" +
		`- loan-flow-target: divergent (full coverage): node "LegacyStep": contradicted — truth no longer has it (candidate witness deadbeefcafebabe)` + "\n" +
		`- loan-flow-unbuilt: divergent (full coverage): node "NewThing": unrealized — proposed-new, not in truth` + "\n" +
		"- loan-flow-partial: realized (partial coverage — 2 elements excluded from comparison)\n" +
		"- figure 1: unverifiable (illustrative — no truth generator)\n"

	if got != want {
		t.Fatalf("renderDiagramAlignment mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}

	// The full- and partial-coverage realized lines must never read
	// identically (dc-3's own three-valued-coverage-disclosure claim).
	cleanLine := "- loan-flow-clean: realized (full coverage)\n"
	partialLine := "- loan-flow-partial: realized (partial coverage — 2 elements excluded from comparison)\n"
	if cleanLine == partialLine {
		t.Fatal("full-coverage and partial-coverage realized lines render identically")
	}
}

// TestRenderDiagramAlignment_EmptySets_ExplicitPlaceholders proves the
// subsection still renders explicit-empty placeholder lines — never an
// omitted heading or a blank body — when both the accepted-proposal and
// illustrative-diagram sets are empty.
func TestRenderDiagramAlignment_EmptySets_ExplicitPlaceholders(t *testing.T) {
	var b strings.Builder
	renderDiagramAlignment(&b, nil, nil)
	got := b.String()

	want := "- (no accepted proposals)\n" +
		"- (no illustrative diagrams in this spec's body)\n"
	if got != want {
		t.Fatalf("renderDiagramAlignment(empty) = %q, want %q", got, want)
	}
}

// TestRenderBody_DiagramAlignmentSubsection_NeverOmitted proves RenderBody
// itself always emits the "### Diagram alignment" heading under
// "## Computed", unconditionally — never behind a len(...) > 0 guard that
// would make the whole subsection vanish rather than read empty.
func TestRenderBody_DiagramAlignmentSubsection_NeverOmitted(t *testing.T) {
	body := RenderBody(nil, nil, nil, nil, nil, nil)
	for _, want := range []string{
		"## Computed",
		"### Diagram alignment",
		"(no accepted proposals)",
		"(no illustrative diagrams in this spec's body)",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("RenderBody(empty) missing %q; got:\n%s", want, body)
		}
	}
}
