package corpus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
)

// invalidDir holds the decode-failure twins (PLAN.md phase 2 deliverable
// 4): each is a corpus file with exactly one injected defect — an unknown
// field, or a restricted-dialect violation (anchor, alias, custom tag) —
// proven here to fail loudly with an error naming the offense.
const invalidDir = "../../testdata/corpus-invalid"

func readInvalid(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(invalidDir, name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return data
}

// TestInvalidTwins_UnknownField proves each unknown-field twin fails
// KnownFields(true) decode and the error names the offending field.
func TestInvalidTwins_UnknownField(t *testing.T) {
	cases := []struct {
		file      string
		wantField string
		decode    func([]byte) error
	}{
		{
			file:      "spec-unknown-field.md",
			wantField: "bogus_extra_field",
			decode: func(fm []byte) error {
				_, err := artifact.DecodeSpec(fm)
				return err
			},
		},
		{
			file:      "adr-unknown-field.md",
			wantField: "severity",
			decode: func(fm []byte) error {
				_, err := artifact.DecodeADR(fm)
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			raw := readInvalid(t, tc.file)
			fm, _, err := artifact.SplitFrontmatter(raw)
			if err != nil {
				t.Fatalf("SplitFrontmatter: %v", err)
			}
			err = tc.decode(fm)
			if err == nil {
				t.Fatalf("%s: want decode error, got nil", tc.file)
			}
			if !strings.Contains(err.Error(), tc.wantField) {
				t.Fatalf("%s: error = %q, want it to name %q", tc.file, err, tc.wantField)
			}
		})
	}
}

// TestInvalidTwins_UnknownField_JSON is the JSON-record complement: board
// and evidence records with an injected unknown field must fail
// DisallowUnknownFields.
func TestInvalidTwins_UnknownField_JSON(t *testing.T) {
	cases := []struct {
		file      string
		wantField string
		decode    func([]byte) error
	}{
		{
			file:      "board-unknown-field.json",
			wantField: "extra_untracked_field",
			decode: func(data []byte) error {
				_, err := artifact.DecodeBoard(data)
				return err
			},
		},
		{
			file:      "evidence-unknown-field.json",
			wantField: "confidence",
			decode: func(data []byte) error {
				_, err := artifact.DecodeEvidence(data)
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			raw := readInvalid(t, tc.file)
			err := tc.decode(raw)
			if err == nil {
				t.Fatalf("%s: want decode error, got nil", tc.file)
			}
			if !strings.Contains(err.Error(), tc.wantField) {
				t.Fatalf("%s: error = %q, want it to name %q", tc.file, err, tc.wantField)
			}
		})
	}
}

// TestInvalidTwins_Dialect proves each dialect twin (anchor, alias, custom
// tag) fails checkDialect via DecodeStrict, with an error naming the
// dialect rule.
func TestInvalidTwins_Dialect(t *testing.T) {
	cases := []struct {
		file       string
		wantSubstr string
	}{
		{file: "spec-anchor.md", wantSubstr: "anchor"},
		{file: "spec-alias.md", wantSubstr: "anchor"}, // the anchor is what checkDialect trips on first
		{file: "spec-custom-tag.md", wantSubstr: "custom tag"},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			raw := readInvalid(t, tc.file)
			fm, _, err := artifact.SplitFrontmatter(raw)
			if err != nil {
				t.Fatalf("SplitFrontmatter: %v", err)
			}
			_, err = artifact.DecodeSpec(fm)
			if err == nil {
				t.Fatalf("%s: want dialect decode error, got nil", tc.file)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("%s: error = %q, want it to contain %q", tc.file, err, tc.wantSubstr)
			}
		})
	}
}
