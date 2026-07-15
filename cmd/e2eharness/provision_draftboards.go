package main

// The draft-boards fixture provisioning (spec/draft-boards; e2e
// tests/38-draft-boards.spec.ts): the branch shapes the /b/{branch}
// routes must serve. Every name below is bound by e2e/tests/fixtures.ts —
// change them together.
//
//   - design/draft-tab-a  local, carrying draft spec draft-tab-a — tab A
//     of the two-tab isolation proof (ac-2), and ac-1's authoring wall
//   - design/draft-tab-b  local, carrying draft spec draft-tab-b — tab B
//   - design/decline-ledger-v2  local, carrying a DRAFT edition of
//     decline-ledger (landed on main by provision.go) — ac-3's same-spec
//     fixture: sealed at /board/spec/decline-ledger, authoring under /b/
//   - design/sealed-remote  remote-tracking ONLY (pushed to the local
//     bare origin, local branch deleted) — dc-4's sealed render fixture
//
// Runs AFTER provisionBoard (the checkout sits on design/refi-decline-flow)
// and restores that checkout when done, so `verdi serve`'s branch state —
// the board suite's authoring-mode fixture — is untouched. Every branch is
// cut from main so its tree carries the corpus (peek targets, the
// committed .verdi/.gitignore that keeps the data zone untracked).

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	dbTabAName         = "draft-tab-a"
	dbTabBName         = "draft-tab-b"
	dbSealedRemoteName = "sealed-remote"
	dbSameSpecName     = "decline-ledger" // landed on main by provision.go
	dbSameSpecBranch   = "design/decline-ledger-v2"
)

// dbSameSpecLanded is decline-ledger's LANDED edition — written onto main
// by provisionStore (provision.go) so it is on the default branch and on
// every branch cut from it, and renders as the sealed read-only record at
// the unprefixed /board/spec/decline-ledger.
const dbSameSpecLanded = `---
id: spec/decline-ledger
kind: spec
class: feature
title: "Decline ledger"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "decline decisions are not ledgered durably", anchor: "#problem" }
outcome: { text: "landed ledger outcome (draft boards e2e)", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "every decline decision is ledgered", evidence: [attestation], anchor: "#ac-1" }
frozen: { at: 2026-07-01, commit: 2f230011b192c5ac1c0ed5442be76fc401c4cbca }
---
# Decline ledger

## Problem

## Outcome

## ac-1

Prose.
`

// draftBoardSpec renders a minimal valid feature-class draft whose one AC
// card is the two-tab edit target. The problem text carries a
// branch-unique marker the e2e asserts board content against.
func draftBoardSpec(name, marker string) string {
	return `---
id: spec/` + name + `
kind: spec
class: feature
title: "` + name + ` (draft boards fixture)"
status: draft
owners: [platform-team]
problem: { text: "` + marker + `: this flow exists only on its own design branch", anchor: "#problem" }
outcome: { text: "the ` + name + ` flow is specified on its own branch", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "` + name + ` original criterion", evidence: [attestation], anchor: "#ac-1" }
---
# ` + name + `

## Problem

## Outcome

## ac-1

Prose.
`
}

// dbSameSpecDraftEdition is decline-ledger's DRAFT edition (ac-3): the
// SAME spec name that is landed read-only on main, as a draft on its own
// design branch — the mode law's two simultaneous truths of one spec.
const dbSameSpecDraftEdition = `---
id: spec/decline-ledger
kind: spec
class: feature
title: "Decline ledger (draft edition)"
status: draft
owners: [platform-team]
problem: { text: "the landed decline-ledger record needs a second round", anchor: "#problem" }
outcome: { text: "draft edition outcome (draft boards e2e)", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "the second round is specified", evidence: [attestation], anchor: "#ac-1" }
---
# Decline ledger (draft edition)

## Problem

## Outcome

## ac-1

Prose.
`

// provisionDraftBoards cuts the four fixture branches from main, each
// carrying its files as one committed layer, pushes the sealed-remote
// branch to the local bare origin and deletes its local branch, then
// restores the serving checkout to designBranch.
func provisionDraftBoards(storeRoot string) error {
	type fixture struct {
		branch string
		files  map[string]string
		remote bool // push to origin and delete the local branch
	}
	fixtures := []fixture{
		{branch: "design/" + dbTabAName, files: map[string]string{
			filepath.Join(".verdi", "specs", "active", dbTabAName, "spec.md"): draftBoardSpec(dbTabAName, "tab A problem"),
		}},
		{branch: "design/" + dbTabBName, files: map[string]string{
			filepath.Join(".verdi", "specs", "active", dbTabBName, "spec.md"): draftBoardSpec(dbTabBName, "tab B problem"),
		}},
		{branch: dbSameSpecBranch, files: map[string]string{
			filepath.Join(".verdi", "specs", "active", dbSameSpecName, "spec.md"): dbSameSpecDraftEdition,
		}},
		{branch: "design/" + dbSealedRemoteName, files: map[string]string{
			filepath.Join(".verdi", "specs", "active", dbSealedRemoteName, "spec.md"): draftBoardSpec(dbSealedRemoteName, "sealed remote problem"),
		}, remote: true},
	}

	for _, f := range fixtures {
		if err := runGit(storeRoot, nil, "checkout", "--quiet", "-b", f.branch, "main"); err != nil {
			return fmt.Errorf("cutting %s: %w", f.branch, err)
		}
		for rel, content := range f.files {
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
		if err := runGit(storeRoot, nil, "commit", "--quiet", "--no-verify", "-m", "design: "+f.branch+" draft-boards fixture"); err != nil {
			return err
		}
		if f.remote {
			if err := runGit(storeRoot, nil, "push", "--quiet", "origin", f.branch); err != nil {
				return err
			}
			// Step off before deleting: a branch cannot be deleted while
			// checked out.
			if err := runGit(storeRoot, nil, "checkout", "--quiet", "main"); err != nil {
				return err
			}
			if err := runGit(storeRoot, nil, "branch", "--quiet", "-D", f.branch); err != nil {
				return err
			}
		}
	}

	// Restore the board suite's serving branch state.
	if err := runGit(storeRoot, nil, "checkout", "--quiet", designBranch); err != nil {
		return fmt.Errorf("restoring %s: %w", designBranch, err)
	}
	return nil
}
