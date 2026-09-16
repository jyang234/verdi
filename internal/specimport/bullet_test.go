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
			name:     "item beginning with a fenced block",
			list:     "- ```\n  code\n  ```\n- plain\n",
			wantText: "```\ncode\n```",
			wantSpan: "```\n  code\n  ```",
		},
		{
			name:     "item beginning with a blockquote",
			list:     "- > quoted\n- plain\n",
			wantText: "> quoted",
			wantSpan: "> quoted",
		},
		{
			name:     "item beginning with a blockquote, then a paragraph",
			list:     "- > quoted\n\n  after\n- plain\n",
			wantText: "> quoted\n\nafter",
			wantSpan: "> quoted\n\n  after",
		},
		{
			name:     "fenced block with an info string and an interior blank line",
			list:     "- ```go\n  a\n\n  b\n  ```\n- plain\n",
			wantText: "```go\na\n\nb\n```",
			wantSpan: "```go\n  a\n\n  b\n  ```",
		},
		{
			name:     "CRLF item beginning with a fenced block",
			list:     "- ```\r\n  code\r\n  ```\r\n- plain\r\n",
			wantText: "```\r\ncode\r\n```",
			wantSpan: "```\r\n  code\r\n  ```",
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

// TestNormalize_ListItemTransformStripsOnlyTheBulletMarker pins the width
// the declared transform removes: the item's own bullet marker and the
// spaces that set its content column, and nothing else. The body used to
// start at goldmark's first leaf SEGMENT, which sits inside whatever block
// opens the item, so an item opening with a fence or a blockquote silently
// lost that block's own syntax before it was published as copied-source.
func TestNormalize_ListItemTransformStripsOnlyTheBulletMarker(t *testing.T) {
	cases := []struct {
		name     string
		list     string
		wantText string
		wantSpan string
	}{
		{
			name:     "marker plus one space",
			list:     "- alpha\n  beta\n- plain\n",
			wantText: "alpha\nbeta",
			wantSpan: "alpha\n  beta",
		},
		{
			name:     "marker plus three spaces sets a wider content column",
			list:     "-   alpha\n    beta\n- plain\n",
			wantText: "alpha\nbeta",
			wantSpan: "alpha\n    beta",
		},
		{
			name:     "an indented item keeps its own content column",
			list:     "  - alpha\n    beta\n  - plain\n",
			wantText: "alpha\nbeta",
			wantSpan: "alpha\n    beta",
		},
		{
			name:     "content deferred to the line after the marker",
			list:     "-\n  alpha\n  beta\n- plain\n",
			wantText: "alpha\nbeta",
			wantSpan: "alpha\n  beta",
		},
		{
			name:     "a star marker is stripped like a dash",
			list:     "* alpha\n  beta\n* plain\n",
			wantText: "alpha\nbeta",
			wantSpan: "alpha\n  beta",
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
			if got := string(raw[ac1.Spans[0].Start:ac1.Spans[0].End]); got != c.wantSpan {
				t.Fatalf("ac-1 span bytes = %q, want %q", got, c.wantSpan)
			}
		})
	}
}

// TestNormalize_ListItemTransformKeepsEmptyItemsUnrepresentable pins the
// other side of the start boundary: an item really is unsupported when it
// has no body bytes at all, but an item whose body happens to carry no
// goldmark leaf segment — an empty fenced block, an empty blockquote — is
// ordinary flat-list content and must not be dropped as if it were empty.
func TestNormalize_ListItemTransformKeepsEmptyItemsUnrepresentable(t *testing.T) {
	t.Run("a genuinely empty item contributes nothing and consumes its ordinal", func(t *testing.T) {
		raw := []byte(bulletFixture("- alpha\n-\n- gamma\n"))
		req := minimalRequest()
		req.Sources[0].Data = raw
		plan, err := Normalize(req)
		if err != nil {
			t.Fatalf("Normalize: unexpected error: %v", err)
		}
		if ac1, ok := fieldByTarget(plan.Fields, "ac-1"); !ok || ac1.Text != "alpha" {
			t.Fatalf("ac-1 = %+v ok=%v", ac1, ok)
		}
		if ac2, ok := fieldByTarget(plan.Fields, "ac-2"); ok {
			t.Fatalf("the empty item produced a field %+v; it must contribute nothing", ac2)
		}
		if ac3, ok := fieldByTarget(plan.Fields, "ac-3"); !ok || ac3.Text != "gamma" {
			t.Fatalf("ac-3 = %+v ok=%v, want the empty item's ordinal still consumed", ac3, ok)
		}
	})

	t.Run("an item whose only block carries no leaf segment is still content", func(t *testing.T) {
		raw := []byte(bulletFixture("- ```\n  ```\n- plain\n"))
		req := minimalRequest()
		req.Sources[0].Data = raw
		plan, err := Normalize(req)
		if err != nil {
			t.Fatalf("Normalize: unexpected error: %v", err)
		}
		ac1, ok := fieldByTarget(plan.Fields, "ac-1")
		if !ok || len(ac1.Spans) != 1 {
			t.Fatalf("ac-1 = %+v ok=%v, want the empty fenced block kept as this item's body", ac1, ok)
		}
		if ac1.Text != "```\n```" {
			t.Fatalf("ac-1.Text = %q, want the fence delimiters themselves", ac1.Text)
		}
		if got := string(raw[ac1.Spans[0].Start:ac1.Spans[0].End]); got != "```\n  ```" {
			t.Fatalf("ac-1 span bytes = %q, want the item's complete body", got)
		}
	})
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
