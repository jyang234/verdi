package specimport

import (
	"errors"
	"testing"
)

// TestNormalize_CorePlanExample reproduces the implementation plan's own
// core positive example verbatim (docs/superpowers/plans/2026-09-14-
// mechanical-spec-import.md, Task 1's fixture-helper snippet).
func TestNormalize_CorePlanExample(t *testing.T) {
	raw := []byte("# Sample\n\n## Problem\n\nFirst line.\nSecond line.\n\n## Outcome\n\nAn outcome.\n")
	req := Request{Schema: RequestSchema, Format: FormatMarkdownV1,
		Target:         Target{Slug: "sample", Class: "feature", Title: "Sample"},
		Primary:        "source",
		Sources:        []Source{{ID: "source", Label: "sample.md", Data: raw}},
		RetainUnmapped: true,
	}
	plan, err := Normalize(req)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Fields[0].Target != "problem" || plan.Fields[0].Text != "First line.\nSecond line." {
		t.Fatalf("problem lost source content: %+v", plan.Fields)
	}
}

// TestNormalize_SnapshotDataIsTheSelectedBytes pins the retained-record
// coordinate system: Snapshot.Data is the SELECTED bytes, Digest hashes
// exactly those bytes, and every Span/Coverage offset indexes them. The
// original input's fingerprint and line coordinates survive as metadata
// (OriginalDigest/StartLine/EndLine) so an original-file check is still
// possible, but the bytes later tasks persist and verify in Git are the
// ones the user actually selected (parent design: "Retain selected source
// bytes with their path labels, byte digests").
func TestNormalize_SnapshotDataIsTheSelectedBytes(t *testing.T) {
	whole := "line one\nline two\n"
	req := Request{
		Schema:         RequestSchema,
		Format:         FormatManualV1,
		Target:         Target{Slug: "two-lines", Class: "feature", Title: "Two Lines"},
		Primary:        "source",
		Sources:        []Source{{ID: "source", Label: "two-lines.md", Data: []byte(whole), StartLine: 2, EndLine: 2}},
		RetainUnmapped: true,
	}
	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	if len(plan.Sources) != 1 {
		t.Fatalf("plan has %d snapshots, want 1", len(plan.Sources))
	}
	snap := plan.Sources[0]

	if string(snap.Data) != "line two\n" {
		t.Errorf("Snapshot.Data = %q, want only the selected line %q", snap.Data, "line two\n")
	}
	if got := sha256Hex(snap.Data); got != snap.Digest {
		t.Errorf("sha256(Snapshot.Data) = %s but Digest = %s; the retained bytes must hash to their own digest", got, snap.Digest)
	}
	if snap.OriginalDigest != sha256Hex([]byte(whole)) {
		t.Errorf("Snapshot.OriginalDigest = %s, want the whole input's digest %s", snap.OriginalDigest, sha256Hex([]byte(whole)))
	}
	if snap.OriginalDigest == snap.Digest {
		t.Error("OriginalDigest and Digest must differ here: a sub-range was selected")
	}
	if snap.StartLine != 2 || snap.EndLine != 2 {
		t.Errorf("Snapshot line coordinates = (%d,%d), want the original (2,2) retained as metadata", snap.StartLine, snap.EndLine)
	}

	cov := coverageForSource(t, plan, "source")
	if cov.TotalBytes != len(snap.Data) {
		t.Errorf("Coverage.TotalBytes = %d but len(Snapshot.Data) = %d; coverage must partition exactly the retained bytes", cov.TotalBytes, len(snap.Data))
	}
}

const nativeSpecFixture = `---
id: spec/native-widget
kind: spec
class: feature
title: "Native Widget"
status: draft
owners: [team-a]
acceptance_criteria:
  - { id: ac-1, text: "The widget works.", evidence: [static] }
---
# Native Widget

## ac-1

The widget works.
`

func nativeRequest() Request {
	return Request{
		Schema:  RequestSchema,
		Format:  FormatNative,
		Target:  Target{Slug: "native-widget", Class: "feature", Title: "Native Widget"},
		Primary: "source",
		Sources: []Source{{ID: "source", Label: "native-widget.md", Data: []byte(nativeSpecFixture)}},
	}
}

