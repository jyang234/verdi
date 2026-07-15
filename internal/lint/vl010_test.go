package lint

import (
	"path/filepath"
	"strings"
	"testing"
)

// vl010SupersededBaseSpec is a frozen, accepted-pending-build feature spec
// used by the round-5 status-only-flip exception tests (D-12). Feature class
// (grandfathered — no problem/outcome required) keeps the fixture minimal so
// no unrelated rule fires alongside VL-010.
const vl010SupersededBaseSpec = `---
id: spec/vl-010-superseded
kind: spec
class: feature
title: "VL-010: superseded flip"
status: accepted-pending-build
owners: [platform-team]
story: jira:LOAN-0013
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
frozen: { at: 2026-05-14, commit: 2f230011b192c5ac1c0ed5442be76fc401c4cbca }
---
# VL-010: superseded flip
`

// TestVL010_StatusOnlySupersededFlipAllowed proves round-5's second legal
// diff shape on a frozen file (D-12): a status-line-only edit flipping the
// spec to `superseded` (the accept ritual's predecessor flip), with nothing
// else changed.
func TestVL010_StatusOnlySupersededFlipAllowed(t *testing.T) {
	beforeDir := adHocOverlayDir(t, ".verdi/specs/active/vl-010-superseded/spec.md", vl010SupersededBaseSpec)
	repo := buildLintRepo(t, beforeDir)
	beforeCommit := repo.Heads[len(repo.Heads)-1]

	after := strings.Replace(vl010SupersededBaseSpec, "status: accepted-pending-build", "status: superseded", 1)
	specPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "vl-010-superseded", "spec.md")
	writeTestFile(t, specPath, after)
	commitAll(t, repo.Dir, "supersede flip")

	findings := runLint(t, repo.Dir, Context{DiffBase: beforeCommit}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-010" {
			t.Fatalf("VL-010 fired on a status-only superseded flip: %s", f.String())
		}
	}
}

// TestVL010_SupersededFlipWithOtherEditStillFails proves the exception is
// strictly status-line-only: a diff that flips to superseded AND edits any
// other line is still an illegal frozen modification.
func TestVL010_SupersededFlipWithOtherEditStillFails(t *testing.T) {
	beforeDir := adHocOverlayDir(t, ".verdi/specs/active/vl-010-superseded/spec.md", vl010SupersededBaseSpec)
	repo := buildLintRepo(t, beforeDir)
	beforeCommit := repo.Heads[len(repo.Heads)-1]

	after := strings.Replace(vl010SupersededBaseSpec, "status: accepted-pending-build", "status: superseded", 1)
	after = strings.Replace(after, `title: "VL-010: superseded flip"`, `title: "VL-010: superseded flip EDITED"`, 1)
	specPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "vl-010-superseded", "spec.md")
	writeTestFile(t, specPath, after)
	commitAll(t, repo.Dir, "supersede flip plus edit")

	findings := runLint(t, repo.Dir, Context{DiffBase: beforeCommit}, Options{})
	var sawVL010 bool
	for _, f := range findings {
		if f.Rule == "VL-010" {
			sawVL010 = true
		}
	}
	if !sawVL010 {
		t.Fatalf("VL-010 did not fire on a frozen file edited beyond its status line:\n%s", findingsString(findings))
	}
}

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

// vl010FeaturePredecessorBaseSpec mirrors vl010SupersededBaseSpec exactly in
// shape (frozen, accepted-pending-build, class: feature) but is named and
// scoped explicitly for round 6's ac-1 (feature-supersession-state): this
// rule is class-agnostic (it diffs raw frontmatter lines and never inspects
// `class:`), so the SAME exception TestVL010_StatusOnlySupersededFlipAllowed
// already proves for the rung-3 story flip also, unmodified, admits ac-1's
// feature-predecessor flip (accept.go's flipPredecessorToSuperseded, shared
// by both call sites). This fixture/test pair exists to make that
// class-agnostic coverage explicit and traceable to ac-1, not because VL-010
// needed any change.
const vl010FeaturePredecessorBaseSpec = `---
id: spec/vl-010-feature-predecessor
kind: spec
class: feature
title: "VL-010: feature predecessor superseded flip"
status: accepted-pending-build
owners: [platform-team]
story: jira:LOAN-0015
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
frozen: { at: 2026-05-14, commit: 2f230011b192c5ac1c0ed5442be76fc401c4cbca }
---
# VL-010: feature predecessor superseded flip
`

