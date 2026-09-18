// AC-1 (spec/instruction-conformance): instruction-file enumeration is
// derived from the filesystem, never a hardcoded literal list — every
// `.claude/skills/*/SKILL.md` and every `.agents/skills/*/SKILL.md` (two
// globs, one per skill host: Claude Code and Codex, SI-201) plus the
// required repo-root `CLAUDE.md`. The repo-root CLAUDE.md is a required
// minimum: its absence is itself a finding, never a silent,
// vacuously-clean zero-file run. An absent or empty skills directory
// under either host is a legal, honest zero-skills state (this repo may
// retire its skills entirely per DC-4 — see instructionconformance_test.go's
// TestInstructionConformance_RepoTreeIsClean).
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

// instructionSkillHostDirs are the per-host skill roots an instruction
// file can live under: Claude Code's `.claude/skills` and Codex's
// `.agents/skills` (SI-201). Both are enumerated, because R-W3-8 commits
// BOTH rendered copies to this repository and makes both subject to
// spec/instruction-conformance's enumeration; globbing only `.claude`
// left the four `.agents/skills/verdi-*/SKILL.md` files outside the gate
// (final-review F1).
var instructionSkillHostDirs = []string{".agents", ".claude"}

// enumerateInstructionFiles walks root's `<hostDir>/skills/*/SKILL.md`
// for every instructionSkillHostDirs entry (filesystem globs — they pick
// up a newly-added skill, under either host, with no code change) plus
// root's own `CLAUDE.md`, returning every enumerated file's absolute path,
// sorted. The repo-root CLAUDE.md is a required minimum: if absent, no
// file is added for it and a finding is appended instead — this rule must
// never silently vanish into a vacuously-clean zero-file run. An absent or
// empty skills directory under either host yields zero skill files, no
// error, and no finding: a legal, honest state (filepath.Glob against a
// nonexistent directory returns an empty match list with a nil error,
// which is exactly the behavior this relies on).
func enumerateInstructionFiles(t *testing.T, root string) (files []string, findings []instructionFinding) {
	t.Helper()

	for _, hostDir := range instructionSkillHostDirs {
		pattern := filepath.Join(root, hostDir, "skills", "*", "SKILL.md")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("enumerateInstructionFiles: glob %s: %v", pattern, err)
		}
		files = append(files, matches...)
	}

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

