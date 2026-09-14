package specimport

import (
	"errors"
	"os"
	"testing"
)

func readMarkdownFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/markdown/" + name)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return data
}

func fieldByTarget(fields []Field, target string) (Field, bool) {
	for _, f := range fields {
		if f.Target == target {
			return f, true
		}
	}
	return Field{}, false
}

func findingByCode(findings []Finding, code string) (Finding, bool) {
	for _, f := range findings {
		if f.Code == code {
			return f, true
		}
	}
	return Finding{}, false
}

func TestNormalize_MarkdownExtractsMultilineProblemAndOutcomeAndCriteria(t *testing.T) {
	data := readMarkdownFixture(t, "positive-basic.md")
	req := minimalRequest()
	req.Sources[0].Data = data

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}

	problem, ok := fieldByTarget(plan.Fields, "problem")
	if !ok {
		t.Fatalf("no problem field: %+v", plan.Fields)
	}
	wantProblem := "Operators currently retype every requirement by hand.\nThis wastes their afternoon."
	if problem.Text != wantProblem {
		t.Fatalf("problem.Text = %q, want %q", problem.Text, wantProblem)
	}
	if problem.Origin != OriginCopiedSource {
		t.Fatalf("problem.Origin = %q, want %q", problem.Origin, OriginCopiedSource)
	}
	if len(problem.Spans) != 1 || problem.Spans[0].SourceID != "source" || problem.Spans[0].Start != 29 || problem.Spans[0].End != 111 {
		t.Fatalf("problem.Spans = %+v, want one span [29,111) on source \"source\"", problem.Spans)
	}

	outcome, ok := fieldByTarget(plan.Fields, "outcome")
	if !ok {
		t.Fatalf("no outcome field: %+v", plan.Fields)
	}
	wantOutcome := "Operators import existing specs directly into a board."
	if outcome.Text != wantOutcome {
		t.Fatalf("outcome.Text = %q, want %q", outcome.Text, wantOutcome)
	}
	if len(outcome.Spans) != 1 || outcome.Spans[0].Start != 125 || outcome.Spans[0].End != 179 {
		t.Fatalf("outcome.Spans = %+v, want one span [125,179)", outcome.Spans)
	}

	wantCriteria := []struct {
		target string
		text   string
		start  int
		end    int
	}{
		{"ac-1", "The importer reads a Markdown file.", 207, 242},
		{"ac-2", "The importer preserves exact wording.", 245, 282},
		{"ac-3", "The importer reports missing fields.", 285, 321},
	}
	for _, want := range wantCriteria {
		got, ok := fieldByTarget(plan.Fields, want.target)
		if !ok {
			t.Fatalf("no field for target %s: %+v", want.target, plan.Fields)
		}
		if got.Text != want.text {
			t.Errorf("%s.Text = %q, want %q", want.target, got.Text, want.text)
		}
		if len(got.Spans) != 1 || got.Spans[0].Start != want.start || got.Spans[0].End != want.end {
			t.Errorf("%s.Spans = %+v, want one span [%d,%d)", want.target, got.Spans, want.start, want.end)
		}
		if got.Spans[0].Transform != TransformListItem {
			t.Errorf("%s.Spans[0].Transform = %q, want %q", want.target, got.Spans[0].Transform, TransformListItem)
		}
	}

	// Every acceptance criterion lacks an explicit evidence declaration: one
	// missing-evidence finding per AC (spec-import-contract.md, "The
	// preview has a missing-evidence finding until an explicit Mapping
	// supplies kinds").
	for _, want := range wantCriteria {
		if _, ok := findingForTarget(plan.Findings, FindingMissingEvidence, want.target); !ok {
			t.Errorf("no missing-evidence finding for %s: %+v", want.target, plan.Findings)
		}
	}
}

func findingForTarget(findings []Finding, code, target string) (Finding, bool) {
	for _, f := range findings {
		if f.Code == code && f.Target == target {
			return f, true
		}
	}
	return Finding{}, false
}

