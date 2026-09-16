package specimport

import (
	"errors"
	"os"
	"strings"
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

// objectSectionFixture wraps one object section's list in the smallest
// complete markdown-v1 document, so one empty item's disclosure can be
// observed through the public Normalize gate for any of the four object
// kinds rather than only for acceptance criteria.
func objectSectionFixture(heading, list string) string {
	return "# T\n\n## Problem\n\np\n\n## Outcome\n\no\n\n## " + heading + "\n\n" + list
}

// TestNormalize_MarkdownEmptyListItemReportsItsOrdinalObject pins the object
// grammar over an item the source left blank: "one direct item = one object",
// "empty fields... are reported", and RetainUnmapped disposes of BYTES, so it
// can never make an empty OBJECT valid.
//
// An empty item used to consume its ordinal in silence: a complete labeled
// source whose acceptance-criteria list began with a bare "-" normalized to
// findings=[] under retain_unmapped, so the object that item declares existed
// in the numbering and nowhere else — no field, no finding, nothing for a
// reviewer to act on. The disclosure names that would-be ordinal target and
// invents no Field, text or span for it.
func TestNormalize_MarkdownEmptyListItemReportsItsOrdinalObject(t *testing.T) {
	t.Run("every object kind reports the empty item's own ordinal target", func(t *testing.T) {
		cases := []struct{ heading, prefix, section string }{
			{"Acceptance Criteria", "ac", "acceptance-criteria"},
			{"Constraints", "co", "constraints"},
			{"Decisions", "dc", "decisions"},
			{"Open Questions", "oq", "open-questions"},
		}
		for _, c := range cases {
			t.Run(c.heading, func(t *testing.T) {
				req := minimalRequest()
				req.Sources[0].Data = []byte(objectSectionFixture(c.heading, "-\n- done\n"))

				plan, err := Normalize(req)
				if err != nil {
					t.Fatalf("Normalize: unexpected error: %v", err)
				}
				empty := c.prefix + "-1"
				if f, ok := fieldByTarget(plan.Fields, empty); ok {
					t.Fatalf("%s produced a field %+v; an empty item has no text and no span to publish", empty, f)
				}
				f, ok := findingForTarget(plan.Findings, FindingEmptyField, empty)
				if !ok {
					t.Fatalf("no empty-field finding for the empty item's ordinal %s: %+v", empty, plan.Findings)
				}
				if !f.Blocking {
					t.Errorf("empty-field finding for %s is not blocking: %+v", empty, f)
				}
				if !strings.Contains(f.Message, c.section) {
					t.Errorf("empty-field message %q does not name the %s section the item came from", f.Message, c.section)
				}
				// The empty item still consumes ordinal 1: the item after it
				// keeps its source-position number.
				if got, ok := fieldByTarget(plan.Fields, c.prefix+"-2"); !ok || got.Text != "done" {
					t.Fatalf("%s-2 = %+v ok=%v, want the following item still numbered from its source position", c.prefix, got, ok)
				}
			})
		}
	})

	t.Run("an interior empty item keeps ordinal continuity and source coverage", func(t *testing.T) {
		raw := []byte(bulletFixture("- alpha\n-\n- gamma\n"))
		req := minimalRequest()
		req.Sources[0].Data = raw

		plan, err := Normalize(req)
		if err != nil {
			t.Fatalf("Normalize: unexpected error: %v", err)
		}
		f, ok := findingForTarget(plan.Findings, FindingEmptyField, "ac-2")
		if !ok || !f.Blocking {
			t.Fatalf("empty-field for the interior empty item = %+v ok=%v, want it blocking on ac-2: %+v", f, ok, plan.Findings)
		}
		if got, ok := fieldByTarget(plan.Fields, "ac-2"); ok {
			t.Fatalf("ac-2 = %+v; the reported ordinal must carry no fabricated field", got)
		}
		ac1, ok := fieldByTarget(plan.Fields, "ac-1")
		if !ok || ac1.Text != "alpha" {
			t.Fatalf("ac-1 = %+v ok=%v", ac1, ok)
		}
		ac3, ok := fieldByTarget(plan.Fields, "ac-3")
		if !ok || ac3.Text != "gamma" {
			t.Fatalf("ac-3 = %+v ok=%v, want the empty item's ordinal still consumed", ac3, ok)
		}
		// Reporting the gap changes no byte's disposition: both real items
		// stay mapped and every selected byte still has exactly one.
		cov := coverageForSource(t, plan, "source")
		mappedIntervalFor(t, cov, ac1.Spans[0].Start, ac1.Spans[0].End, "ac-1")
		mappedIntervalFor(t, cov, ac3.Spans[0].Start, ac3.Spans[0].End, "ac-3")
		if got, want := cov.MappedBytes+cov.RetainedBytes+cov.UnresolvedBytes, cov.TotalBytes; got != want {
			t.Fatalf("mapped+retained+unresolved = %d, want TotalBytes %d", got, want)
		}
	})

	t.Run("retain_unmapped does not make the empty object valid", func(t *testing.T) {
		// The owner's reproduction: a complete labeled source, an attested
		// second criterion, and every unmapped byte explicitly retained.
		raw := []byte(bulletFixture("-\n- done\n"))
		req := minimalRequest()
		req.Sources[0].Data = raw
		req.RetainUnmapped = true
		req.Mappings = []Mapping{{Target: "ac-2", Evidence: []string{"attestation"}}}

		plan, err := Normalize(req)
		if err != nil {
			t.Fatalf("Normalize: unexpected error: %v", err)
		}
		if len(plan.Findings) == 0 {
			t.Fatalf("a source declaring an empty object normalized with no findings at all: fields %+v", plan.Fields)
		}
		f, ok := findingForTarget(plan.Findings, FindingEmptyField, "ac-1")
		if !ok || !f.Blocking {
			t.Fatalf("empty-field for ac-1 = %+v ok=%v, want retaining the bytes to leave the empty object blocking: %+v", f, ok, plan.Findings)
		}
		ac2, ok := fieldByTarget(plan.Fields, "ac-2")
		if !ok || ac2.Text != "done" || len(ac2.Evidence) != 1 {
			t.Fatalf("ac-2 = %+v ok=%v, want the attested second criterion untouched", ac2, ok)
		}
		cov := coverageForSource(t, plan, "source")
		if got, want := cov.MappedBytes+cov.RetainedBytes+cov.UnresolvedBytes, cov.TotalBytes; got != want {
			t.Fatalf("mapped+retained+unresolved = %d, want TotalBytes %d", got, want)
		}
	})

	t.Run("an explicit nonblank mapping supplies the value and keeps the disclosure", func(t *testing.T) {
		raw := []byte(bulletFixture("-\n- done\n"))
		corrected := "The criterion the source left blank."
		req := minimalRequest()
		req.Sources[0].Data = raw
		req.Mappings = []Mapping{{Target: "ac-1", Text: &corrected, Evidence: []string{"behavioral"}}}

		plan, err := Normalize(req)
		if err != nil {
			t.Fatalf("Normalize: unexpected error: %v", err)
		}
		ac1, ok := fieldByTarget(plan.Fields, "ac-1")
		if !ok || ac1.Text != corrected || ac1.Origin != OriginUserAdded {
			t.Fatalf("ac-1 = %+v ok=%v, want the user's own value for the ordinal the source left empty", ac1, ok)
		}
		f, ok := findingForTarget(plan.Findings, FindingEmptyField, "ac-1")
		if !ok {
			t.Fatalf("the source-empty disclosure vanished once a mapping supplied the value: %+v", plan.Findings)
		}
		if f.Blocking {
			t.Errorf("empty-field for ac-1 still blocking after an explicit nonblank mapping: %+v", f)
		}
		if _, ok := findingForTarget(plan.Findings, FindingMissingEvidence, "ac-1"); ok {
			t.Errorf("missing-evidence for ac-1 despite the mapping declaring a kind: %+v", plan.Findings)
		}
	})

	t.Run("a correction still needs its own evidence", func(t *testing.T) {
		raw := []byte(bulletFixture("-\n- done\n"))
		corrected := "The criterion the source left blank."
		req := minimalRequest()
		req.Sources[0].Data = raw
		req.Mappings = []Mapping{{Target: "ac-1", Text: &corrected}}

		plan, err := Normalize(req)
		if err != nil {
			t.Fatalf("Normalize: unexpected error: %v", err)
		}
		f, ok := findingForTarget(plan.Findings, FindingMissingEvidence, "ac-1")
		if !ok || !f.Blocking {
			t.Fatalf("missing-evidence for the corrected ac-1 = %+v ok=%v, want supplying a value not to waive evidence: %+v", f, ok, plan.Findings)
		}
	})

	t.Run("a syntax-bearing empty block is not an empty item", func(t *testing.T) {
		cases := []struct{ name, list, wantText string }{
			{"empty fenced block", "- ```\n  ```\n- plain\n", "```\n```"},
			{"empty blockquote", "- >\n- plain\n", ">"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				req := minimalRequest()
				req.Sources[0].Data = []byte(bulletFixture(c.list))

				plan, err := Normalize(req)
				if err != nil {
					t.Fatalf("Normalize: unexpected error: %v", err)
				}
				if ac1, ok := fieldByTarget(plan.Fields, "ac-1"); !ok || ac1.Text != c.wantText {
					t.Fatalf("ac-1 = %+v ok=%v, want the block's own syntax kept as content", ac1, ok)
				}
				if f, ok := findingByCode(plan.Findings, FindingEmptyField); ok {
					t.Fatalf("empty-field %+v for an item whose body is real syntax", f)
				}
			})
		}
	})

	t.Run("a nested list is still an unresolved section, not an empty object", func(t *testing.T) {
		req := minimalRequest()
		req.Sources[0].Data = []byte(bulletFixture("- outer item\n\n  - inner item\n- plain\n"))

		plan, err := Normalize(req)
		if err != nil {
			t.Fatalf("Normalize: unexpected error: %v", err)
		}
		if _, ok := findingForTarget(plan.Findings, FindingAmbiguousField, "acceptance-criteria"); !ok {
			t.Fatalf("no ambiguous-field finding for the nested list: %+v", plan.Findings)
		}
		if f, ok := findingByCode(plan.Findings, FindingEmptyField); ok {
			t.Fatalf("empty-field %+v for an unresolved section; a refused section reports no per-ordinal object", f)
		}
	})
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

// TestNormalize_SourceDeclaredIDResolutionRequiresTheCorrespondingItem pins
// the closed contract review's residual 4: "An explicit mapping to the
// source-declared ID OVER THAT ITEM resolves the blocker". Keying resolution
// on Mapping.Target alone let a spanless user-added mapping publish invented
// text under the source's own declared identifier while the source's real
// ac-7 item was disposed of as ordinary retained-only.
func TestNormalize_SourceDeclaredIDResolutionRequiresTheCorrespondingItem(t *testing.T) {
	data := readMarkdownFixture(t, "source-id-list-item.md")
	itemStart, itemEnd := spanOf(t, data, "ac-7: The importer preserves an existing source-declared identifier.")
	otherStart, otherEnd := spanOf(t, data, "The importer reads a Markdown file.")

	notes := []byte("- ac-7: A different document's own ac-7 item.\n")
	notesStart, notesEnd := spanOf(t, notes, "ac-7: A different document's own ac-7 item.")

	invented := "Something the source never said"
	cases := []struct {
		name    string
		mapping Mapping
	}{
		{"user-added with no span at all", Mapping{Target: "ac-7", Text: &invented}},
		{"the whole source", Mapping{Target: "ac-7", SourceID: "source", Start: 0, End: len(data), Transform: TransformIdentity}},
		{"a different item", Mapping{Target: "ac-7", SourceID: "source", Start: otherStart, End: otherEnd, Transform: TransformListItem}},
		{"a partial selection of the item", Mapping{Target: "ac-7", SourceID: "source", Start: itemStart, End: itemEnd - 5, Transform: TransformIdentity}},
		{"the same id in another source", Mapping{Target: "ac-7", SourceID: "notes", Start: notesStart, End: notesEnd, Transform: TransformListItem}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := minimalRequest()
			req.Sources[0].Data = data
			req.Sources = append(req.Sources, Source{ID: "notes", Label: "notes.md", Data: notes})
			req.Mappings = []Mapping{c.mapping}

			plan, err := Normalize(req)
			if err != nil {
				t.Fatalf("Normalize: unexpected error: %v", err)
			}
			f, ok := findingForTarget(plan.Findings, FindingSourceIDRequiresMap, "ac-7")
			if !ok {
				t.Fatalf("source-id-requires-mapping for ac-7 cleared by %s: %+v", c.name, plan.Findings)
			}
			if !f.Blocking {
				t.Errorf("source-id-requires-mapping for ac-7 is no longer blocking after %s: %+v", c.name, f)
			}
		})
	}

	t.Run("the item including its bullet marker resolves it", func(t *testing.T) {
		req := minimalRequest()
		req.Sources[0].Data = data
		req.Mappings = []Mapping{
			{Target: "ac-7", SourceID: "source", Start: itemStart - 2, End: itemEnd, Transform: TransformListItem},
		}
		plan, err := Normalize(req)
		if err != nil {
			t.Fatalf("Normalize: unexpected error: %v", err)
		}
		if _, ok := findingForTarget(plan.Findings, FindingSourceIDRequiresMap, "ac-7"); ok {
			t.Fatalf("a mapping over the blocked item itself did not resolve the blocker: %+v", plan.Findings)
		}
		if ac3, ok := fieldByTarget(plan.Fields, "ac-3"); !ok || ac3.Text != "The importer reports missing fields." {
			t.Fatalf("ordinal continuity disturbed: ac-3 = %+v ok=%v", ac3, ok)
		}
	})
}