func TestNormalize_NativeAcceptsMatchingSpecAndPreservesBytesIdentically(t *testing.T) {
	req := nativeRequest()
	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	if string(plan.Native) != nativeSpecFixture {
		t.Fatalf("Plan.Native does not preserve the native primary byte-identically")
	}
	if len(plan.Fields) != 0 {
		t.Fatalf("native mode must not decompose content into Fields: %+v", plan.Fields)
	}
	cov := coverageForSource(t, plan, "source")
	if len(cov.Intervals) != 1 || cov.Intervals[0].Disposition != DispositionMapped {
		t.Fatalf("native coverage = %+v, want one mapped whole-document interval", cov)
	}
	if cov.MappedBytes != len(nativeSpecFixture) || cov.RetainedBytes != 0 || cov.UnresolvedBytes != 0 {
		t.Fatalf("native coverage bytes = %+v, want the whole document mapped", cov)
	}
}

func TestNormalize_NativeRejectsMismatchedTargetTitle(t *testing.T) {
	req := nativeRequest()
	req.Target.Title = "A Different Title"
	if _, err := Normalize(req); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Normalize: got err %v, want ErrInvalidRequest for a title mismatch", err)
	}
}

func TestNormalize_NativeRejectsMismatchedTargetSlug(t *testing.T) {
	req := nativeRequest()
	req.Target.Slug = "a-different-slug"
	if _, err := Normalize(req); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Normalize: got err %v, want ErrInvalidRequest for a slug mismatch", err)
	}
}

func TestNormalize_NativeRejectsNonDraftStatus(t *testing.T) {
	closed := `---
id: spec/native-widget
kind: spec
class: feature
title: "Native Widget"
status: closed
owners: [team-a]
acceptance_criteria:
  - { id: ac-1, text: "The widget works.", evidence: [static] }
frozen: { at: "2026-01-01", commit: "abcdef1" }
---
# Native Widget

## ac-1

The widget works.
`
	req := nativeRequest()
	req.Sources[0].Data = []byte(closed)
	if _, err := Normalize(req); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Normalize: got err %v, want ErrInvalidRequest for a closed (non-draft) native primary", err)
	}
}

func TestNormalize_NativeRejectsMalformedFrontmatter(t *testing.T) {
	req := nativeRequest()
	req.Sources[0].Data = []byte("---\nnot: [valid, spec, frontmatter\n---\nbody\n")
	if _, err := Normalize(req); err == nil {
		t.Fatal("Normalize: want an error for malformed native frontmatter")
	}
}

func TestNormalize_ManualV1RetainsEverythingAndReportsMissingStatements(t *testing.T) {
	req := Request{
		Schema:         RequestSchema,
		Format:         FormatManualV1,
		Target:         Target{Slug: "manual-import", Class: "feature", Title: "Manual Import"},
		Primary:        "source",
		Sources:        []Source{{ID: "source", Label: "raw.md", Data: []byte("# Anything\n\nUnstructured prose nobody mapped yet.\n")}},
		RetainUnmapped: true,
	}
	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	if len(plan.Fields) != 0 {
		t.Fatalf("manual-v1 must perform no automatic extraction: %+v", plan.Fields)
	}
	for _, target := range []string{"problem", "outcome"} {
		if _, ok := findingForTarget(plan.Findings, FindingMissingStatement, target); !ok {
			t.Errorf("manual-v1: no missing-statement finding for %s", target)
		}
	}
	cov := coverageForSource(t, plan, "source")
	if cov.RetainedBytes != cov.TotalBytes || cov.MappedBytes != 0 {
		t.Fatalf("manual-v1 coverage = %+v, want everything retained-only", cov)
	}
}

func TestNormalize_ManualV1ExplicitMappingSuppliesStatement(t *testing.T) {
	req := Request{
		Schema:  RequestSchema,
		Format:  FormatManualV1,
		Target:  Target{Slug: "manual-import", Class: "feature", Title: "Manual Import"},
		Primary: "source",
		Sources: []Source{{ID: "source", Label: "raw.md", Data: []byte("Some prose that becomes the outcome.\n")}},
		Mappings: []Mapping{
			{Target: "outcome", SourceID: "source", Start: 0, End: 36, Transform: TransformIdentity},
		},
		RetainUnmapped: true,
	}
	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	if _, ok := findingForTarget(plan.Findings, FindingMissingStatement, "outcome"); ok {
		t.Fatalf("missing-statement still present after an explicit manual-v1 mapping: %+v", plan.Findings)
	}
	outcome, ok := fieldByTarget(plan.Fields, "outcome")
	if !ok || outcome.Text != "Some prose that becomes the outcome." {
		t.Fatalf("outcome = %+v ok=%v", outcome, ok)
	}
}

