package lint

import (
	"path/filepath"
	"testing"
)

// TestVL010_FrozenFileModified layers testdata/violations/VL-010/before/
// then /after/ as two successive commits atop the corpus+setup base, sets
// DiffBase to the "before" commit, and asserts VL-010 fires on the
// modified frozen ADR.
func TestVL010_FrozenFileModified(t *testing.T) {
	repo := buildLintRepo(t,
		filepath.Join(violationsDir, "VL-010", "before"),
		filepath.Join(violationsDir, "VL-010", "after"),
	)
	diffBase := repo.Heads[len(repo.Heads)-2] // the "before" layer's commit
	findings := runLint(t, repo.Dir, Context{DiffBase: diffBase}, Options{})
	onlyRule(t, findings, "VL-010")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	if findings[0].Path != ".verdi/adr/vl-010-frozen.md" {
		t.Fatalf("finding path = %q, want .verdi/adr/vl-010-frozen.md", findings[0].Path)
	}
}

// TestVL010_NoDiffBase_Silent proves the "can't prove it" posture: with no
// DiffBase established, VL-010 does not guess.
func TestVL010_NoDiffBase_Silent(t *testing.T) {
	repo := buildLintRepo(t,
		filepath.Join(violationsDir, "VL-010", "before"),
		filepath.Join(violationsDir, "VL-010", "after"),
	)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-010" {
			t.Fatalf("VL-010 fired with no DiffBase: %s", f.String())
		}
	}
}

// TestVL010_PureActiveArchiveRenameAllowed proves the one legal diff shape
// on a frozen file: a pure rename moving a spec directory from
// specs/active/ to specs/archive/, content unchanged.
func TestVL010_PureActiveArchiveRenameAllowed(t *testing.T) {
	const specBody = `---
id: spec/vl-010-archive-move
kind: spec
class: feature
title: "VL-010: legal archive move"
status: closed
owners: [platform-team]
story: jira:LOAN-0012
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
frozen: { at: 2026-05-14, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# VL-010: legal archive move
`
	beforeDir := adHocOverlayDir(t, ".verdi/specs/active/vl-010-archive-move/spec.md", specBody)
	repo := buildLintRepo(t, beforeDir)
	beforeCommit := repo.Heads[len(repo.Heads)-1]

	// A second commit performs the move: remove the active/ copy, add the
	// identical content at archive/ (git's own rename detection, exercised
	// by DiffNameStatus, does not depend on how the change was staged).
	activePath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "vl-010-archive-move", "spec.md")
	archivePath := filepath.Join(repo.Dir, ".verdi", "specs", "archive", "vl-010-archive-move", "spec.md")
	mustRemove(t, activePath)
	writeTestFile(t, archivePath, specBody)
	commitAll(t, repo.Dir, "archive move")

	findings := runLint(t, repo.Dir, Context{DiffBase: beforeCommit}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-010" {
			t.Fatalf("VL-010 fired on a pure active->archive rename: %s", f.String())
		}
	}
}