// TestVL010_FeaturePredecessorSupersededFlipAllowed proves ac-1's
// (feature-supersession-state) own diff shape — accept.go's
// flipPredecessorToSuperseded status-only-flipping a FEATURE-class
// predecessor to `superseded` — is admitted by the SAME D-12 exception
// TestVL010_StatusOnlySupersededFlipAllowed proves for a story predecessor:
// VL-010 never inspects `class:`, so no rule change was needed here, only
// this explicit proof.
func TestVL010_FeaturePredecessorSupersededFlipAllowed(t *testing.T) {
	beforeDir := adHocOverlayDir(t, ".verdi/specs/active/vl-010-feature-predecessor/spec.md", vl010FeaturePredecessorBaseSpec)
	repo := buildLintRepo(t, beforeDir)
	beforeCommit := repo.Heads[len(repo.Heads)-1]

	after := strings.Replace(vl010FeaturePredecessorBaseSpec, "status: accepted-pending-build", "status: superseded", 1)
	specPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "vl-010-feature-predecessor", "spec.md")
	writeTestFile(t, specPath, after)
	commitAll(t, repo.Dir, "supersede feature predecessor flip (ac-1)")

	findings := runLint(t, repo.Dir, Context{DiffBase: beforeCommit}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-010" {
			t.Fatalf("VL-010 fired on a status-only feature-predecessor superseded flip (ac-1): %s", f.String())
		}
	}
}

// vl010ClosedMoveBaseSpec is a frozen, accepted-pending-build spec used by
// round-6's closed-flip-within-archive-move exception tests (D6-11). Feature
// class (grandfathered — no problem/outcome required) keeps the fixture
// minimal so no unrelated rule fires alongside VL-010.
const vl010ClosedMoveBaseSpec = `---
id: spec/vl-010-closed-move
kind: spec
class: feature
title: "VL-010: closed archive move"
status: accepted-pending-build
owners: [platform-team]
story: jira:LOAN-0014
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
frozen: { at: 2026-05-14, commit: 2f230011b192c5ac1c0ed5442be76fc401c4cbca }
---
# VL-010: closed archive move
`

// TestVL010_StatusOnlyClosedFlipWithinArchiveMoveAllowed proves round-6's
// closed-flip exception (D6-11): a spec.md moving specs/active→specs/archive
// while its status line flips accepted-pending-build→closed and NOTHING else
// changes is legal on an otherwise-frozen spec — exactly what `verdi close`
// now produces. The move is no longer the byte-identical R100 rename the
// pure-rename exception covers, so this second, narrower exception is what
// admits it.
func TestVL010_StatusOnlyClosedFlipWithinArchiveMoveAllowed(t *testing.T) {
	beforeDir := adHocOverlayDir(t, ".verdi/specs/active/vl-010-closed-move/spec.md", vl010ClosedMoveBaseSpec)
	repo := buildLintRepo(t, beforeDir)
	beforeCommit := repo.Heads[len(repo.Heads)-1]

	// The archive move flips the status line accepted-pending-build→closed as
	// part of the move (remove the active/ copy, add the flipped content at
	// archive/ — git's own rename detection pairs them regardless of staging).
	after := strings.Replace(vl010ClosedMoveBaseSpec, "status: accepted-pending-build", "status: closed", 1)
	activePath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "vl-010-closed-move", "spec.md")
	archivePath := filepath.Join(repo.Dir, ".verdi", "specs", "archive", "vl-010-closed-move", "spec.md")
	mustRemove(t, activePath)
	writeTestFile(t, archivePath, after)
	commitAll(t, repo.Dir, "close: archive vl-010-closed-move (status apb->closed)")

	findings := runLint(t, repo.Dir, Context{DiffBase: beforeCommit}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-010" {
			t.Fatalf("VL-010 fired on a status-only apb->closed flip within an active->archive move: %s", f.String())
		}
	}
}

