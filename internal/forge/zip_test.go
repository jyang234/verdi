package forge

import (
	"archive/zip"
	"bytes"
	"testing"
)

func buildZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("zip.Create(%s): %v", name, err)
		}
		if _, err := fw.Write(content); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("closing zip: %v", err)
	}
	return buf.Bytes()
}

func TestExtractBundleFromZip_Happy(t *testing.T) {
	data := buildZip(t, map[string][]byte{
		"derived/spec--x/deadbeef/verdicts.json":      []byte(`[]`),
		"derived/spec--x/deadbeef/tests.json":         []byte(`{}`),
		"derived/spec--x/deadbeef/review.json":        []byte(`[]`),
		"derived/spec--x/deadbeef/boundary-diff.json": []byte(`[]`),
	})

	b, err := ExtractBundleFromZip(data)
	if err != nil {
		t.Fatalf("ExtractBundleFromZip: %v", err)
	}
	if string(b.Verdicts) != `[]` || string(b.Tests) != `{}` || string(b.Review) != `[]` || string(b.BoundaryDiff) != `[]` {
		t.Errorf("bundle = %+v", b)
	}
}

func TestExtractBundleFromZip_IgnoresExtraFiles(t *testing.T) {
	data := buildZip(t, map[string][]byte{
		"derived/spec--x/deadbeef/verdicts.json":      []byte(`[]`),
		"derived/spec--x/deadbeef/tests.json":         []byte(`{}`),
		"derived/spec--x/deadbeef/review.json":        []byte(`[]`),
		"derived/spec--x/deadbeef/boundary-diff.json": []byte(`[]`),
		"derived/spec--x/deadbeef/views/graph.mmd":    []byte(`graph TD`),
	})
	if _, err := ExtractBundleFromZip(data); err != nil {
		t.Fatalf("ExtractBundleFromZip with an extra file: %v", err)
	}
}

func TestExtractBundleFromZip_Negative(t *testing.T) {
	t.Run("not a zip", func(t *testing.T) {
		if _, err := ExtractBundleFromZip([]byte("not a zip")); err == nil {
			t.Fatal("want error, got nil")
		}
	})
	t.Run("missing a file", func(t *testing.T) {
		data := buildZip(t, map[string][]byte{
			"derived/spec--x/deadbeef/verdicts.json": []byte(`[]`),
		})
		if _, err := ExtractBundleFromZip(data); err == nil {
			t.Fatal("want error for missing tests/review/boundary-diff, got nil")
		}
	})
	t.Run("duplicate file", func(t *testing.T) {
		data := buildZip(t, map[string][]byte{
			"derived/spec--x/deadbeef/verdicts.json":      []byte(`[]`),
			"derived/other/deadbeef/verdicts.json":        []byte(`[{}]`),
			"derived/spec--x/deadbeef/tests.json":         []byte(`{}`),
			"derived/spec--x/deadbeef/review.json":        []byte(`[]`),
			"derived/spec--x/deadbeef/boundary-diff.json": []byte(`[]`),
		})
		if _, err := ExtractBundleFromZip(data); err == nil {
			t.Fatal("want error for a duplicated verdicts.json, got nil")
		}
	})
}

func TestDetectKind(t *testing.T) {
	cases := []struct {
		name          string
		manifestForge string
		remoteURL     string
		want          string
		wantErr       bool
	}{
		{"explicit gitlab", "gitlab", "https://example.com/x.git", "gitlab", false},
		{"explicit github", "github", "https://gitlab.com/x.git", "github", false},
		{"auto-detect gitlab.com", "", "git@gitlab.com:acme/svcfix.git", "gitlab", false},
		{"auto-detect github.com", "", "https://github.com/acme/svcfix.git", "github", false},
		{"auto-detect self-hosted gitlab", "", "https://gitlab.internal.acme.com/x.git", "gitlab", false},
		{"undetectable", "", "https://example.com/x.git", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DetectKind(tc.manifestForge, tc.remoteURL)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("DetectKind(%q, %q): want error, got nil", tc.manifestForge, tc.remoteURL)
				}
				return
			}
			if err != nil {
				t.Fatalf("DetectKind(%q, %q): %v", tc.manifestForge, tc.remoteURL, err)
			}
			if got != tc.want {
				t.Fatalf("DetectKind(%q, %q) = %q, want %q", tc.manifestForge, tc.remoteURL, got, tc.want)
			}
		})
	}
}
