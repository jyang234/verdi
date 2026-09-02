package lint

import (
	"os"
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

// TestVL007_ConstitutionInventoryAdmitted pins SI-179 D1's narrow admission:
// VL-007 recognizes the constitution directory while its consumers.json
// grammar remains loader-owned by internal/constitutionimpact.
func TestVL007_ConstitutionInventoryAdmitted(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join("..", "..", "testdata", "constitutionadmission"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	if len(findings) != 0 {
		t.Fatalf("got %d findings, want 0:\n%s", len(findings), findingsString(findings))
	}

	unknown := filepath.Join(repo.Dir, ".verdi", "constitution-next")
	if err := os.Mkdir(unknown, 0o755); err != nil {
		t.Fatal(err)
	}
	findings = runLint(t, repo.Dir, Context{}, Options{})
	if len(findings) != 1 || findings[0].Rule != "VL-007" || findings[0].Path != ".verdi/constitution-next" {
		t.Fatalf("unknown sibling findings = %#v, want one VL-007 refusal", findings)
	}
}
