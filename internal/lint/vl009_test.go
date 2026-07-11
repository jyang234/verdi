package lint

import (
	"path/filepath"
	"testing"
)

func TestVL009_FrozenCommitNotRealHistory(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-009"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-009")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}
