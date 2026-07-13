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

func TestExtractTreeFromZip_Happy(t *testing.T) {
	data := buildZip(t, map[string][]byte{
		"derived/spec--x/deadbeef/verdicts.json":      []byte(`[]`),
		"derived/spec--x/deadbeef/tests.json":         []byte(`{}`),
		"derived/spec--x/deadbeef/review.json":        []byte(`[]`),
		"derived/spec--x/deadbeef/boundary-diff.json": []byte(`[]`),
	})

	tree, err := ExtractTreeFromZip(data)
	if err != nil {
		t.Fatalf("ExtractTreeFromZip: %v", err)
	}
	// The leading "derived/" prefix is stripped so keys are relative to
	// data/derived/ — exactly the <key>/<commit>/<file> form sync writes
	// and readers look under.
	want := map[string]string{
		"spec--x/deadbeef/verdicts.json":      `[]`,
		"spec--x/deadbeef/tests.json":         `{}`,
		"spec--x/deadbeef/review.json":        `[]`,
		"spec--x/deadbeef/boundary-diff.json": `[]`,
	}
	if len(tree) != len(want) {
		t.Fatalf("tree has %d entries, want %d: %v", len(tree), len(want), tree)
	}
	for k, v := range want {
		if string(tree[k]) != v {
			t.Errorf("tree[%q] = %q, want %q", k, tree[k], v)
		}
	}
}

// TestExtractTreeFromZip_PreservesMultipleKeys is the crux of true-closure's
// keying fix: a real verdi-evidence artifact carries MORE THAN ONE
// verdicts.json — one per per-spec subdir selfevidence.go wrote, plus the
// branch-keyed per-service bundle. The extractor must preserve each at its
// own key, never collapse (or, as the pre-fix code did, error on the
// duplicate base name).
func TestExtractTreeFromZip_PreservesMultipleKeys(t *testing.T) {
	commit := "deadbeef"
	data := buildZip(t, map[string][]byte{
		"derived/build--stale-decline/" + commit + "/verdicts.json":      []byte(`[]`),
		"derived/build--stale-decline/" + commit + "/tests.json":         []byte(`{}`),
		"derived/build--stale-decline/" + commit + "/review.json":        []byte(`[]`),
		"derived/build--stale-decline/" + commit + "/boundary-diff.json": []byte(`[]`),
		"derived/spec--stale-decline/" + commit + "/verdicts.json":       []byte(`[{"kind":"static"}]`),
		"derived/spec--other/" + commit + "/verdicts.json":               []byte(`[{"kind":"behavioral"}]`),
	})

	tree, err := ExtractTreeFromZip(data)
	if err != nil {
		t.Fatalf("ExtractTreeFromZip: %v", err)
	}
	for _, key := range []string{
		"build--stale-decline/" + commit + "/verdicts.json",
		"spec--stale-decline/" + commit + "/verdicts.json",
		"spec--other/" + commit + "/verdicts.json",
	} {
		if _, ok := tree[key]; !ok {
			t.Errorf("tree missing key %q (a per-spec verdicts.json was dropped)", key)
		}
	}
	if string(tree["spec--stale-decline/"+commit+"/verdicts.json"]) != `[{"kind":"static"}]` {
		t.Errorf("per-spec verdicts.json content not preserved: %q", tree["spec--stale-decline/"+commit+"/verdicts.json"])
	}
}

// TestExtractTreeFromZip_NoPrefix proves an artifact whose entries are
// already relative to derived/ (no leading "derived/" segment — the shape
// actions/upload-artifact@v4 produces for `path: .verdi/data/derived/`) is
// keyed identically.
func TestExtractTreeFromZip_NoPrefix(t *testing.T) {
	data := buildZip(t, map[string][]byte{
		"spec--x/deadbeef/verdicts.json": []byte(`[]`),
	})
	tree, err := ExtractTreeFromZip(data)
	if err != nil {
		t.Fatalf("ExtractTreeFromZip: %v", err)
	}
	if _, ok := tree["spec--x/deadbeef/verdicts.json"]; !ok {
		t.Errorf("tree = %v, want key spec--x/deadbeef/verdicts.json", tree)
	}
}

// TestExtractTreeFromZip_RecognizesRuntimeJSON proves runtime.json
// (spec/runtime-evidence dc-2 — a sibling of verdicts.json carrying a real
// service's probe output) round-trips through the extractor exactly like
// the four pre-existing bundle files, so a CI-fetched artifact's runtime
// records reach the derived tree `verdi sync` writes to disk.
func TestExtractTreeFromZip_RecognizesRuntimeJSON(t *testing.T) {
	commit := "deadbeef"
	data := buildZip(t, map[string][]byte{
		"derived/spec--x/" + commit + "/verdicts.json": []byte(`[]`),
		"derived/spec--x/" + commit + "/runtime.json":  []byte(`[{"kind":"runtime"}]`),
	})

	tree, err := ExtractTreeFromZip(data)
	if err != nil {
		t.Fatalf("ExtractTreeFromZip: %v", err)
	}
	key := "spec--x/" + commit + "/runtime.json"
	if string(tree[key]) != `[{"kind":"runtime"}]` {
		t.Errorf("tree[%q] = %q, want the seeded runtime.json content", key, tree[key])
	}
}

func TestExtractTreeFromZip_IgnoresExtraFiles(t *testing.T) {
	data := buildZip(t, map[string][]byte{
		"derived/spec--x/deadbeef/verdicts.json":   []byte(`[]`),
		"derived/spec--x/deadbeef/views/graph.mmd": []byte(`graph TD`),
	})
	tree, err := ExtractTreeFromZip(data)
	if err != nil {
		t.Fatalf("ExtractTreeFromZip with an extra file: %v", err)
	}
	if len(tree) != 1 {
		t.Errorf("tree = %v, want only the recognized verdicts.json (views/ ignored)", tree)
	}
}

func TestExtractTreeFromZip_Negative(t *testing.T) {
	t.Run("not a zip", func(t *testing.T) {
		if _, err := ExtractTreeFromZip([]byte("not a zip")); err == nil {
			t.Fatal("want error, got nil")
		}
	})
	t.Run("no recognized derived file", func(t *testing.T) {
		data := buildZip(t, map[string][]byte{
			"derived/spec--x/deadbeef/README.md": []byte(`nope`),
		})
		if _, err := ExtractTreeFromZip(data); err == nil {
			t.Fatal("want error for an artifact with no recognized bundle file, got nil")
		}
	})
	t.Run("path traversal rejected", func(t *testing.T) {
		data := buildZip(t, map[string][]byte{
			"derived/../../../etc/verdicts.json": []byte(`[]`),
		})
		if _, err := ExtractTreeFromZip(data); err == nil {
			t.Fatal("want error for a zip-slip entry, got nil")
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
