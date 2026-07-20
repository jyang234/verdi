// spec/instruction-conformance's own combined outcome: AC-1's enumeration,
// AC-2's verb validation, and AC-3's retired-ritual tripwire, run
// TOGETHER against a root directory. checkInstructionConformance is the
// one function AC-4's fixture proof and AC-5's real-tree proof both drive
// end to end — neither ever stands a synthetic assertion in for actually
// running the real, combined check.
//
// AC-4 (this story's own "the gate must BITE" requirement, mirroring the
// Makefile's lint-showcase/showcase-coverage GUARD rationale): a
// committed, RED fixture instruction file
// (testdata/instructionconformance/red/) carries both an unrecognized
// verb reference (inside a FENCED code block, proving AC-2's fenced-block
// extraction really participates end to end, not just inline spans) and
// an undisclosed retired-ritual phrase — driving the real check against
// it fails, naming the exact fixture file and the exact offending
// verb/phrase. A second, committed CLEAN fixture
// (testdata/instructionconformance/clean/) carries only real verbs and a
// `verdi board commit` mention PAIRED with a retirement disclosure in the
// same file — driving the real check against it passes with zero
// findings. Neither test is a `go test -run` invocation standing in for
// running the underlying gate: spec-align's own Makefile target is
// already a bare, unfiltered `go test ./internal/specalign/...` (no -run
// pattern at all), so the vacuous-pass-by-renamed-or-deleted-function
// class ADJ-47/ADJ-50 found once for docsync_test.go cannot recur here by
// construction — every test in this file is an ordinary, unconditional Go
// test function, no `-run` filter, no build tag, no t.Skip (AC-5's static
// half).
//
// AC-5: run against THIS repo's own real .claude/skills/*/SKILL.md and
// root CLAUDE.md — never a fixture — the combined check reports zero
// findings. This is expected (and, before this story's build phase
// applies DC-4's disposition, DOES) fail red against this repo's
// pre-build tree, naming .claude/skills/commit-to-design/SKILL.md's
// undisclosed `verdi board commit` teaching — spec/instruction-
// conformance's own outcome text: "the gate fails ... which is the
// point".
package specalign

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// checkInstructionConformance runs AC-1's enumeration, AC-2's verb
// validation, and AC-3's retired-ritual tripwire together against root,
// and returns every finding — a missing required CLAUDE.md, an
// unrecognized `verdi <verb>` reference, or an undisclosed retired-ritual
// phrase — in deterministic (file, then detail) order. Verb references
// are de-duplicated per (file, verb) before classification, so a verb
// mentioned N times in one file produces at most one finding for it, not
// N identical ones.
func checkInstructionConformance(t *testing.T, root string) []instructionFinding {
	t.Helper()

	files, findings := enumerateInstructionFiles(t, root)
	banner := unknownVerbBanner(t)

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("checkInstructionConformance: reading %s: %v", f, err)
		}
		content := string(data)

		seen := map[string]bool{}
		for _, verb := range extractVerdiVerbRefs(content) {
			if seen[verb] {
				continue
			}
			seen[verb] = true
			if !classifyVerb(t, verb, banner) {
				findings = append(findings, instructionFinding{
					File:   f,
					Detail: fmt.Sprintf("references unrecognized verb %q (invocation: verdi %s) — dispatch.go does not recognize this verb (spec/instruction-conformance ac-2)", verb, verb),
				})
			}
		}

		if finding := checkRetiredRitualTripwire(f, content); finding != nil {
			findings = append(findings, *finding)
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Detail < findings[j].Detail
	})
	return findings
}

// formatFindings renders findings for a test failure message: one line
// per finding, naming the exact file and detail (AC-4's own "never a bare
// boolean" requirement).
func formatFindings(findings []instructionFinding) string {
	var b strings.Builder
	for _, f := range findings {
		fmt.Fprintf(&b, "  %s: %s\n", f.File, f.Detail)
	}
	return b.String()
}

// TestInstructionConformance_RedFixtureFails is AC-4's dirty-fixture
// proof.
func TestInstructionConformance_RedFixtureFails(t *testing.T) {
	root := filepath.Join(verdiRepoRoot, "internal", "specalign", "testdata", "instructionconformance", "red")
	findings := checkInstructionConformance(t, root)

	skillPath := filepath.Join(root, ".claude", "skills", "stale-skill", "SKILL.md")

	var sawUnknownVerb, sawRitual bool
	for _, f := range findings {
		if f.File != skillPath {
			t.Errorf("finding for unexpected file %s (want only %s): %s", f.File, skillPath, f.Detail)
			continue
		}
		if strings.Contains(f.Detail, `"frobnicate"`) {
			sawUnknownVerb = true
		}
		if strings.Contains(f.Detail, "verdi board commit") {
			sawRitual = true
		}
	}
	if !sawUnknownVerb {
		t.Errorf("red fixture check did not name the unrecognized verb %q — findings:\n%s", "frobnicate", formatFindings(findings))
	}
	if !sawRitual {
		t.Errorf("red fixture check did not name the undisclosed ritual phrase — findings:\n%s", formatFindings(findings))
	}
	if len(findings) != 2 {
		t.Errorf("red fixture: got %d finding(s), want exactly 2 (one unknown-verb, one ritual):\n%s", len(findings), formatFindings(findings))
	}
}

// TestInstructionConformance_CleanFixturePasses is AC-4's clean-fixture
// proof: it must NOT false-positive on legitimate content that discusses
// the retired ritual honestly.
func TestInstructionConformance_CleanFixturePasses(t *testing.T) {
	root := filepath.Join(verdiRepoRoot, "internal", "specalign", "testdata", "instructionconformance", "clean")
	findings := checkInstructionConformance(t, root)
	if len(findings) != 0 {
		t.Errorf("clean fixture: got %d finding(s), want 0:\n%s", len(findings), formatFindings(findings))
	}
}

// TestInstructionConformance_RepoTreeIsClean is AC-5's proof.
func TestInstructionConformance_RepoTreeIsClean(t *testing.T) {
	findings := checkInstructionConformance(t, verdiRepoRoot)
	if len(findings) != 0 {
		t.Errorf("instruction-conformance found %d finding(s) against this repo's own .claude/skills/*/SKILL.md and root CLAUDE.md (spec/instruction-conformance ac-5) — want zero:\n%s", len(findings), formatFindings(findings))
	}
}
