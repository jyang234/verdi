// AC-1 (spec/instruction-conformance): instruction-file enumeration is
// derived from the filesystem, never a hardcoded literal list — every
// `.claude/skills/*/SKILL.md` (a glob) plus the required repo-root
// `CLAUDE.md`. The repo-root CLAUDE.md is a required minimum: its absence
// is itself a finding, never a silent, vacuously-clean zero-file run. An
// absent or empty `.claude/skills/` directory is a legal, honest
// zero-skills state (this repo may retire its one skill entirely per
// DC-4 — see instructionconformance_test.go's TestInstructionConformance_RepoTreeIsClean).
package specalign

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// instructionFinding is one gate-reported problem: which instruction file
// (an absolute path, so a failure names the exact offending file — AC-4's
// own requirement) and a human-readable detail naming the exact offending
// text. Shared by AC-1 (enumerateInstructionFiles' own missing-CLAUDE.md
// finding), AC-2/AC-3's checks (instructionverbs_test.go,
// instructionritual_test.go), and the combined AC-1+2+3 gate
// (instructionconformance_test.go's checkInstructionConformance).
type instructionFinding struct {
	File   string
	Detail string
}

// enumerateInstructionFiles walks root's `.claude/skills/*/SKILL.md` (a
// filesystem glob — picks up a newly-added skill with no code change) plus
// root's own `CLAUDE.md`, returning every enumerated file's absolute path,
// sorted. The repo-root CLAUDE.md is a required minimum: if absent, no
// file is added for it and a finding is appended instead — this rule must
// never silently vanish into a vacuously-clean zero-file run. An absent or
// empty `.claude/skills/` directory yields zero skill files, no error, and
// no finding: a legal, honest state (filepath.Glob against a nonexistent
// directory returns an empty match list with a nil error, which is
// exactly the behavior this relies on).
func enumerateInstructionFiles(t *testing.T, root string) (files []string, findings []instructionFinding) {
	t.Helper()

	pattern := filepath.Join(root, ".claude", "skills", "*", "SKILL.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("enumerateInstructionFiles: glob %s: %v", pattern, err)
	}
	files = append(files, matches...)

	claudeMD := filepath.Join(root, "CLAUDE.md")
	if _, err := os.Stat(claudeMD); err != nil {
		findings = append(findings, instructionFinding{
			File:   claudeMD,
			Detail: "required repo-root CLAUDE.md is missing (spec/instruction-conformance ac-1: the repo-root CLAUDE.md is a required minimum, never a silent zero-file vacuous pass)",
		})
	} else {
		files = append(files, claudeMD)
	}

	sort.Strings(files)
	return files, findings
}

// writeFixtureSkill creates root/.claude/skills/<name>/SKILL.md with
// trivial, valid content — test infrastructure, not itself part of the
// check under test.
func writeFixtureSkill(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, ".claude", "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir skill %s: %v", name, err)
	}
	content := "---\nname: " + name + "\ndescription: fixture skill.\n---\n\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write SKILL.md for %s: %v", name, err)
	}
}

// TestEnumerateInstructionFiles is AC-1's core, table-driven proof:
// enumerated file identity and count track the filesystem exactly (never
// a hardcoded literal list), across a varying skill count, both boundary
// shapes of "no skills" (an absent dir, and a present-but-empty dir), and
// both presence and required-minimum absence of the repo-root CLAUDE.md.
func TestEnumerateInstructionFiles(t *testing.T) {
	tests := []struct {
		name          string
		skills        []string
		omitSkillsDir bool
		omitClaudeMD  bool
		wantFindings  int
	}{
		{
			name:          "absent skills dir, CLAUDE.md present: legal zero-skills state",
			omitSkillsDir: true,
		},
		{
			name:   "present-but-empty skills dir, CLAUDE.md present: legal zero-skills state",
			skills: nil,
		},
		{
			name:   "one skill, CLAUDE.md present",
			skills: []string{"alpha"},
		},
		{
			name:   "three skills, CLAUDE.md present",
			skills: []string{"alpha", "beta", "gamma"},
		},
		{
			name:          "CLAUDE.md absent is a finding, even with zero skills — never a silent vacuous pass",
			omitSkillsDir: true,
			omitClaudeMD:  true,
			wantFindings:  1,
		},
		{
			name:         "CLAUDE.md absent is a finding, even alongside real skills",
			skills:       []string{"alpha"},
			omitClaudeMD: true,
			wantFindings: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if !tc.omitSkillsDir {
				if err := os.MkdirAll(filepath.Join(root, ".claude", "skills"), 0o755); err != nil {
					t.Fatalf("mkdir .claude/skills: %v", err)
				}
			}
			var wantFiles []string
			for _, name := range tc.skills {
				writeFixtureSkill(t, root, name)
				wantFiles = append(wantFiles, filepath.Join(root, ".claude", "skills", name, "SKILL.md"))
			}
			if !tc.omitClaudeMD {
				if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# fixture\n"), 0o644); err != nil {
					t.Fatalf("write CLAUDE.md: %v", err)
				}
				wantFiles = append(wantFiles, filepath.Join(root, "CLAUDE.md"))
			}
			sort.Strings(wantFiles)

			files, findings := enumerateInstructionFiles(t, root)

			if len(findings) != tc.wantFindings {
				t.Errorf("findings = %d %v, want %d", len(findings), findings, tc.wantFindings)
			}
			if len(files) != len(wantFiles) {
				t.Fatalf("files = %d %v, want %d %v", len(files), files, len(wantFiles), wantFiles)
			}
			for i := range files {
				if files[i] != wantFiles[i] {
					t.Errorf("files[%d] = %q, want %q", i, files[i], wantFiles[i])
				}
			}
		})
	}
}

// TestEnumerateInstructionFiles_SkillAddedBetweenCalls is the
// completeness-proof shape (mirroring internal/showcasealign's own
// TestShowcaseCoverage_EnumerationIsComplete, for a different axis — that
// test parses dispatch.go's own dispatch shape; this one exercises a real
// filesystem glob): a skill directory is added to the SAME root BETWEEN
// two enumeration calls, and the second call's file count grows by
// exactly one with NO edit to this test's own assertion code — proving
// enumeration is derived live from the filesystem, never a hardcoded
// literal list that would silently under-enumerate a newly-added skill.
func TestEnumerateInstructionFiles_SkillAddedBetweenCalls(t *testing.T) {
	root := t.TempDir()
	writeFixtureSkill(t, root, "first")
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# fixture\n"), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}

	var before, after []string
	t.Run("before_add", func(t *testing.T) {
		before, _ = enumerateInstructionFiles(t, root)
	})

	writeFixtureSkill(t, root, "second") // the mutation, between the two calls

	t.Run("after_add", func(t *testing.T) {
		after, _ = enumerateInstructionFiles(t, root)
	})

	if len(after) != len(before)+1 {
		t.Fatalf("enumerated file count after adding a skill = %d %v, want %d (before=%d + 1) — enumeration must grow with the filesystem, not a hardcoded list", len(after), after, len(before)+1, len(before))
	}
}
