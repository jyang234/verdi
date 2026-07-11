package lint

import (
	"path/filepath"
	"testing"
)

// TestVL004_EnforcedOnDefaultBranch proves VL-004 fires when linting the
// default branch (I-14): fixturegit always builds "main", so
// CurrentBranch == DefaultBranch == "main" enforces.
func TestVL004_EnforcedOnDefaultBranch(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-004"))
	findings := runLint(t, repo.Dir, Context{DefaultBranch: "main", CurrentBranch: "main"}, Options{})
	onlyRule(t, findings, "VL-004")

	found := false
	for _, f := range findings {
		if f.Path == ".verdi/specs/active/should-not-be-draft/spec.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no VL-004 finding for the overlay's draft spec:\n%s", findingsString(findings))
	}
}

// TestVL004_WarnsOffDefaultBranch proves I-14's "otherwise a warning, not
// a finding" posture: the same draft spec, linted with an unknown or
// differing branch context, produces zero findings.
func TestVL004_WarnsOffDefaultBranch(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-004"))

	t.Run("unknown default branch", func(t *testing.T) {
		findings := runLint(t, repo.Dir, Context{}, Options{})
		for _, f := range findings {
			if f.Rule == "VL-004" {
				t.Fatalf("VL-004 fired with no established default branch: %s", f.String())
			}
		}
	})

	t.Run("on a design branch, not the default", func(t *testing.T) {
		findings := runLint(t, repo.Dir, Context{DefaultBranch: "main", CurrentBranch: "feature/x"}, Options{})
		for _, f := range findings {
			if f.Rule == "VL-004" {
				t.Fatalf("VL-004 fired on a non-default branch: %s", f.String())
			}
		}
	})
}

// TestVL004_EnforcedViaCITargetBranch proves the "or a change targeting
// it" half of I-14: an MR/PR pipeline whose target branch is the default
// branch enforces even off the default branch itself.
func TestVL004_EnforcedViaCITargetBranch(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-004"))
	findings := runLint(t, repo.Dir, Context{
		DefaultBranch: "main",
		CurrentBranch: "feature/x",
		TargetBranch:  "main",
		InCI:          true,
	}, Options{})
	onlyRule(t, findings, "VL-004")
}
