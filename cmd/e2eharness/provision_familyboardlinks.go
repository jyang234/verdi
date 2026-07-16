package main

// The family-board-links fixture provisioning (spec/family-board-links;
// e2e/tests/43-family-board-links.spec.ts). Per dc-5, AC-1's story-to-
// feature-board direction and AC-2's ACTIVE-match direction need NO new
// fixture data at all — they drive the already-committed showcase pair
// examples/showcase/.verdi/specs/active/{stale-decline,borrower-update-api}
// (SHOWCASE.READONLY_SPEC / SHOWCASE.STORY_STUB_MATCHED in fixtures.ts).
// This file provisions only the THREE scenarios dc-5 names as needing new
// data:
//
//   - a feature whose stub is matched ONLY by a story that resolves in
//     specs/archive/ (AC-2's archived-match branch, ADJ-28's completion
//     reading) — flParentName's ac-1 / flArchivedChildName;
//   - a stub with NO matching story anywhere, whose refs/heads/design/<slug>
//     exists locally (AC-3's ref-present in-between branch) —
//     flParentName's ac-2 / flInstantiatedChildName;
//   - a story whose implements edge targets a feature ref absent from the
//     store (AC-4) — flDanglingStoryName.
//
// AC-3's ref-ABSENT branch (the no-match-no-ref stub) needs no branch and
// no story at all — flParentName's own ac-3/flUnstartedChildName stub
// declares no realizing story anywhere and cuts no design branch; its
// absence IS the fixture.
//
// Runs AFTER provisionBoard and provisionDiagrams (the checkout sits on
// designBranch): the always-visible fixtures (the feature, the archived
// child, the dangling story) commit directly onto it. flInstantiatedChildName
// gets its OWN design/<slug> branch, cut from main exactly like
// provisionDraftBoards' own fixture branches — a genuinely scaffolded
// story, committed there, never merged into designBranch, so the served
// checkout's index never reads it (dc-3: "not yet in this checkout's
// active store" is literal here, not simulated). Restores designBranch
// when done, mirroring provisionDraftBoards' own convention exactly.
// Every name below is bound by e2e/tests/fixtures.ts — change them
// together.

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	flParentName            = "family-links-parent"
	flArchivedChildName     = "family-links-archived-child"
	flInstantiatedChildName = "family-links-instantiated-child"
	flUnstartedChildName    = "family-links-unstarted-child"
	flDanglingStoryName     = "family-links-dangling-story"
	flDanglingTargetFeature = "family-links-no-such-feature"
)

// flParentSpec declares three stubs, one per attachStubStoryLinks scenario
// this story must render distinctly on the SAME wall: ac-1's stub is
// realized only by an archived story, ac-2's stub is instantiated (a
// design branch exists) but unmatched anywhere, and ac-3's stub is
// neither matched nor instantiated at all.
func flParentSpec(commit string) string {
	return `---
id: spec/` + flParentName + `
kind: spec
class: feature
title: "Family links parent (e2e fixture)"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "a feature's stub cards give no sign which of them a story already realizes, archived or not", anchor: "#problem" }
outcome: { text: "every stub card discloses its realization state honestly, whichever zone the realizing story resolves in", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "the archived-child stub's realization is discoverable", evidence: [static], anchor: "#ac-1" }
  - { id: ac-2, text: "the instantiated-but-unlanded stub's realization is discoverable", evidence: [static], anchor: "#ac-2" }
  - { id: ac-3, text: "the un-instantiated stub renders plainly", evidence: [static], anchor: "#ac-3" }
stubs:
  - { slug: ` + flArchivedChildName + `, acceptance_criteria: [ac-1] }
  - { slug: ` + flInstantiatedChildName + `, acceptance_criteria: [ac-2] }
  - { slug: ` + flUnstartedChildName + `, acceptance_criteria: [ac-3] }
frozen: { at: 2026-07-01, commit: ` + commit + ` }
---
# Family links parent

## Problem

## Outcome

## ac-1

Realized by an archived story.

## ac-2

Instantiated on a design branch, not yet landed.

## ac-3

Neither realized nor instantiated.
`
}

// flArchivedChildSpec is written DIRECTLY under specs/archive/ (never
// specs/active/) — a closed, frozen story implementing flParentName's
// ac-1, so the served checkout's index discovers it as an ARCHIVED match
// (dc-1's zone-agnostic backlink walk) rather than an active one.
func flArchivedChildSpec(commit string) string {
	return `---
id: spec/` + flArchivedChildName + `
kind: spec
class: story
title: "Family links archived child (e2e fixture)"
status: closed
owners: [platform-team]
story: jira:VERDI-901
problem: { text: "family-links-parent's ac-1 had no implementing story", anchor: "#problem" }
outcome: { text: "family-links-parent's ac-1 is implemented and this story is closed", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/` + flParentName + `#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "the archived child satisfies its parent's ac-1", evidence: [static], anchor: "#ac-1" }
frozen: { at: 2026-07-01, commit: ` + commit + ` }
---
# Family links archived child

## Problem

## Outcome

## ac-1

Closed and archived; the board must still link to it, disclosing that state.
`
}

