package workbench

import (
	"net/http"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/specimport"
)

// f13PinnedPrimaryDigest is the f13-reference-v1 profile's pinned primary
// SHA-256 (internal/specimport/profile_f13.go), the exact value an
// f13 record's profile_primary_digest carries (ac-4).
const f13PinnedPrimaryDigest = "7c6d95d4aa516cf682e6d4a33d1c260f1c861d938d46559241a7c764bffca577"

// TestRenderSpecImportRecord_FormatFacts (spec/uat-round-1 ac-4, wave
// ledger R-1): the record page's "Verified import" block states the
// request format and the pinned profile primary digest as two fact rows.
// Neither is ever blank: a pre-ac-4 record (Format absent) says so
// explicitly, and a format that names no reference profile says the
// digest is not applicable rather than implying a missing value.
func TestRenderSpecImportRecord_FormatFacts(t *testing.T) {
	const notRecorded = "not recorded (pre-ac-4 record)"
	cases := []struct {
		name       string
		format     string
		digest     string
		wantFormat string
		wantDigest string
	}{
		{
			name:       "f13 reference profile carries the pinned digest",
			format:     specimport.FormatF13Reference,
			digest:     f13PinnedPrimaryDigest,
			wantFormat: "f13-reference-v1",
			wantDigest: f13PinnedPrimaryDigest,
		},
		{
			name:       "markdown-v1 names no reference profile",
			format:     specimport.FormatMarkdownV1,
			wantFormat: "markdown-v1",
			wantDigest: "not applicable",
		},
		{
			name:       "native names no reference profile",
			format:     specimport.FormatNative,
			wantFormat: "native",
			wantDigest: "not applicable",
		},
		{
			name:       "manual-v1 names no reference profile",
			format:     specimport.FormatManualV1,
			wantFormat: "manual-v1",
			wantDigest: "not applicable",
		},
		{
			name:       "pre-ac-4 record without a format is disclosed, never blank",
			wantFormat: notRecorded,
			wantDigest: notRecorded,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			view := specimport.RecordView{
				Record: specimport.Record{
					SpecRef:              "spec/widget",
					Format:               tc.format,
					ProfilePrimaryDigest: tc.digest,
				},
				ImportCommit:       "abc123",
				CurrentSpecMatches: true,
			}
			out, err := renderSpecImportRecord(view, "design/widget", "widget")
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			html := string(out)
			gotFormat, n := testIDElementText(html, "record-format")
			if n != 1 {
				t.Fatalf("record-format elements = %d, want 1; page: %s", n, html)
			}
			if gotFormat != tc.wantFormat {
				t.Errorf("Format fact = %q, want %q", gotFormat, tc.wantFormat)
			}
			gotDigest, n := testIDElementText(html, "record-profile-primary-digest")
			if n != 1 {
				t.Fatalf("record-profile-primary-digest elements = %d, want 1; page: %s", n, html)
			}
			if gotDigest != tc.wantDigest {
				t.Errorf("Profile primary digest fact = %q, want %q", gotDigest, tc.wantDigest)
			}
			// Both rows live in the "Verified import" facts block, labelled.
			verified := strings.Index(html, "<h2>Verified import</h2>")
			sources := strings.Index(html, "<h2>Retained sources</h2>")
			for _, label := range []string{"<dt>Format</dt>", "<dt>Profile primary digest</dt>"} {
				at := strings.Index(html, label)
				if at < 0 || at < verified || at > sources {
					t.Errorf("%s at %d is not inside the Verified import block (%d..%d)", label, at, verified, sources)
				}
			}
		})
	}
}

// TestSpecImport_RecordView_ShowsFormatAndProfileDigest drives a real
// f13-reference-v1 import through the handler and reads the record page
// back: the persisted record's format and pinned digest reach the two
// fact rows (ac-4's "the record page ... display[s] them"), and a
// markdown-v1 import states the digest as not applicable.
func TestSpecImport_RecordView_ShowsFormatAndProfileDigest(t *testing.T) {
	root := newSpecImportStore(t)
	c := newImportClient(t, NewHandlerWith(root, Deps{}))

	var sources []any
	for _, name := range []string{"primary-f13", "transition-core", "review-journal", "review-validation", "stage-plan-c346c005"} {
		sources = append(sources, importSource(name, name+".md", readSpecImportFixture(t, "f13/"+name+".md")))
	}
	var mappings []any
	for i := 1; i <= 8; i++ {
		mappings = append(mappings, map[string]any{"target": "ac-" + string(rune('0'+i)), "evidence": []string{"attestation"}})
	}
	request := map[string]any{
		"schema":           specimport.RequestSchema,
		"target":           map[string]any{"slug": "gatekeeper-record", "class": "feature", "title": "Bounded gatekeeper state machine"},
		"format":           specimport.FormatF13Reference,
		"primary":          "primary-f13",
		"sources":          sources,
		"defer_statements": true,
		"retain_unmapped":  true,
		"mappings":         mappings,
	}
	status, ready, failure := c.preview(request)
	if status != http.StatusOK || !ready.Ready {
		t.Fatalf("f13 preview = %d ready=%v %+v %+v", status, ready.Ready, ready.Findings, failure)
	}
	status, created, failure := c.apply(request, ready.Digest, nil)
	if status != http.StatusOK {
		t.Fatalf("f13 apply = %d %+v %+v", status, created, failure)
	}
	status, page := c.get("/design/import/record?branch=design%2Fgatekeeper-record&spec=gatekeeper-record")
	if status != http.StatusOK {
		t.Fatalf("f13 record page = %d: %s", status, page)
	}
	if got, _ := testIDElementText(page, "record-format"); got != "f13-reference-v1" {
		t.Errorf("f13 record Format fact = %q, want %q", got, "f13-reference-v1")
	}
	if got, _ := testIDElementText(page, "record-profile-primary-digest"); got != f13PinnedPrimaryDigest {
		t.Errorf("f13 record Profile primary digest fact = %q, want the pinned %q", got, f13PinnedPrimaryDigest)
	}

	labeled := importLabeled(t, c, "labeled-record")
	status, page = c.get("/design/import/record?branch=design%2Flabeled-record&spec=labeled-record")
	if status != http.StatusOK {
		t.Fatalf("markdown record page = %d (%+v): %s", status, labeled, page)
	}
	if got, _ := testIDElementText(page, "record-format"); got != "markdown-v1" {
		t.Errorf("markdown record Format fact = %q, want %q", got, "markdown-v1")
	}
	if got, _ := testIDElementText(page, "record-profile-primary-digest"); got != "not applicable" {
		t.Errorf("markdown record Profile primary digest fact = %q, want %q", got, "not applicable")
	}
}
