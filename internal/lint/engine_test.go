package lint

import (
	"context"
	"testing"
)

// TestEngine_Run_Happy proves NewEngine().Run over the clean corpus+setup
// repo (already exercised in depth by clean_test.go) returns a nil error
// and a deterministically-sorted, empty finding set.
func TestEngine_Run_Happy(t *testing.T) {
	repo := buildLintRepo(t)
	// runLint (not NewEngine().Run directly) so the known, documented
	// corpus-baseline VL-020 findings (harness_test.go's
	// knownCorpusBaselineFindings) are filtered exactly as every other
	// test in this package expects.
	findings := runLint(t, repo.Dir, Context{}, Options{})
	if len(findings) != 0 {
		t.Fatalf("got %d findings, want 0:\n%s", len(findings), findingsString(findings))
	}
}

// TestEngine_Run_Negative_NotAStoreRoot proves Run reports an operational
// error (not a Finding) when root has no .verdi/ directory at all.
func TestEngine_Run_Negative_NotAStoreRoot(t *testing.T) {
	dir := t.TempDir()
	_, err := NewEngine().Run(context.Background(), dir, Context{}, Options{})
	if err == nil {
		t.Fatal("Run on a non-store directory: want error, got nil")
	}
}

// TestEngine_Run_Sorted proves findings from multiple rules come back
// sorted deterministically by rule, then path, then message.
func TestEngine_Run_Sorted(t *testing.T) {
	repo := buildLintRepo(t, "../../testdata/violations/VL-007")
	findings, err := NewEngine().Run(context.Background(), repo.Dir, Context{}, Options{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for i := 1; i < len(findings); i++ {
		a, b := findings[i-1], findings[i]
		if a.Rule > b.Rule || (a.Rule == b.Rule && a.Path > b.Path) {
			t.Fatalf("findings not sorted at index %d: %v then %v", i, a, b)
		}
	}
}
