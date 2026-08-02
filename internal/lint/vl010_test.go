package lint

import (
	"path/filepath"
	"strings"
	"testing"
)

// vl010SupersededBaseSpec is a frozen, accepted-pending-build feature spec
// used by TestVL010_StatusOnlySupersededFlipRefused (the round-5 D-12
// exception's removal proof, Task 7). Feature class (grandfathered — no
// problem/outcome required) keeps the fixture minimal so no unrelated rule
// fires alongside VL-010.
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
frozen: { at: 2026-05-14, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# VL-010: superseded flip
`

// TestVL010_StatusOnlySupersededFlipRefused proves round-5's D-12
// status-only superseded-flip exception was deliberately REMOVED once its
// sole production writer (cmd/verdi's old accept ritual, cmd/verdi's now-
// deleted supersede.go) was retired (Task 7, docs/superpowers/specs/
// 2026-08-01-merge-signals-spec-acceptance-design.md): supersession is now
// derived entirely from Git reachability (internal/specstate) — a
// predecessor's own frozen bytes are NEVER mutated, not even a
// status-only edit to `superseded`. VL-010 now refuses this shape exactly
// like any other frozen-file modification, for both a story-class and a
// feature-class predecessor (VL-010 is class-agnostic, never inspecting
// `class:`).
func TestVL010_StatusOnlySupersededFlipRefused(t *testing.T) {
	cases := []struct {
		name     string
		specName string
		baseMD   string
	}{
		{"story predecessor", "vl-010-superseded", vl010SupersededBaseSpec},
		{"feature predecessor", "vl-010-feature-predecessor", vl010FeaturePredecessorBaseSpec},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			beforeDir := adHocOverlayDir(t, ".verdi/specs/active/"+tc.specName+"/spec.md", tc.baseMD)
			repo := buildLintRepo(t, beforeDir)
			beforeCommit := repo.Heads[len(repo.Heads)-1]

			after := strings.Replace(tc.baseMD, "status: accepted-pending-build", "status: superseded", 1)
			specPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", tc.specName, "spec.md")
			writeTestFile(t, specPath, after)
			commitAll(t, repo.Dir, "supersede flip")

			findings := runLint(t, repo.Dir, Context{DiffBase: beforeCommit}, Options{})
			var sawVL010 bool
			for _, f := range findings {
				if f.Rule == "VL-010" {
					sawVL010 = true
				}
			}
			if !sawVL010 {
				t.Fatalf("VL-010 did not fire on a status-only superseded flip — the D-12 exception must be gone: %s", findingsString(findings))
			}
		})
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
// `class:`), so TestVL010_StatusOnlySupersededFlipRefused's table above
// proves the SAME refusal for both a story and a feature predecessor.
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
frozen: { at: 2026-05-14, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# VL-010: feature predecessor superseded flip
`

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
frozen: { at: 2026-05-14, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
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

// vl010NestedProbeADR is a minimal frozen ADR used by
// TestVL010_OnlyRootStoreArtifactsSwept to prove VL-010's sweep is scoped to
// the root store's own .verdi/ tree: the SAME frozen-then-modified diff shape
// fires when the file lives at the root store's `.verdi/adr/...` but is
// ignored when it lives inside a nested/fixture store (`<dir>/.verdi/adr/...`,
// e.g. examples/showcase's committed fixture store or a testdata/violations
// overlay), whose frozen-stamped files are fixtures, not this store's
// artifacts.
const vl010NestedProbeADR = `---
id: adr/vl-010-nested-probe
kind: adr
title: "VL-010 nested-store probe"
status: accepted
owners: [platform-team]
decided: 2026-05-14
frozen: { at: 2026-05-14, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# VL-010 nested-store probe
`

// TestVL010_OnlyRootStoreArtifactsSwept proves VL-010 governs only the root
// store's own frozen artifacts (paths under the root `.verdi/`) — exactly the
// tree walk.go's walkDocuments walks — and never sweeps frozen-stamped files
// inside a nested store or fixture overlay reached only by the whole-repo git
// diff. Table-driven: the identical modify-a-frozen-file diff is a VL-010
// violation at the root store path (happy path) and a no-op at a nested path
// (negative path).
func TestVL010_OnlyRootStoreArtifactsSwept(t *testing.T) {
	for _, tc := range []struct {
		name      string
		relPath   string
		wantVL010 bool
	}{
		{
			name:      "root store frozen file modified is swept",
			relPath:   ".verdi/adr/vl-010-nested-probe.md",
			wantVL010: true,
		},
		{
			name:      "nested store frozen file modified is not swept",
			relPath:   "vendored/nested-store/.verdi/adr/vl-010-nested-probe.md",
			wantVL010: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			overlay := adHocOverlayDir(t, tc.relPath, vl010NestedProbeADR)
			repo := buildLintRepo(t, overlay)
			beforeCommit := repo.Heads[len(repo.Heads)-1]

			after := strings.Replace(vl010NestedProbeADR,
				`title: "VL-010 nested-store probe"`,
				`title: "VL-010 nested-store probe EDITED"`, 1)
			probePath := filepath.Join(repo.Dir, filepath.FromSlash(tc.relPath))
			writeTestFile(t, probePath, after)
			commitAll(t, repo.Dir, "modify frozen probe adr")

			findings := runLint(t, repo.Dir, Context{DiffBase: beforeCommit}, Options{})
			var sawVL010 bool
			for _, f := range findings {
				if f.Rule == "VL-010" {
					sawVL010 = true
				}
			}
			if sawVL010 != tc.wantVL010 {
				t.Fatalf("VL-010 fired = %v, want %v for a modified frozen file at %s:\n%s",
					sawVL010, tc.wantVL010, tc.relPath, findingsString(findings))
			}
		})
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

// vl010UnstampedFeatureSpec is a complete, valid, STATUSLESS feature spec
// with no `frozen:` stamp at all — the shape a merge-signaled proposal now
// has once it lands on the default branch (the design's "Artifact
// compatibility": "new merge-accepted artifacts ... do not require a
// content-changing frozen stamp"). baseProtected's second signal (Task 4's
// immutability keystone) must protect it anyway: any strict-decoded
// feature/story spec at the diff base is protected, stamped or not.
const vl010UnstampedFeatureSpec = `---
id: spec/vl-010-unstamped-feature
kind: spec
class: feature
title: "VL-010: unstamped accepted feature"
owners: [platform-team]
story: jira:LOAN-0020
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
---
# VL-010: unstamped accepted feature
`

// TestVL010_UnstampedFeatureModifiedAtBase_Fails proves the keystone: a
// feature spec at the diff base with NO frozen stamp and NO persisted
// status is still protected — modifying it fails VL-010 exactly like a
// legacy frozen-stamped spec would.
func TestVL010_UnstampedFeatureModifiedAtBase_Fails(t *testing.T) {
	beforeDir := adHocOverlayDir(t, ".verdi/specs/active/vl-010-unstamped-feature/spec.md", vl010UnstampedFeatureSpec)
	repo := buildLintRepo(t, beforeDir)
	beforeCommit := repo.Heads[len(repo.Heads)-1]

	after := strings.Replace(vl010UnstampedFeatureSpec, `title: "VL-010: unstamped accepted feature"`, `title: "VL-010: unstamped accepted feature EDITED"`, 1)
	specPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "vl-010-unstamped-feature", "spec.md")
	writeTestFile(t, specPath, after)
	commitAll(t, repo.Dir, "edit unstamped feature at the diff base")

	findings := runLint(t, repo.Dir, Context{DiffBase: beforeCommit}, Options{})
	onlyRule(t, findings, "VL-010")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	if findings[0].Path != ".verdi/specs/active/vl-010-unstamped-feature/spec.md" {
		t.Fatalf("finding path = %q, want the unstamped feature spec", findings[0].Path)
	}
}

// TestVL010_UnstampedFeatureDeletedAtBase_Fails is the deletion
// complement: 02's "ANY diff touching a frozen [or Git-derived-accepted]
// file fails" covers removal exactly like modification.
func TestVL010_UnstampedFeatureDeletedAtBase_Fails(t *testing.T) {
	beforeDir := adHocOverlayDir(t, ".verdi/specs/active/vl-010-unstamped-feature/spec.md", vl010UnstampedFeatureSpec)
	repo := buildLintRepo(t, beforeDir)
	beforeCommit := repo.Heads[len(repo.Heads)-1]

	specPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "vl-010-unstamped-feature", "spec.md")
	mustRemove(t, specPath)
	commitAll(t, repo.Dir, "delete unstamped feature at the diff base")

	findings := runLint(t, repo.Dir, Context{DiffBase: beforeCommit}, Options{})
	onlyRule(t, findings, "VL-010")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	if findings[0].Path != ".verdi/specs/active/vl-010-unstamped-feature/spec.md" {
		t.Fatalf("finding path = %q, want the unstamped feature spec", findings[0].Path)
	}
}

// TestVL010_NewUnstampedFeaturePath_Succeeds proves adding a brand-new,
// unstamped, statusless feature spec (never existing at the diff base at
// all) never trips VL-010 — the keystone protects existing baselines, not
// new proposals (added files are out of this rule's scope by
// construction, matching every added-file case already proven elsewhere).
func TestVL010_NewUnstampedFeaturePath_Succeeds(t *testing.T) {
	repo := buildLintRepo(t)
	beforeCommit := repo.Heads[len(repo.Heads)-1]

	const newSpec = `---
id: spec/vl-010-new-unstamped-feature
kind: spec
class: feature
title: "VL-010: new unstamped feature"
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
---
# VL-010: new unstamped feature
`
	specPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "vl-010-new-unstamped-feature", "spec.md")
	writeTestFile(t, specPath, newSpec)
	commitAll(t, repo.Dir, "add new unstamped feature")

	findings := runLint(t, repo.Dir, Context{DiffBase: beforeCommit}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-010" {
			t.Fatalf("VL-010 fired on a brand-new unstamped feature path: %s", f.String())
		}
	}
}