func TestNormalize_MarkdownFrontmatterAndSetextPositive(t *testing.T) {
	data := readMarkdownFixture(t, "positive-frontmatter-setext.md")
	req := minimalRequest()
	req.Sources[0].Data = data

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}

	problem, ok := fieldByTarget(plan.Fields, "problem")
	if !ok {
		t.Fatalf("no problem field: %+v", plan.Fields)
	}
	wantProblem := "Operators currently retype every requirement by hand.\nThis wastes their afternoon."
	if problem.Text != wantProblem {
		t.Fatalf("problem.Text = %q, want %q", problem.Text, wantProblem)
	}
	// The span must be an offset into the ORIGINAL selected bytes (the
	// frontmatter block still occupies bytes [0,38) at the front), never
	// rebased to as if the frontmatter had been deleted first.
	if len(problem.Spans) != 1 || problem.Spans[0].Start != 146 || problem.Spans[0].End != 228 {
		t.Fatalf("problem.Spans = %+v, want one span [146,228) (absolute offset past the frontmatter)", problem.Spans)
	}

	outcome, ok := fieldByTarget(plan.Fields, "outcome")
	if !ok {
		t.Fatalf("no outcome field: %+v", plan.Fields)
	}
	if len(outcome.Spans) != 1 || outcome.Spans[0].Start != 247 || outcome.Spans[0].End != 301 {
		t.Fatalf("outcome.Spans = %+v, want one span [247,301)", outcome.Spans)
	}
}

func TestNormalize_MarkdownCRLFPreserved(t *testing.T) {
	data := readMarkdownFixture(t, "crlf-basic.md")
	req := minimalRequest()
	req.Sources[0].Data = data

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}

	problem, ok := fieldByTarget(plan.Fields, "problem")
	if !ok {
		t.Fatalf("no problem field: %+v", plan.Fields)
	}
	wantProblem := "Operators currently retype every requirement by hand.\r\nThis wastes their afternoon."
	if problem.Text != wantProblem {
		t.Fatalf("problem.Text = %q, want CRLF-preserved %q", problem.Text, wantProblem)
	}
	if len(problem.Spans) != 1 || problem.Spans[0].Start != 33 || problem.Spans[0].End != 116 {
		t.Fatalf("problem.Spans = %+v, want one span [33,116)", problem.Spans)
	}
}

func TestNormalize_MarkdownMissingLabelsReportBothStatements(t *testing.T) {
	data := readMarkdownFixture(t, "missing-label.md")
	req := minimalRequest()
	req.Sources[0].Data = data

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	if _, ok := fieldByTarget(plan.Fields, "problem"); ok {
		t.Fatalf("problem field present despite no Problem heading: %+v", plan.Fields)
	}
	if _, ok := fieldByTarget(plan.Fields, "outcome"); ok {
		t.Fatalf("outcome field present despite no Outcome heading: %+v", plan.Fields)
	}
	for _, target := range []string{"problem", "outcome"} {
		f, ok := findingForTarget(plan.Findings, FindingMissingStatement, target)
		if !ok {
			t.Fatalf("no missing-statement finding for %s: %+v", target, plan.Findings)
		}
		if !f.Blocking {
			t.Errorf("missing-statement finding for %s is not blocking: %+v", target, f)
		}
	}
}

func TestNormalize_MarkdownEmptyLabelReportsEmptyField(t *testing.T) {
	data := readMarkdownFixture(t, "empty-label.md")
	req := minimalRequest()
	req.Sources[0].Data = data

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	if _, ok := fieldByTarget(plan.Fields, "problem"); ok {
		t.Fatalf("problem field present despite empty body: %+v", plan.Fields)
	}
	if _, ok := findingForTarget(plan.Findings, FindingEmptyField, "problem"); !ok {
		t.Fatalf("no empty-field finding for problem: %+v", plan.Findings)
	}
	// Outcome is a normal, populated field, unaffected by the sibling
	// section's structural defect.
	if _, ok := fieldByTarget(plan.Fields, "outcome"); !ok {
		t.Fatalf("outcome field missing even though it is well-formed: %+v", plan.Fields)
	}
}

func TestNormalize_MarkdownDuplicateLabelReportsAmbiguousField(t *testing.T) {
	data := readMarkdownFixture(t, "duplicate-label.md")
	req := minimalRequest()
	req.Sources[0].Data = data

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	if _, ok := fieldByTarget(plan.Fields, "problem"); ok {
		t.Fatalf("problem field present despite duplicate aliases; must not silently select the first: %+v", plan.Fields)
	}
	f, ok := findingForTarget(plan.Findings, FindingAmbiguousField, "problem")
	if !ok {
		t.Fatalf("no ambiguous-field finding for problem: %+v", plan.Findings)
	}
	if !f.Blocking {
		t.Errorf("ambiguous-field finding is not blocking: %+v", f)
	}
}