// flDanglingStorySpec's implements edge targets flDanglingTargetFeature's
// ac-1 — a feature ref absent from the store — the AC-4 unresolved-target
// fixture (dc-5's EDGE-zone convention).
const flDanglingStorySpec = `---
id: spec/` + flDanglingStoryName + `
kind: spec
class: story
title: "Family links dangling story (e2e fixture)"
status: draft
owners: [platform-team]
story: jira:VERDI-902
problem: { text: "this story's parent feature ref does not resolve anywhere in the store", anchor: "#problem" }
outcome: { text: "the board discloses the unresolved parent ref in place of a dead link", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/` + flDanglingTargetFeature + `#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "the story validates with one acceptance criterion", evidence: [static], anchor: "#ac-1" }
---
# Family links dangling story

## Problem

## Outcome

## ac-1

The parent this story implements was never real in this store.
`

// flInstantiatedChildSpec is committed ONLY onto its own design/<slug>
// branch (never onto designBranch) — a genuinely scaffolded story
// mirroring stub-instantiate's own output, implementing flParentName's
// ac-2, but never merged into the served checkout.
const flInstantiatedChildSpec = `---
id: spec/` + flInstantiatedChildName + `
kind: spec
class: story
title: "Family links instantiated child (e2e fixture)"
status: draft
owners: [platform-team]
story: jira:VERDI-903
problem: { text: "family-links-parent's ac-2 was just instantiated", anchor: "#problem" }
outcome: { text: "this story exists only on its own design branch so far", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/` + flParentName + `#ac-2" }
acceptance_criteria:
  - { id: ac-1, text: "the story validates with one acceptance criterion", evidence: [static], anchor: "#ac-1" }
---
# Family links instantiated child

## Problem

## Outcome

## ac-1

Scaffolded, but not yet in this checkout's active store.
`

// provisionFamilyBoardLinks writes the always-visible fixtures (the
// parent feature, the archived child, the dangling story) onto designBranch
// (already checked out at this point), then cuts flInstantiatedChildName's
// own design branch from main, commits its scaffolded story there, and
// restores designBranch.
func provisionFamilyBoardLinks(storeRoot string) error {
	commit, err := gitOutput(storeRoot, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("resolving HEAD for family-board-links fixtures: %w", err)
	}

	files := map[string]string{
		filepath.Join(".verdi", "specs", "active", flParentName, "spec.md"):         flParentSpec(commit),
		filepath.Join(".verdi", "specs", "archive", flArchivedChildName, "spec.md"): flArchivedChildSpec(commit),
		filepath.Join(".verdi", "specs", "active", flDanglingStoryName, "spec.md"):  flDanglingStorySpec,
	}
	for rel, content := range files {
		path := filepath.Join(storeRoot, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", filepath.Dir(rel), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", rel, err)
		}
	}
	if err := runGit(storeRoot, nil, "add", "-A"); err != nil {
		return err
	}
	if err := runGit(storeRoot, nil, "commit", "--quiet", "--no-verify", "-m", "design: family-board-links fixtures (archived-match + dangling implements target)"); err != nil {
		return err
	}

	// The instantiated-but-unlanded stub (AC-3's ref-present in-between
	// branch): cut from main, exactly like provisionDraftBoards' own
	// fixture branches, so its tree still carries the corpus.
	branch := "design/" + flInstantiatedChildName
	if err := runGit(storeRoot, nil, "checkout", "--quiet", "-b", branch, "main"); err != nil {
		return fmt.Errorf("cutting %s: %w", branch, err)
	}
	instPath := filepath.Join(storeRoot, ".verdi", "specs", "active", flInstantiatedChildName, "spec.md")
	if err := os.MkdirAll(filepath.Dir(instPath), 0o755); err != nil {
		return fmt.Errorf("creating %s dir: %w", flInstantiatedChildName, err)
	}
	if err := os.WriteFile(instPath, []byte(flInstantiatedChildSpec), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", flInstantiatedChildName, err)
	}
	if err := runGit(storeRoot, nil, "add", "-A"); err != nil {
		return err
	}
	if err := runGit(storeRoot, nil, "commit", "--quiet", "--no-verify", "-m", "design: "+branch+" family-board-links fixture (instantiated, not yet landed)"); err != nil {
		return err
	}

	// Restore the board suite's serving checkout.
	if err := runGit(storeRoot, nil, "checkout", "--quiet", designBranch); err != nil {
		return fmt.Errorf("restoring %s: %w", designBranch, err)
	}
	return nil
}
