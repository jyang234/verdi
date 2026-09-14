package specimport

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestNormalize_ValidateAcceptsMinimalRequest(t *testing.T) {
	if err := minimalRequest().Validate(); err != nil {
		t.Fatalf("Validate: unexpected error: %v", err)
	}
}

func TestNormalize_ValidateAcceptsZeroZeroLineRange(t *testing.T) {
	req := minimalRequest()
	req.Sources[0].StartLine, req.Sources[0].EndLine = 0, 0
	if err := req.Validate(); err != nil {
		t.Fatalf("Validate: unexpected error: %v", err)
	}
}

func TestNormalize_ValidateRejectsInvalidTargetSlug(t *testing.T) {
	req := minimalRequest()
	req.Target.Slug = "Not Valid!"
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest", err)
	}
}

func TestNormalize_ValidateRejectsInvalidTargetClass(t *testing.T) {
	req := minimalRequest()
	req.Target.Class = "component"
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest for class=component", err)
	}
}

func TestNormalize_ValidateRejectsBlankTitle(t *testing.T) {
	req := minimalRequest()
	req.Target.Title = "   "
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest", err)
	}
}

func TestNormalize_ValidateRejectsInvalidStoryRef(t *testing.T) {
	req := minimalRequest()
	req.Target.Class = "story"
	req.Target.Story = "not-a-scheme-ref"
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest", err)
	}
}

func TestNormalize_ValidateAcceptsValidStoryRef(t *testing.T) {
	req := minimalRequest()
	req.Target.Class = "story"
	req.Target.Story = "jira:LOAN-1482"
	if err := req.Validate(); err != nil {
		t.Fatalf("Validate: unexpected error: %v", err)
	}
}

func TestNormalize_ValidateRejectsTooFewSources(t *testing.T) {
	req := minimalRequest()
	req.Sources = nil
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest", err)
	}
}

func TestNormalize_ValidateRejectsTooManySources(t *testing.T) {
	req := minimalRequest()
	req.Sources = nil
	req.Format = FormatManualV1
	for i := 0; i < MaxSources+1; i++ {
		req.Sources = append(req.Sources, Source{
			ID:    sourceIDForIndex(i),
			Label: "s",
			Data:  []byte("x"),
		})
	}
	req.Primary = req.Sources[0].ID
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest for %d sources", err, len(req.Sources))
	}
}

func sourceIDForIndex(i int) string {
	return "s-" + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26))
}

func TestNormalize_ValidateRejectsDuplicateSourceIDs(t *testing.T) {
	req := minimalRequest()
	req.Sources = append(req.Sources, Source{ID: "source", Label: "dup.md", Data: []byte("x")})
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest for duplicate source ids", err)
	}
}

func TestNormalize_ValidateRejectsInvalidSourceIDSlug(t *testing.T) {
	req := minimalRequest()
	req.Sources[0].ID = "Source_1"
	req.Primary = "Source_1"
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest for invalid source id", err)
	}
}

func TestNormalize_ValidateRejectsPrimaryNotFound(t *testing.T) {
	req := minimalRequest()
	req.Primary = "does-not-exist"
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest", err)
	}
}

func TestNormalize_ValidateRejectsEmptySourceData(t *testing.T) {
	req := minimalRequest()
	req.Sources[0].Data = nil
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest", err)
	}
}

func TestNormalize_ValidateRejectsNonUTF8SourceData(t *testing.T) {
	req := minimalRequest()
	req.Sources[0].Data = []byte{0xff, 0xfe, 0xfd}
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest", err)
	}
}

func TestNormalize_ValidateRejectsOversizedSource(t *testing.T) {
	req := minimalRequest()
	req.Sources[0].Data = bytes.Repeat([]byte("a"), MaxSourceBytes+1)
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest", err)
	}
}

func TestNormalize_ValidateRejectsOversizedTotalSources(t *testing.T) {
	req := minimalRequest()
	req.Format = FormatManualV1
	big := bytes.Repeat([]byte("a"), MaxSourceBytes)
	req.Sources = []Source{
		{ID: "s1", Label: "s1.md", Data: big},
		{ID: "s2", Label: "s2.md", Data: big},
		{ID: "s3", Label: "s3.md", Data: big},
		{ID: "s4", Label: "s4.md", Data: big},
		{ID: "s5", Label: "s5.md", Data: big},
	}
	req.Primary = "s1"
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest for total bytes over the 8 MiB cap", err)
	}
}

