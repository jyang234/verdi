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

// TestVL007_PolicyDirAdmitted proves the SI-6 admission: a store carrying
// the constitution home .verdi/policy/ (the ratified verdi-store-layout
// policy/ entry, internal grammar owned by the policy-authority unit)
// produces no VL-007 finding — and no other finding either, since the
// constitution kinds live outside lint's 02-registry classification walk
// and are validated at the constitution store's own load seam instead.
func TestVL007_PolicyDirAdmitted(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join("..", "..", "testdata", "policyadmission"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	if len(findings) != 0 {
		t.Fatalf("got %d findings, want 0:\n%s", len(findings), findingsString(findings))
	}
}
