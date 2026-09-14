package specimport

import "testing"

func TestNormalize_ExplicitMappingOverlapSharesIntervalWithoutDoubleCounting(t *testing.T) {
	data := readMarkdownFixture(t, "positive-basic.md")
	req := minimalRequest()
	req.Sources[0].Data = data
	// Problem's automatic span is [29,111). Add an explicit constraint
	// mapping over a sub-range fully inside it, so [29,57) is claimed by
	// BOTH "problem" and "co-1".
	req.Mappings = []Mapping{
		{Target: "co-1", SourceID: "source", Start: 29, End: 57, Transform: TransformIdentity},
	}

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}

	co1, ok := fieldByTarget(plan.Fields, "co-1")
	if !ok {
		t.Fatalf("co-1 field missing: %+v", plan.Fields)
	}
	if co1.Text != string(data[29:57]) {
		t.Fatalf("co-1.Text = %q, want %q", co1.Text, string(data[29:57]))
	}

	cov := coverageForSource(t, plan, "source")
	if got, want := cov.MappedBytes+cov.RetainedBytes+cov.UnresolvedBytes, cov.TotalBytes; got != want {
		t.Fatalf("mapped+retained+unresolved = %d, want TotalBytes %d", got, want)
	}

	var overlap *Interval
	for i := range cov.Intervals {
		if cov.Intervals[i].Start == 29 && cov.Intervals[i].End == 57 {
			overlap = &cov.Intervals[i]
		}
	}
	if overlap == nil {
		t.Fatalf("no [29,57) interval in coverage: %+v", cov.Intervals)
	}
	if overlap.Disposition != DispositionMapped {
		t.Fatalf("overlap interval disposition = %q, want mapped", overlap.Disposition)
	}
	if len(overlap.Targets) != 2 || !containsString(overlap.Targets, "problem") || !containsString(overlap.Targets, "co-1") {
		t.Fatalf("overlap interval targets = %v, want both problem and co-1 listed, neither double-counted", overlap.Targets)
	}
	// MappedBytes must equal the UNION of every mapped span for this
	// source, counting the shared [29,57) sub-range once: problem
	// [29,111)=82, outcome [125,179)=54, ac-1 [207,242)=35,
	// ac-2 [245,282)=37, ac-3 [285,321)=36; co-1 [29,57) is wholly inside
	// problem's span so it contributes zero NEW bytes to the union. A
	// naive per-target sum (which would double-count the shared 28 bytes)
	// would instead total 272.
	const wantMappedBytes = 82 + 54 + 35 + 37 + 36
	if cov.MappedBytes != wantMappedBytes {
		t.Fatalf("MappedBytes = %d, want %d (the deduplicated union, not a naive per-target sum)", cov.MappedBytes, wantMappedBytes)
	}
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func TestNormalize_EvidenceOnlyMappingPreservesCopiedOriginAndSpan(t *testing.T) {
	data := readMarkdownFixture(t, "positive-basic.md")
	req := minimalRequest()
	req.Sources[0].Data = data
	req.Mappings = []Mapping{
		{Target: "ac-1", Evidence: []string{"static", "behavioral"}},
	}

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	ac1, ok := fieldByTarget(plan.Fields, "ac-1")
	if !ok {
		t.Fatalf("ac-1 missing: %+v", plan.Fields)
	}
	if ac1.Text != "The importer reads a Markdown file." {
		t.Fatalf("ac-1.Text changed by an evidence-only mapping: %q", ac1.Text)
	}
	if ac1.Origin != OriginCopiedSource {
		t.Fatalf("ac-1.Origin = %q, want copied-source preserved", ac1.Origin)
	}
	if len(ac1.Spans) != 1 || ac1.Spans[0].Start != 207 || ac1.Spans[0].End != 242 {
		t.Fatalf("ac-1.Spans changed by an evidence-only mapping: %+v", ac1.Spans)
	}
	if len(ac1.Evidence) != 2 || !containsString(ac1.Evidence, "static") || !containsString(ac1.Evidence, "behavioral") {
		t.Fatalf("ac-1.Evidence = %v, want [static behavioral]", ac1.Evidence)
	}
	if _, ok := findingForTarget(plan.Findings, FindingMissingEvidence, "ac-1"); ok {
		t.Fatalf("missing-evidence finding still present for ac-1 after an explicit evidence mapping: %+v", plan.Findings)
	}
	// The sibling criteria are untouched and still gap-flagged.
	for _, target := range []string{"ac-2", "ac-3"} {
		if _, ok := findingForTarget(plan.Findings, FindingMissingEvidence, target); !ok {
			t.Errorf("expected missing-evidence for untouched %s", target)
		}
	}
}