func TestNormalize_MarkdownHeadingInsideFencedBlockIsNotAField(t *testing.T) {
	data := readMarkdownFixture(t, "heading-in-fenced.md")
	req := minimalRequest()
	req.Sources[0].Data = data

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	if _, ok := findingForTarget(plan.Findings, FindingMultipleTargets, "Not a real heading"); ok {
		t.Fatalf("a '#' line inside a fenced block was treated as a heading: %+v", plan.Findings)
	}
	problem, ok := fieldByTarget(plan.Fields, "problem")
	if !ok {
		t.Fatalf("no problem field: %+v", plan.Fields)
	}
	wantProblem := "Operators currently retype every requirement by hand.\n\n```text\n# Not a real heading\n## Neither is this\n```"
	if problem.Text != wantProblem {
		t.Fatalf("problem.Text = %q, want the fenced block absorbed as body content: %q", problem.Text, wantProblem)
	}
	if len(problem.Spans) != 1 || problem.Spans[0].Start != 29 || problem.Spans[0].End != 135 {
		t.Fatalf("problem.Spans = %+v, want one span [29,135)", problem.Spans)
	}
	outcome, ok := fieldByTarget(plan.Fields, "outcome")
	if !ok || len(outcome.Spans) != 1 || outcome.Spans[0].Start != 149 || outcome.Spans[0].End != 203 {
		t.Fatalf("outcome field/spans wrong: %+v ok=%v", outcome, ok)
	}
}

// TestNormalize_MarkdownSectionBoundariesComeFromRealBlockPositions pins
// both ends of the section-extent computation against blocks that carry no
// goldmark line segment at all. A thematic break and an ATX heading with an
// empty label are exactly such blocks; deriving a boundary from a
// descendant leaf's segment silently collapses it to offset 0, which made
// the problem statement absorb the document title and its own "## Problem"
// marker and publish them as copied-source (spec-import-contract.md:
// "Problem/outcome select the entire section body excluding leading/
// trailing blank lines, preserving all interior bytes and line breaks").
func TestNormalize_MarkdownSectionBoundariesComeFromRealBlockPositions(t *testing.T) {
	t.Run("thematic break first in the section", func(t *testing.T) {
		raw := "# Title\n\n## Problem\n\n***\n\nreal problem text\n\n## Outcome\n\no\n"
		req := minimalRequest()
		req.Sources[0].Data = []byte(raw)

		plan, err := Normalize(req)
		if err != nil {
			t.Fatalf("Normalize: unexpected error: %v", err)
		}
		problem, ok := fieldByTarget(plan.Fields, "problem")
		if !ok {
			t.Fatalf("no problem field: %+v", plan.Fields)
		}
		const want = "***\n\nreal problem text"
		if problem.Text != want {
			t.Fatalf("problem.Text = %q, want exactly the section body %q (never the title or its own heading)", problem.Text, want)
		}
		if len(problem.Spans) != 1 || problem.Spans[0].Start != 21 || problem.Spans[0].End != 43 {
			t.Fatalf("problem.Spans = %+v, want one span [21,43)", problem.Spans)
		}
		if raw[problem.Spans[0].Start:problem.Spans[0].End] != want {
			t.Fatalf("the recorded span does not reproduce the reported text")
		}
	})

	t.Run("empty heading as the closing boundary", func(t *testing.T) {
		raw := "# Title\n\n## Problem\n\nreal problem text\n\n##\n\n## Outcome\n\no\n"
		req := minimalRequest()
		req.Sources[0].Data = []byte(raw)

		plan, err := Normalize(req)
		if err != nil {
			t.Fatalf("Normalize: unexpected error: %v", err)
		}
		problem, ok := fieldByTarget(plan.Fields, "problem")
		if !ok {
			t.Fatalf("no problem field (an empty heading boundary must not erase the section): %+v %+v", plan.Fields, plan.Findings)
		}
		const want = "real problem text"
		if problem.Text != want {
			t.Fatalf("problem.Text = %q, want %q", problem.Text, want)
		}
		if len(problem.Spans) != 1 || raw[problem.Spans[0].Start:problem.Spans[0].End] != want {
			t.Fatalf("problem.Spans = %+v, want a span reproducing %q", problem.Spans, want)
		}
	})
}

func TestNormalize_MarkdownMultipleTargetsAfterTitle(t *testing.T) {
	data := readMarkdownFixture(t, "multiple-targets.md")
	req := minimalRequest()
	req.Sources[0].Data = data

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	f, ok := findingByCode(plan.Findings, FindingMultipleTargets)
	if !ok {
		t.Fatalf("no multiple-targets finding: %+v", plan.Findings)
	}
	if !f.Blocking {
		t.Errorf("multiple-targets finding is not blocking: %+v", f)
	}
	// Outcome came after the second top-level heading, so it must not be
	// silently attributed to the first target.
	if _, ok := fieldByTarget(plan.Fields, "outcome"); ok {
		t.Fatalf("outcome field present despite ambiguous multiple-target structure: %+v", plan.Fields)
	}
}

