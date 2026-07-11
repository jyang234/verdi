package lint

import "testing"

// TestClean_CorpusLintsGreen proves the fixture corpus plus the lint test
// setup layer (manifest + gitattributes) produces zero findings from any
// rule — the exit-criteria baseline every VL-xxx overlay test is measured
// against (PLAN.md Phase 4: "go run ./cmd/verdi lint exits 0 on the
// fixture corpus").
func TestClean_CorpusLintsGreen(t *testing.T) {
	repo := buildLintRepo(t)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	if len(findings) != 0 {
		t.Fatalf("clean corpus: got %d findings, want 0:\n%s", len(findings), findingsString(findings))
	}
}
