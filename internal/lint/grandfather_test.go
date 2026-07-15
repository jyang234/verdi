package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardlayout"
)

const grandfatherBadSpec = `---
id: spec/grandfather-bad
kind: spec
class: feature
title: "grandfather: bad decode"
status: draft
owners: [platform-team]
story: jira:LOAN-0099
bogus_field: "would fail VL-001"
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [] }
---
# grandfather: bad
`

// TestGrandfatherArchive_OQ3 proves Options.GrandfatherArchive (OQ-3: "skip
// VL-001..006 under specs/archive/ on import") is implemented but off by
// default (dormant): the same badly-shaped file under specs/archive/ fires
// VL-001 (and would fire VL-006) with the option off, and fires nothing
// with it on.
func TestGrandfatherArchive_OQ3(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/archive/grandfather-bad/spec.md", grandfatherBadSpec)
	repo := buildLintRepo(t, dir)

	t.Run("off by default: fires", func(t *testing.T) {
		findings := runLint(t, repo.Dir, Context{}, Options{})
		found := false
		for _, f := range findings {
			if f.Rule == "VL-001" && f.Path == ".verdi/specs/archive/grandfather-bad/spec.md" {
				found = true
			}
		}
		if !found {
			t.Fatalf("VL-001 did not fire on the archived bad spec with GrandfatherArchive off:\n%s", findingsString(findings))
		}
	})

	t.Run("on: dormant, does not fire", func(t *testing.T) {
		findings := runLint(t, repo.Dir, Context{}, Options{GrandfatherArchive: true})
		for _, f := range findings {
			if f.Path == ".verdi/specs/archive/grandfather-bad/spec.md" {
				t.Fatalf("finding fired on a grandfathered archive file: %s", f.String())
			}
		}
	})
}

// archivedQuartetDispositionsSpec is a v0-style archived feature spec whose
// dispositions: block is REAL (three entries: incorporated with a
// resolving where anchor, contradicted with a note, and a bare
// open-question) — every entry individually valid AND bidirectionally
// reconciled against its board.json sibling below.
//
// This is deliberately NOT examples/showcase's own archived quartet
// (loan-refi-2023): that fixture's board.json carries an empty
// stickies: [] and its spec.md carries no dispositions: block at all, so
// TestClean_CorpusLintsGreen alone never actually exercises VL-014's
// grandfathered per-entry/reconcile logic (vl014.go: `if
// len(spec.Spec.Dispositions) == 0 { continue }` skips the whole rule for
// that fixture). This fixture closes that gap, proving the literal exit
// criterion — "verdi lint against v0's own archived quartets (frozen
// board.json + dispositions: blocks) exits 0 unchanged" — against
// content that genuinely walks VL-014's grandfathered code path rather
// than trivially bypassing it.
const archivedQuartetDispositionsSpec = `---
id: spec/archived-quartet-dispositions
kind: spec
class: feature
title: "Archived quartet: grandfathered dispositions, clean"
status: closed
owners: [platform-team]
story: jira:LOAN-9001
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
dispositions:
  - { sticky: a-01J8Z0K3AAAAAAAAAAAAAAAAAA, disposition: incorporated, where: "#design-notes" }
  - { sticky: a-01J8Z0K4BBBBBBBBBBBBBBBBBB, disposition: contradicted, note: "superseded by the final ac-1 wording" }
  - { sticky: a-01J8Z0K5CCCCCCCCCCCCCCCCCC, disposition: open-question }
frozen: { at: 2026-06-20, commit: faf8d8c412c9df35b5a445146a5fe0e8309caa71 }
---
# Archived quartet: grandfathered dispositions, clean

## Design notes

The incorporated disposition's where anchor resolves here.
`

// archivedQuartetDispositionsBoardJSON's three stickies exactly match
// archivedQuartetDispositionsSpec's three dispositions: entries, in both
// directions (VL-014's reconcile: no dangling disposition, no
// undispositioned sticky).
const archivedQuartetDispositionsBoardJSON = `{
  "schema": "verdi.board/v1",
  "pins": [],
  "stickies": [
    { "id": "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", "x": 10, "y": 10 },
    { "id": "a-01J8Z0K4BBBBBBBBBBBBBBBBBB", "x": 20, "y": 20 },
    { "id": "a-01J8Z0K5CCCCCCCCCCCCCCCCCC", "x": 30, "y": 30 }
  ],
  "yarn": []
}
`

