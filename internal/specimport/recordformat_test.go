package specimport

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestPublishNew_RecordFormat_MarkdownV1HasFormatNoProfileDigest proves
// Service.Apply populates a published record's Format from the request
// (uat-round-1 spec ac-4, closing UAT-005) and leaves ProfilePrimaryDigest
// absent for a format that names no pinned reference profile.
func TestPublishNew_RecordFormat_MarkdownV1HasFormatNoProfileDigest(t *testing.T) {
	_, record := publishedRecord(t, buildImportRepo(t).Dir, evidencedRequest())
	if record.Format != FormatMarkdownV1 {
		t.Fatalf("record.Format = %q, want %q", record.Format, FormatMarkdownV1)
	}
	if record.ProfilePrimaryDigest != "" {
		t.Fatalf("record.ProfilePrimaryDigest = %q, want absent for markdown-v1", record.ProfilePrimaryDigest)
	}
}

// TestPublishNew_RecordFormat_NativeHasFormatNoProfileDigest proves the
// same for native mode.
func TestPublishNew_RecordFormat_NativeHasFormatNoProfileDigest(t *testing.T) {
	_, record := publishedRecord(t, buildImportRepo(t).Dir, newSpecNativeRequest())
	if record.Format != FormatNative {
		t.Fatalf("record.Format = %q, want %q", record.Format, FormatNative)
	}
	if record.ProfilePrimaryDigest != "" {
		t.Fatalf("record.ProfilePrimaryDigest = %q, want absent for native", record.ProfilePrimaryDigest)
	}
}

// f13ReadyRequest builds a fully ready f13-reference-v1 Request: problem/
// outcome explicitly mapped from a support source (profile_f13_test.go's
// own TestNormalize_F13ProfileStatementsMappedFromASupportSource shape)
// and evidence supplied for all eight pinned criteria, so Preview reports
// Ready and Apply can actually publish a record to inspect.
func f13ReadyRequest(t *testing.T) Request {
	t.Helper()
	support := Source{ID: "notes", Label: "notes.md", Data: []byte("The gate cannot be audited.\n\nEvery gate decision is reviewable.\n")}
	req := f13Request(t, true, support)
	req.Mappings = []Mapping{
		{Target: "problem", SourceID: "notes", Start: 0, End: 27, Transform: TransformIdentity},
		{Target: "outcome", SourceID: "notes", Start: 29, End: 63, Transform: TransformIdentity},
	}
	for _, card := range f13PinnedCards {
		req.Mappings = append(req.Mappings, Mapping{Target: card.target, Evidence: []string{"static", "attestation"}})
	}
	return req
}

// TestPublishNew_RecordFormat_F13BindsThePinnedProfileDigest proves that
// for the one shipped reference-profile format, Apply records EXACTLY the
// profile's own pinned constant — never a digest recomputed from the
// request's selected bytes — as ac-4/co-1/co-2 of spec/uat-round-1 require.
func TestPublishNew_RecordFormat_F13BindsThePinnedProfileDigest(t *testing.T) {
	repo := buildImportRepo(t)
	svc := testService(t)
	req := f13ReadyRequest(t)
	ctx := context.Background()

	preview, err := svc.Preview(ctx, repo.Dir, req)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if !preview.Ready {
		t.Fatalf("preview not ready: %+v", preview.Findings)
	}
	result, err := svc.Apply(ctx, repo.Dir, req, preview.Digest, testAgent(t))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	view, err := ReadRecord(ctx, repo.Dir, result.Branch, req.Target.Slug)
	if err != nil {
		t.Fatalf("ReadRecord: %v", err)
	}
	record := view.Record
	if record.Format != FormatF13Reference {
		t.Fatalf("record.Format = %q, want %q", record.Format, FormatF13Reference)
	}
	if record.ProfilePrimaryDigest != f13PrimarySHA256 {
		t.Fatalf("record.ProfilePrimaryDigest = %q, want the exact pinned constant %q", record.ProfilePrimaryDigest, f13PrimarySHA256)
	}
}

// TestDecodeRecord_PreAC4Fixture_DecodesWithFieldsAbsent proves the ac-4
// addition is additive-compatible: a record.json committed before Format/
// ProfilePrimaryDigest existed (this fixture is a real published record
// with its "format" key removed, exactly what a pre-ac-4 binary wrote)
// still strict-decodes successfully, with both new fields reported as
// absent rather than a fabricated value or a decode failure (spec/
// uat-round-1 ac-4: "existing records without them decode with the fields
// absent").
func TestDecodeRecord_PreAC4Fixture_DecodesWithFieldsAbsent(t *testing.T) {
	data, err := os.ReadFile("testdata/record/pre-ac4-record.json")
	if err != nil {
		t.Fatal(err)
	}
	record, err := DecodeRecord(data)
	if err != nil {
		t.Fatalf("DecodeRecord(pre-ac-4 fixture) = %v, want a clean decode", err)
	}
	if record.Format != "" {
		t.Fatalf("record.Format = %q, want absent (\"\") for a pre-ac-4 record", record.Format)
	}
	if record.ProfilePrimaryDigest != "" {
		t.Fatalf("record.ProfilePrimaryDigest = %q, want absent (\"\") for a pre-ac-4 record", record.ProfilePrimaryDigest)
	}
	if record.SpecRef != "spec/sample-feature" {
		t.Fatalf("record.SpecRef = %q, want the fixture's spec/sample-feature (decode did not just silently zero the whole record)", record.SpecRef)
	}
}

// TestDecodeRecord_F13ReferenceFixture_CarriesFormatAndProfileDigest is
// the fixture-based counterpart proving a real, currently-produced record
// decodes with both new fields populated and correct.
func TestDecodeRecord_F13ReferenceFixture_CarriesFormatAndProfileDigest(t *testing.T) {
	data, err := os.ReadFile("testdata/record/f13-reference-record.json")
	if err != nil {
		t.Fatal(err)
	}
	record, err := DecodeRecord(data)
	if err != nil {
		t.Fatalf("DecodeRecord(f13-reference fixture) = %v, want a clean decode", err)
	}
	if record.Format != FormatF13Reference {
		t.Fatalf("record.Format = %q, want %q", record.Format, FormatF13Reference)
	}
	if record.ProfilePrimaryDigest != f13PrimarySHA256 {
		t.Fatalf("record.ProfilePrimaryDigest = %q, want the pinned constant %q", record.ProfilePrimaryDigest, f13PrimarySHA256)
	}

	// Positive serialization: the decoded struct fields alone do not prove
	// the wire form actually carries the digest under its documented
	// snake_case key — encodeRecord (canonjson) must emit it literally.
	encoded, err := encodeRecord(record)
	if err != nil {
		t.Fatalf("encodeRecord: %v", err)
	}
	wantSubstring := fmt.Sprintf(`"profile_primary_digest":%q`, f13PrimarySHA256)
	if !strings.Contains(string(encoded), wantSubstring) {
		t.Fatalf("encodeRecord(record) does not contain %s; got: %s", wantSubstring, encoded)
	}
}
