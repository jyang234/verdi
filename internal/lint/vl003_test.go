package lint

import (
	"path/filepath"
	"testing"
)

func TestVL003_DanglingLink(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-003", "dangling-link"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-003")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

func TestVL003_DanglingPin(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-003", "dangling-pin"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-003")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}