// TestGrandfatherArchivedQuartet_DispositionsBlock_LintsClean is this
// phase's (V1-P9, item 3) primary exit-criterion proof: `verdi lint`
// against a v0 archived quartet — frozen board.json plus a real,
// fully-valid dispositions: block, VL-014's grandfathered scope — exits 0
// unchanged, under default Options{} (the same policy `verdi lint` runs
// in CI, no special flags).
func TestGrandfatherArchivedQuartet_DispositionsBlock_LintsClean(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, ".verdi/specs/archive/archived-quartet-dispositions")
	writeTestFile(t, filepath.Join(base, "spec.md"), archivedQuartetDispositionsSpec)
	writeTestFile(t, filepath.Join(base, "board.json"), archivedQuartetDispositionsBoardJSON)

	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Path == ".verdi/specs/archive/archived-quartet-dispositions/spec.md" ||
			f.Path == ".verdi/specs/archive/archived-quartet-dispositions/board.json" {
			t.Fatalf("v0 archived quartet (frozen board.json + a real dispositions: block, every entry valid and bidirectionally reconciled) produced a finding — want 0: %s", f.String())
		}
	}
}

// TestVL014_NewStyleSpec_Archive_NoDispositionsBlock_NeverFires is the
// archive/ side of TestVL014_NewStyleSpec_NoDispositionsBlock_NeverFires
// (vl014_test.go, V1-P2): the exit criterion's synthetic negative case —
// "a new-style spec with a disagreeing sibling board.json is never caught
// by the narrowed VL-014" — proven again under specs/archive/, not just
// specs/active/. A new-style spec (round-four surface fields, no
// dispositions: block at all) that has already been moved to the archive
// (a closed story/feature) must not start tripping VL-014 just because it
// now shares its directory with a stale/leftover board.json.
func TestVL014_NewStyleSpec_Archive_NoDispositionsBlock_NeverFires(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, ".verdi/specs/archive/vl-014-new-style-archived")
	spec := `---
id: spec/vl-014-new-style-archived
kind: spec
class: feature
title: "VL-014 grandfather-scope-negative: new-style spec, archived, no dispositions"
status: closed
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: "#ac-1" }
frozen: { at: 2026-06-20, commit: faf8d8c412c9df35b5a445146a5fe0e8309caa71 }
---
# VL-014 grandfather-scope-negative: new-style spec, archived, no dispositions

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.
`
	board := `{
  "schema": "verdi.board/v1",
  "pins": [],
  "stickies": [
    { "id": "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", "x": 10, "y": 10 }
  ],
  "yarn": []
}
`
	writeTestFile(t, filepath.Join(base, "spec.md"), spec)
	writeTestFile(t, filepath.Join(base, "board.json"), board)

	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-014" {
			t.Fatalf("VL-014 fired on an ARCHIVED new-style spec with no dispositions: block: %s", f.String())
		}
	}
}