func TestNormalize_MarkdownUnresolvedNestedListDoesNotProduceCriteria(t *testing.T) {
	data := readMarkdownFixture(t, "unresolved-list.md")
	req := minimalRequest()
	req.Sources[0].Data = data

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	if _, ok := fieldByTarget(plan.Fields, "ac-1"); ok {
		t.Fatalf("ac-1 field present despite a nested list breaking the flat-list grammar: %+v", plan.Fields)
	}
	if _, ok := findingByCode(plan.Findings, FindingAmbiguousField); !ok {
		t.Fatalf("no ambiguous-field finding for the nested acceptance-criteria list: %+v", plan.Findings)
	}
}

// TestNormalize_MarkdownOrderedListIsNotPromotedToObjects pins the closed
// grammar: "Object sections accept a flat Markdown bullet list" and "Strip
// only the bullet marker and its following space". An ordered list is not a
// bullet list, so that section is unresolved rather than silently promoted
// with its "1." markers consumed as if they were bullets.
func TestNormalize_MarkdownOrderedListIsNotPromotedToObjects(t *testing.T) {
	raw := "# Widget Import\n\n## Problem\n\np\n\n## Outcome\n\no\n\n## Acceptance Criteria\n\n1. First criterion.\n2. Second criterion.\n"
	req := minimalRequest()
	req.Sources[0].Data = []byte(raw)

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	for _, target := range []string{"ac-1", "ac-2"} {
		if f, ok := fieldByTarget(plan.Fields, target); ok {
			t.Errorf("%s promoted from an ordered list: %+v", target, f)
		}
	}
	f, ok := findingForTarget(plan.Findings, FindingAmbiguousField, "acceptance-criteria")
	if !ok {
		t.Fatalf("no ambiguous-field finding for the ordered acceptance-criteria list: %+v", plan.Findings)
	}
	if !f.Blocking {
		t.Errorf("ordered-list finding is not blocking: %+v", f)
	}
}

// TestNormalize_MarkdownNestedSubheadingKeepsWholeStatementSection pins that
// a deeper subheading inside a statement section is interior content, not a
// section boundary: the contract recognizes field sections at exactly one
// level below the title and requires the entire section body, "preserving
// all interior bytes and line breaks". Truncating the statement at the
// subheading dropped half of it into retained-only with no finding at all.
func TestNormalize_MarkdownNestedSubheadingKeepsWholeStatementSection(t *testing.T) {
	raw := "# Widget Import\n\n## Problem\n\np\n\n### Detail\n\nq\n\n## Outcome\n\no\n"
	req := minimalRequest()
	req.Sources[0].Data = []byte(raw)

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	problem, ok := fieldByTarget(plan.Fields, "problem")
	if !ok {
		t.Fatalf("no problem field: %+v %+v", plan.Fields, plan.Findings)
	}
	const want = "p\n\n### Detail\n\nq"
	if problem.Text != want {
		t.Fatalf("problem.Text = %q, want the whole section body %q", problem.Text, want)
	}
	if len(problem.Spans) != 1 || raw[problem.Spans[0].Start:problem.Spans[0].End] != want {
		t.Fatalf("problem.Spans = %+v, want a span reproducing the whole section body", problem.Spans)
	}
	// The interior bytes are accounted as mapped, not silently retained.
	cov := coverageForSource(t, plan, "source")
	for _, iv := range cov.Intervals {
		if iv.Start <= 33 && 33 < iv.End && iv.Disposition != DispositionMapped {
			t.Fatalf("the subheading's bytes fell into a %q interval %+v instead of the problem statement", iv.Disposition, iv)
		}
	}
	if got, want := cov.MappedBytes+cov.RetainedBytes+cov.UnresolvedBytes, cov.TotalBytes; got != want {
		t.Fatalf("mapped+retained+unresolved = %d, want TotalBytes %d", got, want)
	}
}

// TestNormalize_MarkdownSubheadingInsideObjectSectionIsUnresolved is the
// object-section half of the same boundary rule: a subheading inside an
// object section is mixed non-list content, so the section is reported
// unresolved rather than quietly truncated to the leading list.
func TestNormalize_MarkdownSubheadingInsideObjectSectionIsUnresolved(t *testing.T) {
	raw := "# Widget Import\n\n## Problem\n\np\n\n## Outcome\n\no\n\n## Acceptance Criteria\n\n- First criterion.\n\n### Notes\n\nmore prose\n"
	req := minimalRequest()
	req.Sources[0].Data = []byte(raw)

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	if f, ok := fieldByTarget(plan.Fields, "ac-1"); ok {
		t.Errorf("ac-1 extracted while the rest of its section was dropped: %+v", f)
	}
	if _, ok := findingForTarget(plan.Findings, FindingAmbiguousField, "acceptance-criteria"); !ok {
		t.Fatalf("no ambiguous-field finding for an object section with mixed content: %+v", plan.Findings)
	}
}

