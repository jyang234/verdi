package lint

import (
	"path/filepath"
	"testing"
)

// TestVL012_MissingGitattributesLines layers testdata/violations/VL-012/,
// which replaces the harness's own well-formed .gitattributes (from
// setupLayer) with one missing the required generated-attribute lines —
// overlays are layered as later git commits, so a later layer's file
// content wins.
func TestVL012_MissingGitattributesLines(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-012"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-012")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	if findings[0].Path != ".gitattributes" {
		t.Fatalf("finding path = %q, want .gitattributes", findings[0].Path)
	}
}