// writeFixtureSkill creates root/<hostDir>/skills/<name>/SKILL.md with
// trivial, valid content — test infrastructure, not itself part of the
// check under test. hostDir is one of instructionSkillHostDirs.
func writeFixtureSkill(t *testing.T, root, hostDir, name string) {
	t.Helper()
	dir := filepath.Join(root, hostDir, "skills", name)
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
		claudeSkills  []string
		agentSkills   []string
		omitSkillsDir bool
		omitClaudeMD  bool
		wantFindings  int
	}{
		{
			name:          "absent skills dirs, CLAUDE.md present: legal zero-skills state",
			omitSkillsDir: true,
		},
		{
			name: "present-but-empty skills dirs, CLAUDE.md present: legal zero-skills state",
		},
		{
			name:         "one Claude Code skill, CLAUDE.md present",
			claudeSkills: []string{"alpha"},
		},
		{
			name:        "one Codex skill and no Claude Code skill, CLAUDE.md present",
			agentSkills: []string{"alpha"},
		},
		{
			name:         "three Claude Code skills, CLAUDE.md present",
			claudeSkills: []string{"alpha", "beta", "gamma"},
		},
		{
			name:         "both hosts' skills are enumerated, including a same-named pair",
			claudeSkills: []string{"alpha", "beta"},
			agentSkills:  []string{"alpha", "gamma"},
		},
		{
			name:          "CLAUDE.md absent is a finding, even with zero skills — never a silent vacuous pass",
			omitSkillsDir: true,
			omitClaudeMD:  true,
			wantFindings:  1,
		},
		{
			name:         "CLAUDE.md absent is a finding, even alongside real skills",
			claudeSkills: []string{"alpha"},
			agentSkills:  []string{"alpha"},
			omitClaudeMD: true,
			wantFindings: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if !tc.omitSkillsDir {
				for _, hostDir := range instructionSkillHostDirs {
					if err := os.MkdirAll(filepath.Join(root, hostDir, "skills"), 0o755); err != nil {
						t.Fatalf("mkdir %s/skills: %v", hostDir, err)
					}
				}
			}
			var wantFiles []string
			for hostDir, skills := range map[string][]string{".claude": tc.claudeSkills, ".agents": tc.agentSkills} {
				for _, name := range skills {
					writeFixtureSkill(t, root, hostDir, name)
					wantFiles = append(wantFiles, filepath.Join(root, hostDir, "skills", name, "SKILL.md"))
				}
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
	for _, hostDir := range instructionSkillHostDirs {
		t.Run(hostDir, func(t *testing.T) {
			root := t.TempDir()
			writeFixtureSkill(t, root, hostDir, "first")
			if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# fixture\n"), 0o644); err != nil {
				t.Fatalf("write CLAUDE.md: %v", err)
			}

			var before, after []string
			t.Run("before_add", func(t *testing.T) {
				before, _ = enumerateInstructionFiles(t, root)
			})

			writeFixtureSkill(t, root, hostDir, "second") // the mutation, between the two calls

			t.Run("after_add", func(t *testing.T) {
				after, _ = enumerateInstructionFiles(t, root)
			})

			if len(after) != len(before)+1 {
				t.Fatalf("enumerated file count after adding a %s skill = %d %v, want %d (before=%d + 1) — enumeration must grow with the filesystem, not a hardcoded list", hostDir, len(after), after, len(before)+1, len(before))
			}
		})
	}
}

// TestEnumerateInstructionFiles_RepoTreeCoversBothHosts is R-W3-8's own
// non-vacuity witness against THIS repository's real tree: every skill
// directory actually present under either host root — discovered with
// os.ReadDir, independently of enumerateInstructionFiles' own globs — is
// among the enumerated instruction files. Before final-review F1 the
// Codex half (`.agents/skills/verdi-*/SKILL.md`, four committed files)
// was silently outside the enumeration while the plan and R-W3-8 said it
// was inside; dropping either host root from the glob again fails here.
//
// A repository with no skills at all under a host root passes vacuously,
// which is the legal zero-skills state AC-1 protects (DC-4) — this test
// asserts coverage of what exists, never that anything must exist.
//
// The two host roots are spelled out here rather than read from
// instructionSkillHostDirs on purpose: that variable is the enumeration's
// OWN derivation, so comparing it against itself would let the very
// regression this test exists to catch (a host root dropped from the
// list) pass vacuously. SI-201 names these two directories; this literal
// is the contract side of the comparison.
func TestEnumerateInstructionFiles_RepoTreeCoversBothHosts(t *testing.T) {
	files, _ := enumerateInstructionFiles(t, verdiRepoRoot)
	enumerated := map[string]bool{}
	for _, f := range files {
		enumerated[f] = true
	}
	for _, hostDir := range []string{".claude", ".agents"} {
		skillsDir := filepath.Join(verdiRepoRoot, hostDir, "skills")
		entries, err := os.ReadDir(skillsDir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatalf("reading %s: %v", skillsDir, err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			skillFile := filepath.Join(skillsDir, e.Name(), "SKILL.md")
			if _, err := os.Stat(skillFile); err != nil {
				continue
			}
			if !enumerated[skillFile] {
				t.Errorf("%s exists but is not an enumerated instruction file (spec/instruction-conformance ac-1, R-W3-8: this repository's rendered skills for BOTH hosts are enumerated instruction files)", skillFile)
			}
		}
	}
}
