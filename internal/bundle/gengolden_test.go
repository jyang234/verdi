package bundle

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGenerateBundleGolden is opt-in tooling, not part of the regular
// suite: run manually (`VERDI_GENGOLDEN=1 go test -run
// TestGenerateBundleGolden -v ./internal/bundle`) to (re)produce
// testdata/svcfix-canned/bundle-golden/*.json — the golden
// cmd/verdi's TestRunSync_OrRegen_MatchesGolden compares `sync --or-regen`
// output against — from the same real captures every other test in this
// package uses. Skipped by default so `go test ./...`/`make verify` never
// depends on it (mirrors the fixture/fixture-regen split, PLAN.md §4).
func TestGenerateBundleGolden(t *testing.T) {
	if os.Getenv("VERDI_GENGOLDEN") == "" {
		t.Skip("set VERDI_GENGOLDEN=1 to (re)generate testdata/svcfix-canned/bundle-golden/")
	}
	dir := "../../testdata/svcfix-canned/bundle-golden"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	svc := testService(t)
	if err := Assemble(dir, []ServiceBundle{svc}, passingTestSummary()); err != nil {
		t.Fatal(err)
	}
	t.Log("wrote golden bundle to", filepath.Clean(dir))
}