func TestNormalize_ValidateRejectsMalformedLineRange(t *testing.T) {
	cases := []struct {
		name       string
		start, end int
	}{
		{"start zero end nonzero", 0, 5},
		{"start nonzero end zero", 5, 0},
		{"start after end", 5, 3},
		{"negative start", -1, 3},
		{"end beyond file", 1, 999},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := minimalRequest()
			req.Sources[0].StartLine = c.start
			req.Sources[0].EndLine = c.end
			if err := req.Validate(); !errors.Is(err, ErrInvalidSource) {
				t.Fatalf("Validate: got err %v, want ErrInvalidSource", err)
			}
		})
	}
}

func TestNormalize_ValidateRejectsDuplicateMappingTarget(t *testing.T) {
	req := minimalRequest()
	text1, text2 := "First addition.", "Second addition."
	req.Mappings = []Mapping{
		{Target: "outcome", Text: &text1},
		{Target: "outcome", Text: &text2},
	}
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest for duplicate mapping target", err)
	}
}

func TestNormalize_ValidateRejectsMappingBadTargetShape(t *testing.T) {
	req := minimalRequest()
	text := "Some text."
	req.Mappings = []Mapping{{Target: "not-an-object-id", Text: &text}}
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest", err)
	}
}

func TestNormalize_ValidateRejectsMappingBadTransform(t *testing.T) {
	req := minimalRequest()
	req.Mappings = []Mapping{{Target: "outcome", SourceID: "source", Start: 0, End: 5, Transform: "uppercase"}}
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest", err)
	}
}

func TestNormalize_ValidateRejectsMappingListItemOnNonObjectTarget(t *testing.T) {
	req := minimalRequest()
	req.Mappings = []Mapping{{Target: "outcome", SourceID: "source", Start: 0, End: 5, Transform: TransformListItem}}
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest", err)
	}
}

func TestNormalize_ValidateRejectsMappingEvidenceOnNonACTarget(t *testing.T) {
	req := minimalRequest()
	req.Mappings = []Mapping{{Target: "outcome", Evidence: []string{"static"}}}
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest", err)
	}
}

func TestNormalize_ValidateRejectsMappingDuplicateEvidenceKind(t *testing.T) {
	req := minimalRequest()
	req.Mappings = []Mapping{{Target: "ac-1", Evidence: []string{"static", "static"}}}
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest", err)
	}
}

func TestNormalize_ValidateRejectsMappingUnknownEvidenceKind(t *testing.T) {
	req := minimalRequest()
	req.Mappings = []Mapping{{Target: "ac-1", Evidence: []string{"vibes"}}}
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest", err)
	}
}

func TestNormalize_ValidateRejectsUserAddedMappingBlankText(t *testing.T) {
	req := minimalRequest()
	blank := "   "
	req.Mappings = []Mapping{{Target: "outcome", Text: &blank}}
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest", err)
	}
}

func TestNormalize_ValidateRejectsSourcelessMappingWithOffsets(t *testing.T) {
	req := minimalRequest()
	text := "Some text."
	req.Mappings = []Mapping{{Target: "outcome", Text: &text, Start: 1, End: 5}}
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest", err)
	}
}

func TestNormalize_ValidateAcceptsSourceBackedMappingShape(t *testing.T) {
	req := minimalRequest()
	req.Mappings = []Mapping{{Target: "outcome", SourceID: "source", Start: 0, End: 5, Transform: TransformIdentity}}
	if err := req.Validate(); err != nil {
		t.Fatalf("Validate: unexpected error: %v", err)
	}
}

func TestNormalize_ValidateAcceptsUserAddedMappingShape(t *testing.T) {
	req := minimalRequest()
	text := "A brand new requirement with no source backing."
	req.Mappings = []Mapping{{Target: "ac-9", Text: &text}}
	if err := req.Validate(); err != nil {
		t.Fatalf("Validate: unexpected error: %v", err)
	}
}

