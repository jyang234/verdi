package lint

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestVL002_PathMismatch(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-002", "path-mismatch"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-002")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

// TestVL002_DuplicateRef proves the global-uniqueness sub-check fires for
// both files sharing id "adr/vl-002-duplicate". The overlay's on-disk
// filenames (vl-002-a.md / vl-002-b.md — neither can equal the shared id's
// implied filename, since two distinct files cannot both occupy that one
// path) also legitimately trip VL-002's own id/path-agreement sub-check;
// that is still exactly VL-002 firing, not a different rule, so onlyRule
// (rule-id equality) is satisfied either way.
func TestVL002_DuplicateRef(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-002", "duplicate-ref"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-002")

	dupCount := 0
	for _, f := range findings {
		if strings.Contains(f.Message, "declared by more than one file") {
			dupCount++
		}
	}
	if dupCount != 2 {
		t.Fatalf("got %d duplicate-ref findings, want 2 (one per file):\n%s", dupCount, findingsString(findings))
	}
}