// duplicateDeclaredIDSource declares ac-7 on TWO list items, the shape that
// makes id-keyed resolution unsound.
const duplicateDeclaredIDSource = "# T\n\n## Problem\n\np\n\n## Outcome\n\no\n\n" +
	"## Acceptance Criteria\n\n" +
	"- ac-7: first declared item\n" +
	"- ac-7: second declared item\n" +
	"- plain third\n"

func sourceIDFindings(findings []Finding, target string) []Finding {
	var out []Finding
	for _, f := range findings {
		if f.Code == FindingSourceIDRequiresMap && f.Target == target {
			out = append(out, f)
		}
	}
	return out
}

// TestNormalize_SourceDeclaredIDResolutionIsPerOccurrence pins that a
// mapping resolves the OCCURRENCE it selects, not every item that happens to
// declare the same id. Resolution used to be recorded in a map keyed by id
// and then dropped every finding carrying that Target, so mapping the first
// ac-7 item silently cleared the second item's blocker: that second declared
// identity disappeared from the preview with no field and no disclosure,
// while validateMappings refuses a second mapping for the same target, so
// the user had no way to resolve it on its own terms either.
func TestNormalize_SourceDeclaredIDResolutionIsPerOccurrence(t *testing.T) {
	data := []byte(duplicateDeclaredIDSource)
	firstStart, firstEnd := spanOf(t, data, "ac-7: first declared item")
	secondStart, secondEnd := spanOf(t, data, "ac-7: second declared item")

	t.Run("both declarations block before any mapping", func(t *testing.T) {
		req := minimalRequest()
		req.Sources[0].Data = data
		plan, err := Normalize(req)
		if err != nil {
			t.Fatalf("Normalize: unexpected error: %v", err)
		}
		got := sourceIDFindings(plan.Findings, "ac-7")
		if len(got) != 2 {
			t.Fatalf("source-id findings for ac-7 = %d, want one per declaring item: %+v", len(got), plan.Findings)
		}
		for _, f := range got {
			if !f.Blocking {
				t.Errorf("source-id finding is not blocking: %+v", f)
			}
		}
	})

	selected := []struct {
		name       string
		start, end int
		wantText   string
		keptStart  int
		keptEnd    int
	}{
		{"the first item", firstStart, firstEnd, "ac-7: first declared item", secondStart, secondEnd},
		{"the second item", secondStart, secondEnd, "ac-7: second declared item", firstStart, firstEnd},
	}
	for _, c := range selected {
		t.Run("mapping "+c.name+" leaves the other declaration blocking", func(t *testing.T) {
			req := minimalRequest()
			req.Sources[0].Data = data
			req.Mappings = []Mapping{
				{Target: "ac-7", SourceID: "source", Start: c.start, End: c.end, Transform: TransformListItem},
			}
			plan, err := Normalize(req)
			if err != nil {
				t.Fatalf("Normalize: unexpected error: %v", err)
			}
			got := sourceIDFindings(plan.Findings, "ac-7")
			if len(got) != 1 {
				t.Fatalf("source-id findings for ac-7 = %d, want exactly the unselected declaration: %+v", len(got), plan.Findings)
			}
			if !got[0].Blocking {
				t.Errorf("the unresolved declaration is no longer blocking: %+v", got[0])
			}
			// Useful corrective guidance: the conflict is unsatisfiable by a
			// second Mapping, so the finding must say what the user can do.
			if !strings.Contains(got[0].Message, "on 2 list items") || !strings.Contains(got[0].Message, "distinct id") {
				t.Errorf("no corrective guidance for the unsatisfiable duplicate: %q", got[0].Message)
			}

			ac7, ok := fieldByTarget(plan.Fields, "ac-7")
			if !ok || ac7.Text != c.wantText {
				t.Fatalf("ac-7 = %+v ok=%v, want the selected occurrence %q", ac7, ok, c.wantText)
			}
			// The unselected item keeps its bytes under the user's
			// RetainUnmapped disposition; nothing claims them as ac-7.
			cov := coverageForSource(t, plan, "source")
			for _, iv := range cov.Intervals {
				if iv.Start < c.keptEnd && c.keptStart < iv.End && iv.Disposition != DispositionRetained {
					t.Errorf("interval %+v over the unselected declaration is %q, want retained-only", iv, iv.Disposition)
				}
			}
		})
	}

	t.Run("a second mapping for the same target is still refused", func(t *testing.T) {
		req := minimalRequest()
		req.Sources[0].Data = data
		req.Mappings = []Mapping{
			{Target: "ac-7", SourceID: "source", Start: firstStart, End: firstEnd, Transform: TransformListItem},
			{Target: "ac-7", SourceID: "source", Start: secondStart, End: secondEnd, Transform: TransformListItem},
		}
		if _, err := Normalize(req); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("two mappings for ac-7: got err %v, want ErrInvalidRequest (duplicate explicit target)", err)
		}
	})

	t.Run("without retain_unmapped the unselected identity is still named", func(t *testing.T) {
		req := minimalRequest()
		req.Sources[0].Data = data
		req.RetainUnmapped = false
		req.Mappings = []Mapping{
			{Target: "ac-7", SourceID: "source", Start: firstStart, End: firstEnd, Transform: TransformListItem},
		}
		plan, err := Normalize(req)
		if err != nil {
			t.Fatalf("Normalize: unexpected error: %v", err)
		}
		if got := sourceIDFindings(plan.Findings, "ac-7"); len(got) != 1 {
			t.Fatalf("source-id findings for ac-7 = %d, want the unselected declaration named rather than folded into a generic unresolved-coverage finding: %+v", len(got), plan.Findings)
		}
	})

	t.Run("distinct declared ids resolve independently", func(t *testing.T) {
		raw := []byte("# T\n\n## Problem\n\np\n\n## Outcome\n\no\n\n## Acceptance Criteria\n\n" +
			"- ac-7: first declared item\n- ac-8: second declared item\n")
		start, end := spanOf(t, raw, "ac-7: first declared item")
		req := minimalRequest()
		req.Sources[0].Data = raw
		req.Mappings = []Mapping{
			{Target: "ac-7", SourceID: "source", Start: start, End: end, Transform: TransformListItem},
		}
		plan, err := Normalize(req)
		if err != nil {
			t.Fatalf("Normalize: unexpected error: %v", err)
		}
		if got := sourceIDFindings(plan.Findings, "ac-7"); len(got) != 0 {
			t.Fatalf("ac-7 still blocking after a mapping over its only declaring item: %+v", got)
		}
		got := sourceIDFindings(plan.Findings, "ac-8")
		if len(got) != 1 {
			t.Fatalf("source-id findings for ac-8 = %d, want its own untouched blocker: %+v", len(got), plan.Findings)
		}
		if strings.Contains(got[0].Message, "distinct id") {
			t.Errorf("a uniquely declared id carries duplicate-conflict guidance: %q", got[0].Message)
		}
	})
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
