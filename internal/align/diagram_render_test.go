package align

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/diagramverify"
	"github.com/jyang234/verdi/internal/disclosure"
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

// TestDiagramSweepDisclosure pins the disclosure VALUE the sweep report's
// fixed advisory line constructs — its source, its (absent) scope, its
// derived id and its fixed severity — mirroring internal/disclosure's own
// constructor tests. The sweep names no single artifact (the disclosure is
// about the whole sweep, not one finding), so the scope is empty by
// construction and the id is the bare source.
func TestDiagramSweepDisclosure(t *testing.T) {
	d := DiagramSweepDisclosure()
	if d.Source != DiagramSweepDisclosureSource {
		t.Errorf("Source = %q, want %q", d.Source, DiagramSweepDisclosureSource)
	}
	if d.Scope != "" {
		t.Errorf("Scope = %q, want empty (the disclosure is about the sweep, not one finding)", d.Scope)
	}
	if d.ID != DiagramSweepDisclosureSource {
		t.Errorf("ID = %q, want the bare source %q", d.ID, DiagramSweepDisclosureSource)
	}
	if d.Severity != disclosure.SeverityDisclosedUnproven {
		t.Errorf("Severity = %q, want %q", d.Severity, disclosure.SeverityDisclosedUnproven)
	}
	// spec/judged-sweep dc-5 requires the report to state the sweep is
	// advisory and non-exhaustive VERBATIM; the seam migration wraps that
	// sentence, it never rewords it.
	if !strings.Contains(d.Text, "advisory and non-exhaustive") {
		t.Errorf("Text = %q, want dc-5's verbatim advisory/non-exhaustive wording", d.Text)
	}
	// Negative path — the defect this replaced: the line was a bare const, so
	// nothing declared its severity at all and IsRendered rejected it. The
	// text must still never state the severity itself; Render supplies it.
	if strings.Contains(d.Text, disclosure.SeverityDisclosedUnproven) {
		t.Errorf("Text = %q states the severity itself; Render already supplies it", d.Text)
	}
	if !disclosure.IsRendered(DiagramSweepDisclosureLine()) {
		t.Errorf("DiagramSweepDisclosureLine() = %q is not recognized by disclosure.IsRendered: no disclosure consumer could count it", DiagramSweepDisclosureLine())
	}
	if DiagramSweepDisclosureLine() != disclosure.Render(d) {
		t.Errorf("DiagramSweepDisclosureLine() = %q, want the seam's rendering %q", DiagramSweepDisclosureLine(), disclosure.Render(d))
	}
}

// TestRenderDiagramSweepBody_EmitsTheSeamRenderedDisclosure exercises the
// producing call site: the body carries the disclosure as its own bare,
// flush-left line — unindented and unprefixed, so disclosure.IsRendered
// recognizes it — before any finding, in a report with findings and in a
// clean one alike (spec/judged-sweep ac-4/dc-5).
func TestRenderDiagramSweepBody_EmitsTheSeamRenderedDisclosure(t *testing.T) {
	want := disclosure.Render(DiagramSweepDisclosure())

	for _, tc := range []struct {
		name     string
		findings []artifact.ConflictFinding
	}{
		{name: "clean sweep"},
		{name: "with a finding", findings: []artifact.ConflictFinding{{ID: "judged-x", Kind: artifact.FindingJudged, Text: "t"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := RenderDiagramSweepBody("diagram/example-flow", tc.findings)
			lines := strings.Split(body, "\n")
			idx := -1
			for i, l := range lines {
				if l == want {
					idx = i
				}
			}
			if idx < 0 {
				t.Fatalf("body does not carry the seam-rendered disclosure %q:\n%s", want, body)
			}
			// Before any finding (dc-5): the "## Findings" heading follows it.
			for i, l := range lines {
				if strings.HasPrefix(l, "- **") && i < idx {
					t.Fatalf("a finding line precedes the disclosure at line %d:\n%s", idx, body)
				}
			}
		})
	}
}