func TestNormalize_MarkdownNoHeadingIsUnsupportedStructure(t *testing.T) {
	data := readMarkdownFixture(t, "unsupported-structure.md")
	req := minimalRequest()
	req.Sources[0].Data = data

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	f, ok := findingByCode(plan.Findings, FindingUnsupportedStructure)
	if !ok {
		t.Fatalf("no unsupported-structure finding: %+v", plan.Findings)
	}
	if !f.Blocking {
		t.Errorf("unsupported-structure finding is not blocking: %+v", f)
	}
	if len(plan.Fields) != 0 {
		t.Fatalf("fields extracted from a headingless document: %+v", plan.Fields)
	}
}

func TestNormalize_MarkdownUnclosedFrontmatterIsInvalidSource(t *testing.T) {
	data := readMarkdownFixture(t, "unclosed-frontmatter.md")
	req := minimalRequest()
	req.Sources[0].Data = data

	if _, err := Normalize(req); !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("Normalize on an unclosed frontmatter delimiter: got err %v, want ErrInvalidSource", err)
	}
}

func TestNormalize_MarkdownSourceDeclaredIDRequiresMappingAndKeepsPositionalNumbering(t *testing.T) {
	data := readMarkdownFixture(t, "source-id-list-item.md")
	req := minimalRequest()
	req.Sources[0].Data = data

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}

	if _, ok := fieldByTarget(plan.Fields, "ac-7"); ok {
		t.Fatalf("ac-7 was silently generated from a source-declared id token: %+v", plan.Fields)
	}
	f, ok := findingForTarget(plan.Findings, FindingSourceIDRequiresMap, "ac-7")
	if !ok {
		t.Fatalf("no source-id-requires-mapping finding for ac-7: %+v", plan.Findings)
	}
	if !f.Blocking {
		t.Errorf("source-id-requires-mapping finding is not blocking: %+v", f)
	}

	ac1, ok := fieldByTarget(plan.Fields, "ac-1")
	if !ok || ac1.Text != "The importer reads a Markdown file." {
		t.Fatalf("ac-1 wrong: %+v ok=%v", ac1, ok)
	}
	if _, ok := fieldByTarget(plan.Fields, "ac-2"); ok {
		t.Fatalf("ac-2 must not exist: the blocked item's ordinal is reserved so numbering keeps source position, not a compacted count")
	}
	ac3, ok := fieldByTarget(plan.Fields, "ac-3")
	if !ok || ac3.Text != "The importer reports missing fields." {
		t.Fatalf("ac-3 wrong: %+v ok=%v (third source item keeps its source-position id even though the second item was blocked)", ac3, ok)
	}
}

func TestNormalize_MarkdownSourceDeclaredIDResolvedByExplicitMapping(t *testing.T) {
	data := readMarkdownFixture(t, "source-id-list-item.md")
	req := minimalRequest()
	req.Sources[0].Data = data
	req.Mappings = []Mapping{
		{Target: "ac-7", SourceID: "source", Start: 216, End: 284, Transform: TransformListItem},
	}

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	if _, ok := findingForTarget(plan.Findings, FindingSourceIDRequiresMap, "ac-7"); ok {
		t.Fatalf("source-id-requires-mapping finding still present after an explicit resolving mapping: %+v", plan.Findings)
	}
	ac7, ok := fieldByTarget(plan.Fields, "ac-7")
	if !ok {
		t.Fatalf("ac-7 field missing after explicit resolution: %+v", plan.Fields)
	}
	if ac7.Text != "ac-7: The importer preserves an existing source-declared identifier." {
		t.Fatalf("ac-7.Text = %q", ac7.Text)
	}
	// Neighbouring ids are unaffected by the resolution.
	if ac1, ok := fieldByTarget(plan.Fields, "ac-1"); !ok || ac1.Text != "The importer reads a Markdown file." {
		t.Fatalf("ac-1 disturbed by resolving ac-7: %+v ok=%v", ac1, ok)
	}
	if ac3, ok := fieldByTarget(plan.Fields, "ac-3"); !ok || ac3.Text != "The importer reports missing fields." {
		t.Fatalf("ac-3 disturbed by resolving ac-7: %+v ok=%v", ac3, ok)
	}
}
