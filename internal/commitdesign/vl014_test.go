package commitdesign

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/lint"
)

// TestRun_OutputPassesVL014 is the VL-014 interplay test PLAN.md Phase 10's
// exit criteria demand: "running commit-to-design on the fixture board
// yields output that passes `verdi lint` when every sticky is
// dispositioned". Run's own output is fed into the real, in-process
// artifactlint engine (internal/lint) — not a re-implementation of VL-014's
// rule, the actual one phase 4 shipped.
func TestRun_OutputPassesVL014(t *testing.T) {
	repo := buildRepo(t)
	seedBoard(t, repo)
	ctx := context.Background()

	if _, err := Run(ctx, Input{Root: repo.Dir, BoardKey: "STORY-1482", SpecName: "vl014-happy", StoryRef: "jira:LOAN-1482"}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	findings := runLintEngine(t, repo.Dir)
	if got := findingsForRule(findings, "VL-014"); len(got) != 0 {
		t.Fatalf("commit-to-design output failed VL-014:\n%s", findingsString(got))
	}
}

// TestRun_RemovingAStickyFromTheDispositionsBlock_FailsVL014Exactly proves
// the other half of the exit criterion: editing the committed spec to drop
// one disposition entry (as if a careless hand-edit happened after the
// ritual) makes VL-014 fail, and ONLY VL-014 — the exact rule id, no
// unrelated rule storm, and the message names the specific dangling
// sticky.
func TestRun_RemovingAStickyFromTheDispositionsBlock_FailsVL014Exactly(t *testing.T) {
	repo := buildRepo(t)
	seedBoard(t, repo)
	ctx := context.Background()

	res, err := Run(ctx, Input{Root: repo.Dir, BoardKey: "STORY-1482", SpecName: "vl014-missing", StoryRef: "jira:LOAN-1482"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The seeded board has two stickies; Run's own scaffold dispositions
	// both. Rewrite spec.md dropping the SECOND disposition entry only,
	// leaving the first — this models "a sticky in board.json has no
	// dispositions[] entry" (VL-014's bidirectional-completeness half).
	specPath := filepath.Join(repo.Dir, res.SpecRelPath)
	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	edited := removeSecondDispositionLine(t, string(raw))
	if err := os.WriteFile(specPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAll(t, repo.Dir, "test: drop a disposition entry")

	findings := runLintEngine(t, repo.Dir)
	got := findingsForRule(findings, "VL-014")
	if len(got) == 0 {
		t.Fatal("expected VL-014 to fire after removing a disposition entry, got none")
	}
	for _, f := range findings {
		if f.Rule != "VL-014" {
			t.Fatalf("unexpected rule storm: %s fired too:\n%s", f.Rule, findingsString(findings))
		}
	}
	foundMissingMsg := false
	for _, f := range got {
		if containsAll(f.Message, "no dispositions") {
			foundMissingMsg = true
		}
	}
	if !foundMissingMsg {
		t.Fatalf("expected a %q-shaped message, got:\n%s", "has no dispositions[] entry", findingsString(got))
	}
}

// TestRun_DanglingDisposition_FailsVL014Exactly models the mirror-image
// violation: a disposition entry naming a sticky id that is NOT a real
// board sticky (as if a hand-edit typo'd or duplicated an id after the
// ritual ran).
func TestRun_DanglingDisposition_FailsVL014Exactly(t *testing.T) {
	repo := buildRepo(t)
	seedBoard(t, repo)
	ctx := context.Background()

	res, err := Run(ctx, Input{Root: repo.Dir, BoardKey: "STORY-1482", SpecName: "vl014-dangling", StoryRef: "jira:LOAN-1482"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	specPath := filepath.Join(repo.Dir, res.SpecRelPath)
	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	edited := appendDanglingDisposition(string(raw))
	if err := os.WriteFile(specPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAll(t, repo.Dir, "test: add a dangling disposition")

	findings := runLintEngine(t, repo.Dir)
	got := findingsForRule(findings, "VL-014")
	if len(got) == 0 {
		t.Fatal("expected VL-014 to fire for a dangling disposition, got none")
	}
	for _, f := range findings {
		if f.Rule != "VL-014" {
			t.Fatalf("unexpected rule storm: %s fired too:\n%s", f.Rule, findingsString(findings))
		}
	}
	foundDanglingMsg := false
	for _, f := range got {
		if containsAll(f.Message, "not a real sticky") {
			foundDanglingMsg = true
		}
	}
	if !foundDanglingMsg {
		t.Fatalf("expected a %q-shaped message, got:\n%s", "which is not a real sticky", findingsString(got))
	}
}

// -- test-local helpers (kept separate from the CLI/lint packages' own
// harness_test.go convention, but doing the same job) --

func runLintEngine(t *testing.T, root string) []lint.Finding {
	t.Helper()
	findings, err := lint.NewEngine().Run(context.Background(), root, lint.Context{}, lint.Options{})
	if err != nil {
		t.Fatalf("lint.Engine.Run: %v", err)
	}
	return findings
}

func findingsForRule(all []lint.Finding, rule string) []lint.Finding {
	var out []lint.Finding
	for _, f := range all {
		if f.Rule == rule {
			out = append(out, f)
		}
	}
	return out
}

func findingsString(findings []lint.Finding) string {
	s := ""
	for _, f := range findings {
		s += f.String() + "\n"
	}
	return s
}

func containsAll(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}
