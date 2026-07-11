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

// TestVL010_FrozenFileDeleted proves 02's letter "ANY diff touching a
// frozen file fails" covers DELETION: the file is gone from HEAD, so
// frozen-ness cannot be read there — it is evaluated on the base side,
// where the `frozen:` stamp is still present. Layers the frozen ADR as a
// commit, then deletes it in a second commit and diffs against the first.
func TestVL010_FrozenFileDeleted(t *testing.T) {
	repo := buildLintRepo(t,
		filepath.Join(violationsDir, "VL-010", "deletion"),
	)
	beforeCommit := repo.Heads[len(repo.Heads)-1] // the overlay layer's commit

	// Delete the frozen ADR in a follow-up commit (a real `git rm`, modeled
	// as remove + stage-all, per the harness's rename convention).
	adrPath := filepath.Join(repo.Dir, ".verdi", "adr", "vl-010-frozen-deleted.md")
	mustRemove(t, adrPath)
	commitAll(t, repo.Dir, "delete frozen adr")

	findings := runLint(t, repo.Dir, Context{DiffBase: beforeCommit}, Options{})
	onlyRule(t, findings, "VL-010")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	if findings[0].Path != ".verdi/adr/vl-010-frozen-deleted.md" {
		t.Fatalf("finding path = %q, want .verdi/adr/vl-010-frozen-deleted.md", findings[0].Path)
	}
}

// TestVL010_FrozenStampStrippedAndEdited proves an edit that also strips the
// `frozen:` stamp does not escape: HEAD-side frozen-ness is false (the stamp
// is gone and the ADR is downgraded to a valid, un-frozen `proposed`), but
// the rule reads the BASE side, where the stamp still stands, so the
// modification is caught. onlyRule guards that no OTHER rule fires — the
// stripped HEAD document is deliberately kept schema-clean.
func TestVL010_FrozenStampStrippedAndEdited(t *testing.T) {
	repo := buildLintRepo(t,
		filepath.Join(violationsDir, "VL-010", "stamp-strip", "before"),
		filepath.Join(violationsDir, "VL-010", "stamp-strip", "after"),
	)
	diffBase := repo.Heads[len(repo.Heads)-2] // the "before" (frozen) layer
	findings := runLint(t, repo.Dir, Context{DiffBase: diffBase}, Options{})
	onlyRule(t, findings, "VL-010")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	if findings[0].Path != ".verdi/adr/vl-010-frozen-stripped.md" {
		t.Fatalf("finding path = %q, want .verdi/adr/vl-010-frozen-stripped.md", findings[0].Path)
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
