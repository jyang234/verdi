package specimport

import "testing"

// bulletFixture wraps one Acceptance Criteria list in the smallest complete
// markdown-v1 document, so an item's behaviour is observed through the
// public Normalize gate rather than through the unexported extractor.
func bulletFixture(list string) string {
	return "# T\n\n## Problem\n\np\n\n## Outcome\n\no\n\n## Acceptance Criteria\n\n" + list
}

// mappedIntervalFor returns the coverage interval that exactly matches
// [start,end) and lists target among its destinations.
func mappedIntervalFor(t *testing.T, cov Coverage, start, end int, target string) Interval {
	t.Helper()
	for _, iv := range cov.Intervals {
		if iv.Start == start && iv.End == end && iv.Disposition == DispositionMapped && containsString(iv.Targets, target) {
			return iv
		}
	}
	t.Fatalf("no mapped [%d,%d) interval for %s in %+v", start, end, target, cov.Intervals)
	return Interval{}
}

// TestNormalize_ListItemTransformPreservesEveryInteriorByte pins the
// contract's "Strip only the bullet marker and its following space;
// multiline continuation indentation is a declared deterministic deindent
// transform... No punctuation or words are paraphrased."
//
// The transform used to be a concatenation of goldmark's per-line leaf
// segments. goldmark trims a paragraph's trailing newline when it closes
// the block and never emits a segment for a fence delimiter, a blockquote
// marker or the blank line between two blocks, so a multi-block item fused
// its own words ("alpha" + "beta" = "alphabeta"), dropped the syntax of its
// interior blocks, and recorded a span that stopped at the last code line
// instead of the item's closing fence.
func TestNormalize_ListItemTransformPreservesEveryInteriorByte(t *testing.T) {
	cases := []struct {
		name     string
		list     string
		wantText string
		wantSpan string
	}{
		{
			name:     "loose item with two paragraphs",
			list:     "- alpha\n\n  beta\n- plain\n",
			wantText: "alpha\n\nbeta",
			wantSpan: "alpha\n\n  beta",
		},
		{
			name:     "item with an interior fenced block",
			list:     "- alpha\n\n  ```\n  code line\n  ```\n- plain\n",
			wantText: "alpha\n\n```\ncode line\n```",
			wantSpan: "alpha\n\n  ```\n  code line\n  ```",
		},
		{
			name:     "item with an interior blockquote",
			list:     "- alpha\n\n  > quoted\n- plain\n",
			wantText: "alpha\n\n> quoted",
			wantSpan: "alpha\n\n  > quoted",
		},
		{
			name:     "single paragraph over two lines",
			list:     "- alpha\n  beta\n- plain\n",
			wantText: "alpha\nbeta",
			wantSpan: "alpha\n  beta",
		},
		{
			name:     "CRLF continuation line",
			list:     "- wrapped criterion\r\n  continued here\r\n- plain\r\n",
			wantText: "wrapped criterion\r\ncontinued here",
			wantSpan: "wrapped criterion\r\n  continued here",
		},
		{
			name:     "CRLF loose item with two paragraphs",
			list:     "- alpha\r\n\r\n  beta\r\n- plain\r\n",
			wantText: "alpha\r\n\r\nbeta",
			wantSpan: "alpha\r\n\r\n  beta",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw := []byte(bulletFixture(c.list))
			req := minimalRequest()
			req.Sources[0].Data = raw

			plan, err := Normalize(req)
			if err != nil {
				t.Fatalf("Normalize: unexpected error: %v", err)
			}
			ac1, ok := fieldByTarget(plan.Fields, "ac-1")
			if !ok || len(ac1.Spans) != 1 {
				t.Fatalf("ac-1 = %+v ok=%v, want one automatically extracted span (findings %+v)", ac1, ok, plan.Findings)
			}
			if ac1.Text != c.wantText {
				t.Fatalf("ac-1.Text = %q, want %q", ac1.Text, c.wantText)
			}
			span := ac1.Spans[0]
			if got := string(raw[span.Start:span.End]); got != c.wantSpan {
				t.Fatalf("ac-1 span bytes = %q, want the item's complete body %q", got, c.wantSpan)
			}

			// The recorded span is the coverage claim: every interior byte of
			// the item — including a closing fence delimiter — is mapped to
			// ac-1, not reported as unmapped complement.
			cov := coverageForSource(t, plan, "source")
			mappedIntervalFor(t, cov, span.Start, span.End, "ac-1")
			if got, want := cov.MappedBytes+cov.RetainedBytes+cov.UnresolvedBytes, cov.TotalBytes; got != want {
				t.Fatalf("mapped+retained+unresolved = %d, want TotalBytes %d", got, want)
			}

			// The explicit path must reproduce the same declared transform
			// over the same span (spec-import-contract.md: allowed
			// transformations are named and reproducible).
			explicit := minimalRequest()
			explicit.Sources[0].Data = raw
			explicit.Mappings = []Mapping{
				{Target: "ac-1", SourceID: "source", Start: span.Start, End: span.End, Transform: TransformListItem},
			}
			explicitPlan, err := Normalize(explicit)
			if err != nil {
				t.Fatalf("Normalize (explicit): unexpected error: %v", err)
			}
			got, ok := fieldByTarget(explicitPlan.Fields, "ac-1")
			if !ok {
				t.Fatalf("explicit ac-1 missing: %+v", explicitPlan.Fields)
			}
			if got.Text != c.wantText {
				t.Fatalf("explicit list-item text = %q, want the same declared transform's output %q", got.Text, c.wantText)
			}
			if got.Origin != OriginCopiedSource {
				t.Fatalf("explicit ac-1.Origin = %q, want copied-source (the text is unchanged)", got.Origin)
			}
		})
	}
}

// TestNormalize_ListItemTransformKeepsNestedListsUnsupported pins that
// preserving multi-block items does not widen the accepted grammar: a
// nested list still makes its section unresolved and is still not
// selectable as an explicit list-item span.
func TestNormalize_ListItemTransformKeepsNestedListsUnsupported(t *testing.T) {
	raw := []byte(bulletFixture("- outer item\n\n  - inner item\n- plain\n"))
	req := minimalRequest()
	req.Sources[0].Data = raw

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	if _, ok := fieldByTarget(plan.Fields, "ac-1"); ok {
		t.Fatalf("a nested list produced an automatic criterion: %+v", plan.Fields)
	}
	if _, ok := findingForTarget(plan.Findings, FindingAmbiguousField, "acceptance-criteria"); !ok {
		t.Fatalf("no ambiguous-field finding for a nested list: %+v", plan.Findings)
	}
}