// TestNormalize_ExplicitMappingDemotesOnlyItsOwnResolvedGap pins that a
// single Plan may not simultaneously carry a resolved Fields entry for a
// target and a blocking finding asserting that target is missing or
// ambiguous. The contract says "Explicit text/span Mappings override the
// automatic mapping for the same target" and the parent design says "A
// required statement absent from the source can be mapped by the user".
// The source-structure fact stays visible as a nonblocking disclosure; only
// the corresponding gap is demoted. Deferral remains Task 2's concern.
func TestNormalize_ExplicitMappingDemotesOnlyItsOwnResolvedGap(t *testing.T) {
	t.Run("missing statement mapped by the user", func(t *testing.T) {
		data := readMarkdownFixture(t, "missing-label.md")
		mapped := "The real problem is latency."
		req := minimalRequest()
		req.Sources[0].Data = data
		req.Mappings = []Mapping{{Target: "problem", Text: &mapped}}

		plan, err := Normalize(req)
		if err != nil {
			t.Fatalf("Normalize: unexpected error: %v", err)
		}
		if f, ok := fieldByTarget(plan.Fields, "problem"); !ok || f.Text != mapped {
			t.Fatalf("problem = %+v ok=%v, want the explicitly mapped value", f, ok)
		}
		resolved, ok := findingForTarget(plan.Findings, FindingMissingStatement, "problem")
		if !ok {
			t.Fatalf("the source-structure disclosure for problem was cleared entirely: %+v", plan.Findings)
		}
		if resolved.Blocking {
			t.Errorf("missing-statement for the explicitly mapped problem is still blocking: %+v", resolved)
		}
		unmapped, ok := findingForTarget(plan.Findings, FindingMissingStatement, "outcome")
		if !ok || !unmapped.Blocking {
			t.Errorf("missing-statement for the UNMAPPED outcome = %+v ok=%v, want it still blocking", unmapped, ok)
		}
	})

	t.Run("ambiguous statement mapped by the user", func(t *testing.T) {
		data := readMarkdownFixture(t, "duplicate-label.md")
		start, end := spanOf(t, data, "A second, conflicting problem statement.")
		req := minimalRequest()
		req.Sources[0].Data = data
		req.Mappings = []Mapping{{Target: "problem", SourceID: "source", Start: start, End: end, Transform: TransformIdentity}}

		plan, err := Normalize(req)
		if err != nil {
			t.Fatalf("Normalize: unexpected error: %v", err)
		}
		f, ok := findingForTarget(plan.Findings, FindingAmbiguousField, "problem")
		if !ok {
			t.Fatalf("the ambiguity disclosure for problem was cleared entirely: %+v", plan.Findings)
		}
		if f.Blocking {
			t.Errorf("ambiguous-field for the explicitly mapped problem is still blocking: %+v", f)
		}
	})

	t.Run("unrelated structural findings are untouched", func(t *testing.T) {
		data := readMarkdownFixture(t, "unresolved-list.md")
		mapped := "A corrected problem."
		req := minimalRequest()
		req.Sources[0].Data = data
		req.RetainUnmapped = false
		req.Mappings = []Mapping{{Target: "problem", Text: &mapped}}

		plan, err := Normalize(req)
		if err != nil {
			t.Fatalf("Normalize: unexpected error: %v", err)
		}
		objectAmbiguity, ok := findingForTarget(plan.Findings, FindingAmbiguousField, "acceptance-criteria")
		if !ok || !objectAmbiguity.Blocking {
			t.Errorf("object-section ambiguity = %+v ok=%v, want it still blocking", objectAmbiguity, ok)
		}
		unresolved, ok := findingForTarget(plan.Findings, FindingUnresolvedCoverage, "source")
		if !ok || !unresolved.Blocking {
			t.Errorf("unresolved-coverage = %+v ok=%v, want it still blocking", unresolved, ok)
		}
	})

	t.Run("multiple-targets and unsupported-structure are untouched", func(t *testing.T) {
		for _, fixture := range []string{"multiple-targets.md", "unsupported-structure.md"} {
			data := readMarkdownFixture(t, fixture)
			mappedProblem := "A corrected problem."
			mappedOutcome := "A corrected outcome."
			req := minimalRequest()
			req.Sources[0].Data = data
			req.Mappings = []Mapping{
				{Target: "problem", Text: &mappedProblem},
				{Target: "outcome", Text: &mappedOutcome},
			}
			plan, err := Normalize(req)
			if err != nil {
				t.Fatalf("Normalize(%s): unexpected error: %v", fixture, err)
			}
			for _, code := range []string{FindingMultipleTargets, FindingUnsupportedStructure} {
				if f, ok := findingByCode(plan.Findings, code); ok && !f.Blocking {
					t.Errorf("%s: %s was demoted by an unrelated statement mapping: %+v", fixture, code, f)
				}
			}
		}
	})

	t.Run("an empty explicit mapping resolves nothing", func(t *testing.T) {
		data := readMarkdownFixture(t, "missing-label.md")
		req := minimalRequest()
		req.Sources[0].Data = data
		req.Mappings = []Mapping{{Target: "problem", SourceID: "source", Start: 0, End: 0, Transform: TransformIdentity}}

		plan, err := Normalize(req)
		if err != nil {
			t.Fatalf("Normalize: unexpected error: %v", err)
		}
		f, ok := findingForTarget(plan.Findings, FindingMissingStatement, "problem")
		if !ok || !f.Blocking {
			t.Fatalf("missing-statement for an empty mapping = %+v ok=%v, want it still blocking", f, ok)
		}
	})
}

