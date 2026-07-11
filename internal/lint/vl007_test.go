package lint

import (
	"path/filepath"
	"testing"
)

func TestVL007_UnknownTopLevelEntry(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-007"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-007")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}
