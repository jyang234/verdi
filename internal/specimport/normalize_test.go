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
