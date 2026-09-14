package specimport

import (
	"bytes"
	"errors"
	"testing"
)

// notesSource is a supporting source holding one of everything a list-item
// mapping must NOT be able to select, plus one real bullet item.
const notesSource = "Ordinary prose that is not a list at all.\n" +
	"\n" +
	"```sh\n" +
	"echo hello\n" +
	"```\n" +
	"\n" +
	"1. An ordered item.\n" +
	"\n" +
	"- A real bullet item.\n"

func spanOf(t *testing.T, data []byte, substring string) (int, int) {
	t.Helper()
	i := bytes.Index(data, []byte(substring))
	if i < 0 {
		t.Fatalf("substring %q not present in the fixture", substring)
	}
	return i, i + len(substring)
}

// TestNormalize_ListItemTransformRejectsSpansThatAreNotBulletItems pins
// "list-item is valid only for an actual supported direct list item span"
// (spec-import-contract.md). Every rejected span below previously produced
// a field marked copied-source with a mapped span, so arbitrary prose or a
// fenced block could be published as if it were a criterion the source
// declared as a list item.
func TestNormalize_ListItemTransformRejectsSpansThatAreNotBulletItems(t *testing.T) {
	notes := []byte(notesSource)
	proseStart, proseEnd := spanOf(t, notes, "Ordinary prose that is not a list at all.")
	fenceStart, fenceEnd := spanOf(t, notes, "```sh\necho hello\n```")
	orderedStart, orderedEnd := spanOf(t, notes, "An ordered item.")
	itemStart, itemEnd := spanOf(t, notes, "A real bullet item.")

	cases := []struct {
		name       string
		start, end int
	}{
		{"ordinary prose", proseStart, proseEnd},
		{"fenced code block", fenceStart, fenceEnd},
		{"ordered list item", orderedStart, orderedEnd},
		{"partial bullet item", itemStart, itemEnd - 5},
		{"whole source", 0, len(notes)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := minimalRequest()
			req.Sources = append(req.Sources, Source{ID: "notes", Label: "notes.md", Data: notes})
			req.Mappings = []Mapping{
				{Target: "ac-9", SourceID: "notes", Start: c.start, End: c.end, Transform: TransformListItem},
			}
			plan, err := Normalize(req)
			if !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("Normalize with a list-item mapping over %s: got err %v (fields %+v), want ErrInvalidRequest", c.name, err, plan.Fields)
			}
		})
	}

	t.Run("the real bullet item is accepted", func(t *testing.T) {
		req := minimalRequest()
		req.Sources = append(req.Sources, Source{ID: "notes", Label: "notes.md", Data: notes})
		req.Mappings = []Mapping{
			{Target: "ac-9", SourceID: "notes", Start: itemStart, End: itemEnd, Transform: TransformListItem},
		}
		plan, err := Normalize(req)
		if err != nil {
			t.Fatalf("Normalize: unexpected error for a real bullet item span: %v", err)
		}
		ac9, ok := fieldByTarget(plan.Fields, "ac-9")
		if !ok || ac9.Text != "A real bullet item." {
			t.Fatalf("ac-9 = %+v ok=%v, want the item's own text", ac9, ok)
		}
	})

	t.Run("the item including its bullet marker is accepted", func(t *testing.T) {
		req := minimalRequest()
		req.Sources = append(req.Sources, Source{ID: "notes", Label: "notes.md", Data: notes})
		req.Mappings = []Mapping{
			{Target: "ac-9", SourceID: "notes", Start: itemStart - 2, End: itemEnd, Transform: TransformListItem},
		}
		plan, err := Normalize(req)
		if err != nil {
			t.Fatalf("Normalize: unexpected error for a marker-inclusive item span: %v", err)
		}
		ac9, ok := fieldByTarget(plan.Fields, "ac-9")
		if !ok || ac9.Text != "A real bullet item." {
			t.Fatalf("ac-9 = %+v ok=%v, want the bullet marker stripped", ac9, ok)
		}
	})
}