// vl010UnstampedComponentSpec is a valid, unfrozen component spec — the
// pre-Task-4 "component specs are authored-living and never frozen"
// behavior (01 §Temporal classes) must be entirely unaffected by
// baseProtected's new feature/story signal.
const vl010UnstampedComponentSpec = `---
id: spec/vl-010-unstamped-component
kind: spec
class: component
title: "VL-010: unstamped component"
status: active
owners: [platform-team]
---
# VL-010: unstamped component
`

// TestVL010_UnstampedComponentModified_StillAllowed is the regression
// guard: an unfrozen component spec at the diff base remains freely
// editable — baseProtected's second signal is scoped to feature/story
// only, exactly as its own doc comment promises.
func TestVL010_UnstampedComponentModified_StillAllowed(t *testing.T) {
	beforeDir := adHocOverlayDir(t, ".verdi/specs/active/vl-010-unstamped-component/spec.md", vl010UnstampedComponentSpec)
	repo := buildLintRepo(t, beforeDir)
	beforeCommit := repo.Heads[len(repo.Heads)-1]

	after := strings.Replace(vl010UnstampedComponentSpec, `title: "VL-010: unstamped component"`, `title: "VL-010: unstamped component EDITED"`, 1)
	specPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "vl-010-unstamped-component", "spec.md")
	writeTestFile(t, specPath, after)
	commitAll(t, repo.Dir, "edit unstamped component at the diff base")

	findings := runLint(t, repo.Dir, Context{DiffBase: beforeCommit}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-010" {
			t.Fatalf("VL-010 fired on an unstamped component spec: %s", f.String())
		}
	}
}