// archivedRoundFourStorySpec is a round-four ("new-style") FEATURE spec,
// already closed and archived — the shape 03 §Evidence model's round-four
// note describes: "new specs archive layout.json ... in the board slot
// instead of a frozen board.json — the board is a projection of the spec,
// not a separate authored artifact, so there is no board snapshot left to
// freeze." No dispositions: block (a round-four spec's readiness ran
// through the board's own scratch-tier rules while it was still on its
// design branch, R4-I-9).
//
// Judgment call (disclosed here and in the phase report, per CLAUDE.md's
// provenance discipline): 02 §Artifact contract's kind-registry table
// states BOTH feature and story specs follow "draft ->
// accepted-pending-build -> closed(archive)" — implying a story spec
// should archive too. VL-002's actual implementation (checkSpecPath,
// vl002.go) only auto-archives class: feature ("feature specs move to
// archive/ once closed; component specs always stay in active/" — its own
// doc comment never mentions story at all): a class: story spec with
// status: closed under specs/archive/ fires VL-002 ("belongs under
// specs/active/, not specs/archive/"), and one left under specs/active/
// would conversely never fire VL-002 at all regardless of status. This
// looks like a real gap between 02's table and VL-002's scope, but fixing
// VL-002 is out of this test-only phase's charge (internal/lint,
// "grandfathering audit tests only") — disclosed, not silently
// invented around. This fixture uses class: feature instead (VL-002's
// actual, current archive-eligible shape), which is still a genuine
// round-four spec (isNewClassSpec, vl006.go: a feature spec is new-class
// the moment it carries any round-four surface field — problem/outcome
// here), so it still exercises exactly what this test needs: a real,
// closed, round-four-shaped archived quartet with a layout.json board
// slot.
const archivedRoundFourStorySpec = `---
id: spec/archived-round4-story
kind: spec
class: feature
title: "Archived round-four feature (layout.json board slot)"
status: closed
owners: [platform-team]
problem: { text: "a closed round-four spec's archived form was never proven to lint/decode validly under the round-four layout.json board slot", anchor: "#problem" }
outcome: { text: "an archived round-four quartet (spec + layout.json + rollup.json + deviation-report.md) lints clean and decodes exactly like an active spec", anchor: "#outcome" }
story: jira:LOAN-9002
acceptance_criteria:
  - { id: ac-1, text: "the archived spec's positions resolve against its own declared objects", evidence: [static], anchor: "#ac-1" }
frozen: { at: 2026-06-20, commit: faf8d8c412c9df35b5a445146a5fe0e8309caa71 }
---
# Archived round-four feature (layout.json board slot)

## Problem

A closed round-four spec's archived form was never proven to lint and
decode validly under the round-four layout.json board slot (03 §Evidence
model, round-four note).

## Outcome

An archived round-four quartet (spec, layout.json, rollup.json,
deviation-report.md) lints clean and decodes exactly like an active spec.

## AC-1

The archived spec's positions resolve against its own declared objects.
`

// archivedRoundFourLayoutJSON is the round-four board slot: positions
// only, `verdi.boardlayout/v1` — VL-018's shape, resolving against
// archivedRoundFourStorySpec's one declared object (ac-1).
const archivedRoundFourLayoutJSON = `{
  "schema": "verdi.boardlayout/v1",
  "positions": {
    "ac-1": { "x": 40, "y": 20 }
  }
}
`

// archivedRoundFourRollupJSON and archivedRoundFourDeviationReport
// complete the archived quartet (03: "spec, board.json [here: layout.json],
// rollup.json, deviation-report.md"), mirroring examples/showcase's own
// loan-refi-2023 quartet's shape. Neither file is read by any VL-xxx
// rule (only spec.md and layout.json/board.json are lint-relevant), so
// their presence here is about proving the FULL quartet — not just the
// spec — is a coherent, valid archived record, per this phase's brief.
const archivedRoundFourRollupJSON = `{
  "schema": "verdi.rollup/v1",
  "story": "jira:LOAN-9002",
  "ref": "spec/archived-round4-story",
  "commit": "faf8d8c412c9df35b5a445146a5fe0e8309caa71",
  "criteria": [
    { "id": "ac-1", "text": "the archived spec's positions resolve against its own declared objects", "status": "evidenced", "summary": "VL-018 passes; positions resolve" }
  ],
  "eligible": true,
  "digest": "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
}
`

const archivedRoundFourDeviationReport = `---
schema: verdi.deviation/v1
covers: faf8d8c412c9df35b5a445146a5fe0e8309caa71
findings: []
digest: sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
integrity: sha256:ddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddde
frozen: { at: 2026-06-20, commit: faf8d8c412c9df35b5a445146a5fe0e8309caa71 }
provenance: { generator: verdi-align, version: v1, inputs: [spec/archived-round4-story@faf8d8c412c9df35b5a445146a5fe0e8309caa71], digest: sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd }
---
# Alignment report: archived-round4-story (final edition)

## Computed

No findings.

## Judged

No findings.
`