// TestNormalize_EmptyExplicitMappingsReportBlockingEmptyFields pins that a
// present mapping target is not a resolved value. An explicit mapping over
// an empty or blank span produced empty statements and objects with no
// finding at all — a Plan reporting a clean result for content it knows is
// empty. empty-field is in the closed vocabulary and is already emitted for
// automatic sections (spec-import-contract.md: "Duplicate aliases for one
// field, empty fields, unsupported nesting or multiple targets are
// reported"). Task 2 still owns canonical requiredness.
func TestNormalize_EmptyExplicitMappingsReportBlockingEmptyFields(t *testing.T) {
	req := Request{
		Schema:  RequestSchema,
		Format:  FormatManualV1,
		Target:  Target{Slug: "blank-import", Class: "feature", Title: "Blank Import"},
		Primary: "source",
		Sources: []Source{{ID: "source", Label: "blank.md", Data: []byte("   \n   \n")}},
		Mappings: []Mapping{
			{Target: "problem", SourceID: "source", Start: 0, End: 0, Transform: TransformIdentity},
			{Target: "outcome", SourceID: "source", Start: 0, End: 8, Transform: TransformTrimBlankLines},
			{Target: "co-1", SourceID: "source", Start: 0, End: 4, Transform: TransformCollapseWS},
		},
		RetainUnmapped: true,
	}
	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: unexpected error: %v", err)
	}
	for _, target := range []string{"problem", "outcome", "co-1"} {
		field, ok := fieldByTarget(plan.Fields, target)
		if !ok {
			t.Fatalf("%s field missing: %+v", target, plan.Fields)
		}
		if field.Text != "" {
			t.Fatalf("%s.Text = %q, want the empty value this fixture actually maps", target, field.Text)
		}
		f, ok := findingForTarget(plan.Findings, FindingEmptyField, target)
		if !ok {
			t.Fatalf("no empty-field finding for the empty mapped %s: %+v", target, plan.Findings)
		}
		if !f.Blocking {
			t.Errorf("empty-field finding for %s is not blocking: %+v", target, f)
		}
	}
}

// --- Direct Go-struct construction bypassing DecodeRequest entirely. ---

func TestNormalize_RejectsDirectStructWithInvalidDuplicateSourceIDs(t *testing.T) {
	req := minimalRequest()
	req.Sources = append(req.Sources, Source{ID: req.Sources[0].ID, Label: "dup", Data: []byte("x")})
	if _, err := Normalize(req); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Normalize on a hand-built request with duplicate source ids: got err %v, want ErrInvalidRequest", err)
	}
}

func TestNormalize_RejectsDirectStructWithInvalidSourceID(t *testing.T) {
	req := minimalRequest()
	req.Sources[0].ID = "NOT VALID"
	req.Primary = "NOT VALID"
	if _, err := Normalize(req); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Normalize on a hand-built request with an invalid source id: got err %v, want ErrInvalidRequest", err)
	}
}

func TestNormalize_RejectsDirectStructWithUnknownFormatEnum(t *testing.T) {
	req := minimalRequest()
	req.Format = "yaml-v7"
	if _, err := Normalize(req); !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("Normalize on a hand-built request with an unknown format: got err %v, want ErrUnsupportedFormat", err)
	}
}

func TestNormalize_RejectsDirectStructWithOversizedSource(t *testing.T) {
	req := minimalRequest()
	req.Sources[0].Data = make([]byte, MaxSourceBytes+1)
	for i := range req.Sources[0].Data {
		req.Sources[0].Data[i] = 'a'
	}
	if _, err := Normalize(req); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Normalize on a hand-built oversized source: got err %v, want ErrInvalidRequest", err)
	}
}

func TestNormalize_RejectsDirectStructWithInvalidMappingTarget(t *testing.T) {
	req := minimalRequest()
	text := "x"
	req.Mappings = []Mapping{{Target: "not-a-real-target!!", Text: &text}}
	if _, err := Normalize(req); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Normalize on a hand-built request with an invalid mapping target: got err %v, want ErrInvalidRequest", err)
	}
}