func TestNormalize_ValidateAcceptsEvidenceOnlyMappingShape(t *testing.T) {
	req := minimalRequest()
	req.Mappings = []Mapping{{Target: "ac-1", Evidence: []string{"static", "behavioral"}}}
	if err := req.Validate(); err != nil {
		t.Fatalf("Validate: unexpected error: %v", err)
	}
}

func TestNormalize_ValidateRejectsEmptyMapping(t *testing.T) {
	req := minimalRequest()
	req.Mappings = []Mapping{{Target: "outcome"}}
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest for a mapping matching no recognized shape", err)
	}
}

func TestNormalize_ValidateRejectsSourceBackedMappingMissingTransform(t *testing.T) {
	req := minimalRequest()
	req.Mappings = []Mapping{{Target: "outcome", SourceID: "source", Start: 0, End: 5}}
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest for a source-backed mapping with no transform", err)
	}
}

func TestNormalize_ValidateRejectsSourceBackedMappingWithEvidence(t *testing.T) {
	req := minimalRequest()
	req.Mappings = []Mapping{{Target: "ac-1", SourceID: "source", Start: 0, End: 5, Transform: TransformIdentity, Evidence: []string{"static"}}}
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest for a source-backed mapping also carrying evidence", err)
	}
}

func TestNormalize_ValidateRejectsUserAddedMappingWithEvidence(t *testing.T) {
	req := minimalRequest()
	text := "New criterion."
	req.Mappings = []Mapping{{Target: "ac-9", Text: &text, Evidence: []string{"static"}}}
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest for a user-added mapping also carrying evidence", err)
	}
}

func TestNormalize_ValidateRejectsMappingUnknownSourceID(t *testing.T) {
	req := minimalRequest()
	req.Mappings = []Mapping{{Target: "outcome", SourceID: "does-not-exist", Start: 0, End: 5, Transform: TransformIdentity}}
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest for an unknown mapping source_id", err)
	}
}

func TestNormalize_ValidateRejectsInvalidLinkType(t *testing.T) {
	req := minimalRequest()
	req.Links = []Link{{Type: "bogus-type", Ref: "spec/other-feature"}}
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest", err)
	}
}

func TestNormalize_ValidateRejectsInvalidLinkRef(t *testing.T) {
	req := minimalRequest()
	req.Links = []Link{{Type: "depends-on", Ref: "not a ref"}}
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest", err)
	}
}

func TestNormalize_ValidateRejectsNativeModeWithMappings(t *testing.T) {
	req := minimalRequest()
	req.Format = FormatNative
	text := "x"
	req.Mappings = []Mapping{{Target: "outcome", Text: &text}}
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest for native+mappings", err)
	}
}

func TestNormalize_ValidateRejectsNativeModeWithLinks(t *testing.T) {
	req := minimalRequest()
	req.Format = FormatNative
	req.Links = []Link{{Type: "depends-on", Ref: "spec/other"}}
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest for native+links", err)
	}
}

func TestNormalize_ValidateRejectsNativeModeWithDeferStatements(t *testing.T) {
	req := minimalRequest()
	req.Format = FormatNative
	req.DeferStatements = true
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest for native+defer_statements", err)
	}
}

// TestNormalize_ValidateRejectsOversizedCanonicalEnvelopeViaMappingText
// proves Validate's own 12 MiB canonical-JSON check — not just
// DecodeRequest's raw-byte check — catches an oversized direct Go struct.
// Mapping.Text has no per-field size cap of its own, so a single huge
// user-added mapping is the only way to exceed the envelope limit while
// staying within every per-source/total-source-byte limit.
func TestNormalize_ValidateRejectsOversizedCanonicalEnvelopeViaMappingText(t *testing.T) {
	req := minimalRequest()
	huge := strings.Repeat("a", MaxEnvelopeBytes)
	req.Mappings = []Mapping{{Target: "outcome", Text: &huge}}
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got err %v, want ErrInvalidRequest for oversized canonical envelope", err)
	}
}
