package lint

import (
	"path/filepath"
	"testing"
)

func TestVL014_MissingSticky(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-014", "missing-sticky"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-014")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

func TestVL014_DanglingDisposition(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-014", "dangling-disposition"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-014")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

func TestVL014_IncorporatedWithoutWhere(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-014", "incorporated-without-where"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-014")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

func TestVL014_ContradictedWithoutNote(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-014", "contradicted-without-note"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-014")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

func TestVL014_UnresolvableWhereAnchor(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-014", "unresolvable-where-anchor"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-014")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}