// TestNormalize_ListItemTransformDeindentsLikeAutomaticExtraction pins the
// parent design's "Allowed formatting transformations must be named and
// reproducible": the same declared transform over the same shape must yield
// the same text whether it was applied automatically or asked for
// explicitly. The explicit path used to return the raw bytes, keeping the
// continuation line's source indentation the automatic path deindents.
func TestNormalize_ListItemTransformDeindentsLikeAutomaticExtraction(t *testing.T) {
	raw := []byte("# Widget Import\n\n## Problem\n\np\n\n## Outcome\n\no\n\n## Acceptance Criteria\n\n- The importer preserves wording\n  across a continuation line.\n- Second criterion.\n")

	automatic := minimalRequest()
	automatic.Sources[0].Data = raw
	autoPlan, err := Normalize(automatic)
	if err != nil {
		t.Fatalf("Normalize (automatic): unexpected error: %v", err)
	}
	ac1, ok := fieldByTarget(autoPlan.Fields, "ac-1")
	if !ok || len(ac1.Spans) != 1 {
		t.Fatalf("ac-1 = %+v ok=%v, want one automatically extracted span", ac1, ok)
	}
	const wantText = "The importer preserves wording\nacross a continuation line."
	if ac1.Text != wantText {
		t.Fatalf("automatic ac-1.Text = %q, want the deindented %q", ac1.Text, wantText)
	}

	explicit := minimalRequest()
	explicit.Sources[0].Data = raw
	explicit.Mappings = []Mapping{
		{Target: "ac-1", SourceID: "source", Start: ac1.Spans[0].Start, End: ac1.Spans[0].End, Transform: TransformListItem},
	}
	explicitPlan, err := Normalize(explicit)
	if err != nil {
		t.Fatalf("Normalize (explicit): unexpected error: %v", err)
	}
	got, ok := fieldByTarget(explicitPlan.Fields, "ac-1")
	if !ok {
		t.Fatalf("explicit ac-1 missing: %+v", explicitPlan.Fields)
	}
	if got.Text != ac1.Text {
		t.Fatalf("explicit list-item text = %q, want the same declared transform's output %q", got.Text, ac1.Text)
	}
	if got.Origin != OriginCopiedSource {
		t.Fatalf("explicit ac-1.Origin = %q, want copied-source (the text is unchanged)", got.Origin)
	}
}

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

// TestNormalize_CorrectedAndAddedCriteriaKeepTheirEvidence pins the
// correction journey the parent design requires ("User corrects a proposed
// mapping, creates the draft") and the declared Task 4 case "editing a
// mapping, selecting evidence". Because duplicate explicit targets fail, a
// second evidence-only mapping cannot repair a corrected criterion, so
// evidence declared on the correcting mapping itself must be preserved.
func TestNormalize_CorrectedAndAddedCriteriaKeepTheirEvidence(t *testing.T) {
	data := readMarkdownFixture(t, "positive-basic.md")
	start, end := spanOf(t, data, "The importer reports missing fields.")
	corrected := "The importer reports every missing field."
	added := "The importer refuses an unreadable file."

	req := minimalRequest()
	req.Sources[0].Data = data
	req.Mappings = []Mapping{
		{Target: "ac-3", SourceID: "source", Start: start, End: end, Transform: TransformListItem, Text: &corrected, Evidence: []string{"behavioral"}},
		{Target: "ac-9", Text: &added, Evidence: []string{"static", "attestation"}},
	}

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}

	ac3, ok := fieldByTarget(plan.Fields, "ac-3")
	if !ok {
		t.Fatalf("ac-3 missing: %+v", plan.Fields)
	}
	if ac3.Text != corrected || ac3.Origin != OriginUserEditedSrc {
		t.Fatalf("ac-3 = %+v, want the correction marked user-edited-source", ac3)
	}
	if len(ac3.Spans) != 1 || ac3.Spans[0].Start != start || ac3.Spans[0].End != end {
		t.Fatalf("ac-3.Spans = %+v, want the original selection retained", ac3.Spans)
	}
	if len(ac3.Evidence) != 1 || ac3.Evidence[0] != "behavioral" {
		t.Fatalf("ac-3.Evidence = %v, want the declared [behavioral]", ac3.Evidence)
	}

	ac9, ok := fieldByTarget(plan.Fields, "ac-9")
	if !ok {
		t.Fatalf("ac-9 missing: %+v", plan.Fields)
	}
	if ac9.Origin != OriginUserAdded || len(ac9.Spans) != 0 {
		t.Fatalf("ac-9 = %+v, want user-added with no fabricated source span", ac9)
	}
	if len(ac9.Evidence) != 2 || !containsString(ac9.Evidence, "static") || !containsString(ac9.Evidence, "attestation") {
		t.Fatalf("ac-9.Evidence = %v, want [static attestation]", ac9.Evidence)
	}

	for _, target := range []string{"ac-3", "ac-9"} {
		if _, ok := findingForTarget(plan.Findings, FindingMissingEvidence, target); ok {
			t.Errorf("missing-evidence still reported for %s after its evidence was declared: %+v", target, plan.Findings)
		}
	}
	// Criteria nobody supplied evidence for are still gap-flagged.
	for _, target := range []string{"ac-1", "ac-2"} {
		if _, ok := findingForTarget(plan.Findings, FindingMissingEvidence, target); !ok {
			t.Errorf("missing-evidence dropped for untouched %s", target)
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