// TestVL010_ArchiveMoveWithNonStatusEditStillFails proves the closed-flip
// exception is strictly status-line-only: an active→archive move that flips
// apb→closed AND edits any other line is still an illegal frozen mutation.
func TestVL010_ArchiveMoveWithNonStatusEditStillFails(t *testing.T) {
	beforeDir := adHocOverlayDir(t, ".verdi/specs/active/vl-010-closed-move/spec.md", vl010ClosedMoveBaseSpec)
	repo := buildLintRepo(t, beforeDir)
	beforeCommit := repo.Heads[len(repo.Heads)-1]

	after := strings.Replace(vl010ClosedMoveBaseSpec, "status: accepted-pending-build", "status: closed", 1)
	after = strings.Replace(after, `title: "VL-010: closed archive move"`, `title: "VL-010: closed archive move EDITED"`, 1)
	activePath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "vl-010-closed-move", "spec.md")
	archivePath := filepath.Join(repo.Dir, ".verdi", "specs", "archive", "vl-010-closed-move", "spec.md")
	mustRemove(t, activePath)
	writeTestFile(t, archivePath, after)
	commitAll(t, repo.Dir, "close: archive move plus an illegal extra edit")

	findings := runLint(t, repo.Dir, Context{DiffBase: beforeCommit}, Options{})
	var sawVL010 bool
	for _, f := range findings {
		if f.Rule == "VL-010" {
			sawVL010 = true
		}
	}
	if !sawVL010 {
		t.Fatalf("VL-010 did not fire on an archive move that edited a frozen spec beyond its status line:\n%s", findingsString(findings))
	}
}

// TestVL010_ArchiveMoveFlipToNonClosedStatusStillFails proves the archive-move
// exception admits ONLY accepted-pending-build→closed: a status-only flip to
// any other terminal status (here superseded) within an active→archive move is
// still rejected — closure is the sole status that belongs under specs/archive/.
func TestVL010_ArchiveMoveFlipToNonClosedStatusStillFails(t *testing.T) {
	beforeDir := adHocOverlayDir(t, ".verdi/specs/active/vl-010-closed-move/spec.md", vl010ClosedMoveBaseSpec)
	repo := buildLintRepo(t, beforeDir)
	beforeCommit := repo.Heads[len(repo.Heads)-1]

	after := strings.Replace(vl010ClosedMoveBaseSpec, "status: accepted-pending-build", "status: superseded", 1)
	activePath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "vl-010-closed-move", "spec.md")
	archivePath := filepath.Join(repo.Dir, ".verdi", "specs", "archive", "vl-010-closed-move", "spec.md")
	mustRemove(t, activePath)
	writeTestFile(t, archivePath, after)
	commitAll(t, repo.Dir, "close: archive move flipping to the wrong terminal status")

	findings := runLint(t, repo.Dir, Context{DiffBase: beforeCommit}, Options{})
	var sawVL010 bool
	for _, f := range findings {
		if f.Rule == "VL-010" {
			sawVL010 = true
		}
	}
	if !sawVL010 {
		t.Fatalf("VL-010 did not fire on an archive move flipping to a non-closed status:\n%s", findingsString(findings))
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
frozen: { at: 2026-05-14, commit: 2f230011b192c5ac1c0ed5442be76fc401c4cbca }
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
