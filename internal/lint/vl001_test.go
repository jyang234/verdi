package lint

import (
	"path/filepath"
	"testing"
)

// TestVL001_Overlays layers testdata/violations/VL-001/ (five files
// covering unknown-field, anchor, alias, custom-tag, and missing-
// frontmatter) onto the corpus and asserts every finding is VL-001, one
// per offending file.
func TestVL001_Overlays(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-001"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-001")

	want := []string{
		"vl-001-unknown-field.md",
		"vl-001-anchor.md",
		"vl-001-alias.md",
		"vl-001-custom-tag.md",
		"missing-frontmatter.md",
	}
	got := map[string]bool{}
	for _, f := range findings {
		got[filepath.Base(f.Path)] = true
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("no VL-001 finding for %s\nfull findings:\n%s", w, findingsString(findings))
		}
	}
	if len(findings) != len(want) {
		t.Errorf("got %d findings, want exactly %d (one per overlay file):\n%s", len(findings), len(want), findingsString(findings))
	}
}