// TestVL010_StatuslessFeaturePureArchiveMoveAllowed proves the
// active-to-archive closure exception (isActiveArchiveMove + e.Pure())
// still admits a statusless feature/story: a byte-identical move requires
// no status-only-flip exception at all (that machinery exists for a
// CONTENT-changing status flip; a pure rename carries none), so it is
// allowed exactly like a legacy frozen spec's pure rename, unaffected by
// baseProtected's new signal.
func TestVL010_StatuslessFeaturePureArchiveMoveAllowed(t *testing.T) {
	beforeDir := adHocOverlayDir(t, ".verdi/specs/active/vl-010-unstamped-feature/spec.md", vl010UnstampedFeatureSpec)
	repo := buildLintRepo(t, beforeDir)
	beforeCommit := repo.Heads[len(repo.Heads)-1]

	activePath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "vl-010-unstamped-feature", "spec.md")
	archivePath := filepath.Join(repo.Dir, ".verdi", "specs", "archive", "vl-010-unstamped-feature", "spec.md")
	mustRemove(t, activePath)
	writeTestFile(t, archivePath, vl010UnstampedFeatureSpec)
	commitAll(t, repo.Dir, "byte-identical archive move of a statusless feature")

	findings := runLint(t, repo.Dir, Context{DiffBase: beforeCommit}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-010" {
			t.Fatalf("VL-010 fired on a byte-identical active->archive move of a statusless feature: %s", f.String())
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
frozen: { at: 2026-05-14, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
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