func TestNormalize_EvidenceOnlyMappingToAbsentTargetFails(t *testing.T) {
	req := minimalRequest()
	req.Mappings = []Mapping{{Target: "ac-99", Evidence: []string{"static"}}}
	if _, err := Normalize(req); err == nil {
		t.Fatal("Normalize: want an error for an evidence-only mapping to a nonexistent automatic target")
	}
}

func TestNormalize_UserAddedMappingCreatesNewFieldWithNoSourceSpan(t *testing.T) {
	req := minimalRequest()
	text := "A requirement with no source backing at all."
	req.Mappings = []Mapping{{Target: "ac-9", Text: &text}}

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	ac9, ok := fieldByTarget(plan.Fields, "ac-9")
	if !ok {
		t.Fatalf("ac-9 missing: %+v", plan.Fields)
	}
	if ac9.Origin != OriginUserAdded || ac9.Text != text {
		t.Fatalf("ac-9 = %+v, want Origin=user-added Text=%q", ac9, text)
	}
	if len(ac9.Spans) != 0 {
		t.Fatalf("ac-9.Spans = %+v, want none: a user-added field has no source byte range", ac9.Spans)
	}
	// A source-less field cannot appear in any source's coverage partition.
	for _, cov := range plan.Coverage {
		for _, iv := range cov.Intervals {
			if containsString(iv.Targets, "ac-9") {
				t.Fatalf("ac-9 unexpectedly appears in coverage for source %s: %+v", cov.SourceID, iv)
			}
		}
	}
}

func TestNormalize_SourceBackedMappingOverridesAutomaticFieldAsUserEdited(t *testing.T) {
	data := readMarkdownFixture(t, "positive-basic.md")
	req := minimalRequest()
	req.Sources[0].Data = data
	edited := "A corrected outcome statement."
	req.Mappings = []Mapping{
		{Target: "outcome", SourceID: "source", Start: 125, End: 179, Transform: TransformIdentity, Text: &edited},
	}

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	outcome, ok := fieldByTarget(plan.Fields, "outcome")
	if !ok {
		t.Fatalf("outcome missing: %+v", plan.Fields)
	}
	if outcome.Origin != OriginUserEditedSrc {
		t.Fatalf("outcome.Origin = %q, want user-edited-source", outcome.Origin)
	}
	if outcome.Text != edited {
		t.Fatalf("outcome.Text = %q, want the supplied edit %q", outcome.Text, edited)
	}
	if len(outcome.Spans) != 1 || outcome.Spans[0].Start != 125 || outcome.Spans[0].End != 179 {
		t.Fatalf("outcome.Spans = %+v, want the original selection retained", outcome.Spans)
	}
}

func TestNormalize_SourceBackedMappingRejectsOutOfRangeOffsets(t *testing.T) {
	req := minimalRequest()
	req.Mappings = []Mapping{{Target: "outcome", SourceID: "source", Start: 0, End: 100000, Transform: TransformIdentity}}
	if _, err := Normalize(req); err == nil {
		t.Fatal("Normalize: want an error for a mapping span beyond the selected source's length")
	}
}
