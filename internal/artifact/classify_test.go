package artifact

import "testing"

func TestClassifyPath_Happy(t *testing.T) {
	cases := []struct {
		rel      string
		wantKind string
	}{
		{"adr/0001-a.md", "adr"},
		{"diagrams/flow.mermaid", "diagram"},
		{"attestations/foo.md", "attestation"},
		{"waivers/bar.md", "waiver"},
		{"conflicts/baz.md", "conflict"},
		// the reaffirmation case: present in the old lint/walk.go table,
		// missing from the old index/walk.go table (spec/shared-homes
		// ac-4's divergence finding) — asserted explicitly here so a
		// future regression on either consumer is caught at the shared
		// source of truth.
		{"reaffirmations/jira-loan-1483/ac-1.md", "reaffirmation"},
		{"obligations/loansvc/ac-1--spec.md", "obligation"},
		{"specs/active/my-spec/spec.md", "spec"},
		{"specs/archive/old-spec/spec.md", "spec"},
	}
	for _, c := range cases {
		t.Run(c.rel, func(t *testing.T) {
			kind, ok := ClassifyPath(c.rel)
			if !ok {
				t.Fatalf("ClassifyPath(%q) ok = false, want true", c.rel)
			}
			if kind != c.wantKind {
				t.Fatalf("ClassifyPath(%q) kind = %q, want %q", c.rel, kind, c.wantKind)
			}
		})
	}
}

func TestClassifyPath_Negative(t *testing.T) {
	cases := []string{
		"verdi.yaml",
		".gitignore",
		"data/foo.json",
		"specs/active/my-spec/board.json",
		"specs/active/my-spec/rollup.json",
		"specs/active/my-spec/deviation-report.md",
		"adr/0001-a.txt",              // wrong extension
		"diagrams/flow.md",            // wrong extension for diagram dir
		"unknown-dir/file.md",         // not a known top-level artifact dir
		"specs/other/my-spec/spec.md", // neither active nor archive
		"",
	}
	for _, rel := range cases {
		t.Run(rel, func(t *testing.T) {
			kind, ok := ClassifyPath(rel)
			if ok {
				t.Fatalf("ClassifyPath(%q) ok = true (kind %q), want false — fail closed on unknown", rel, kind)
			}
			if kind != "" {
				t.Fatalf("ClassifyPath(%q) kind = %q, want empty on ok=false", rel, kind)
			}
		})
	}
}
