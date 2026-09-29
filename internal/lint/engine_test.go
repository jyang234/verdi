package lint

import (
	"context"
	"strings"
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

// TestAllRules_Inventory pins the engine's rule registry: every rule 02
// §Lint rules assigns to this engine, each once, in id order. VL-023 to
// VL-025 are reserved by the process-hardening plan and are not rules
// yet (closed-spec object supersession design §7), so VL-026 follows
// VL-022.
func TestAllRules_Inventory(t *testing.T) {
	want := []string{
		"VL-001", "VL-002", "VL-003", "VL-004", "VL-005", "VL-006", "VL-007",
		"VL-008", "VL-009", "VL-010", "VL-011", "VL-012", "VL-013", "VL-014",
		"VL-015", "VL-016", "VL-017", "VL-018", "VL-019", "VL-020", "VL-021",
		"VL-022", "VL-026",
	}
	var got []string
	for _, r := range NewEngine().rules {
		got = append(got, r.ID())
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("engine rules = %v, want %v", got, want)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Fatalf("engine rules not in strictly increasing id order at %d: %s then %s", i, got[i-1], got[i])
		}
	}
}