// TestArchivedQuartet_RoundFourShape_LintsAndDecodesValidly is this
// phase's (V1-P9, item 3) third exit-criterion clause: "the round-4
// archive shape: an archived quartet whose board slot is layout.json (03
// §Alignment report's round-four note) lints and renders validly." No
// fixture of this shape existed under testdata/ before this phase (grep
// across testdata/ and internal/lint found no archived spec paired with
// layout.json rather than board.json), so this adds one.
//
// "Renders validly" is proven two ways, staying within this package's own
// lane (internal/lint — grandfathering audit tests, this phase's scope;
// internal/workbench's/internal/dex's own render paths are proven by
// their own packages' tests, not duplicated here, mirroring this
// package's TestV0ThinSliceChecklist-adjacent doc precedent of proving
// existence/wiring, not re-deriving another package's coverage):
//
//  1. Zero lint findings — which already entails the artifact contract's
//     rendering preconditions: VL-001 strict decode succeeds, VL-006
//     resolves every declared object's anchor against a real body
//     heading (SpecFrontmatter.ResolveObjectAnchors — the same call a
//     render path would need to succeed), and VL-018 resolves every
//     layout.json position key against a declared object id.
//  2. This test ALSO decodes the spec and reads the layout.json directly
//     (bypassing the lint engine entirely) and re-checks the same two
//     preconditions by hand, so the proof does not rest solely on lint's
//     internal wiring continuing to call them.
func TestArchivedQuartet_RoundFourShape_LintsAndDecodesValidly(t *testing.T) {
	const specRelDir = ".verdi/specs/archive/archived-round4-story"

	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, specRelDir, "spec.md"), archivedRoundFourStorySpec)
	writeTestFile(t, filepath.Join(dir, specRelDir, "layout.json"), archivedRoundFourLayoutJSON)
	writeTestFile(t, filepath.Join(dir, specRelDir, "rollup.json"), archivedRoundFourRollupJSON)
	writeTestFile(t, filepath.Join(dir, specRelDir, "deviation-report.md"), archivedRoundFourDeviationReport)

	repo := buildLintRepo(t, dir)

	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if filepath.ToSlash(filepath.Dir(f.Path)) == specRelDir {
			t.Fatalf("round-four archived quartet (layout.json board slot) produced a finding — want 0: %s", f.String())
		}
	}

	// Independent re-check #2: decode the spec and layout.json directly,
	// outside the lint engine, and re-verify anchor/position resolution
	// by hand.
	fm, body, err := readDecodedSpec(repo.Dir, specRelDir+"/spec.md")
	if err != nil {
		t.Fatalf("decoding archived spec.md directly: %v", err)
	}
	if err := fm.ResolveObjectAnchors(body); err != nil {
		t.Fatalf("archived spec's declared object anchors do not resolve against its own body: %v", err)
	}
	positions, err := boardlayout.ReadFile(filepath.Join(repo.Dir, filepath.FromSlash(specRelDir)))
	if err != nil {
		t.Fatalf("reading archived layout.json directly: %v", err)
	}
	declared := artifact.DeclaredObjectIDs(fm)
	for key := range positions {
		if !declared[key] {
			t.Fatalf("archived layout.json position key %q does not resolve to a declared object id", key)
		}
	}
	if len(positions) == 0 {
		t.Fatal("archived layout.json decoded with zero positions — fixture regression, this test proves nothing")
	}
}

// readDecodedSpec reads and strict-decodes relPath (store-root-relative)
// from repoDir directly, bypassing the lint engine — the second,
// independent half of TestArchivedQuartet_RoundFourShape_LintsAndDecodesValidly's
// proof.
func readDecodedSpec(repoDir, relPath string) (*artifact.SpecFrontmatter, []byte, error) {
	raw, err := os.ReadFile(filepath.Join(repoDir, filepath.FromSlash(relPath)))
	if err != nil {
		return nil, nil, err
	}
	fmBytes, body, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		return nil, nil, err
	}
	fm, err := artifact.DecodeSpec(fmBytes)
	if err != nil {
		return nil, nil, err
	}
	return fm, body, nil
}
