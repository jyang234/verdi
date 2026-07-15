package main

// The directory home's fixture provisioning (spec/directory-home; e2e
// tests/37-directory-home.spec.ts): the ref shapes the whole-store
// directory must render beyond what provision_board.go already leaves
// behind (design/refi-decline-flow, local AND pushed → source "local +
// remote"). Every name below is bound by e2e/tests/fixtures.ts — change
// them together.
//
//   - design/audit-trail       local only            → source "local branch"
//   - design/vendor-onboarding remote-tracking only  → source "remote-tracking"
//     (pushed, then the local branch deleted)
//   - design/uncharted-idea    cut from main, NO draft spec → the ac-3
//     disclosed notice entry (never linked as if a board existed)
//   - design/doomed-draft      local only; the e2e deletes it mid-session
//     through the control server (control.go) and clicks its stale link
//
// Runs AFTER provisionBoard (the checkout sits on design/refi-decline-flow)
// and restores that checkout when done, so `verdi serve`'s branch state —
// the board suite's authoring-mode fixture — is untouched.

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	dirLocalDraftName  = "audit-trail"
	dirRemoteDraftName = "vendor-onboarding"
	dirEmptyBranchName = "uncharted-idea"
	dirDoomedDraftName = "doomed-draft"
)

// directoryDraftSpec renders the leanest valid story-class draft for one
// directory fixture branch (mirroring provision_board.go's emptySpec
// shape: class story requires problem, outcome, a tracker ref, and one
// implements edge; the target is real on main).
func directoryDraftSpec(name, story string) string {
	return `---
id: spec/` + name + `
kind: spec
class: story
title: "` + name + ` (directory fixture)"
status: draft
owners: [platform-team]
story: ` + story + `
problem: { text: "the ` + name + ` flow is untracked", anchor: "#problem" }
outcome: { text: "the ` + name + ` flow is specified", anchor: "#outcome" }
links:
  - { type: implements, ref: spec/escrow-autopay#ac-1 }
---
# ` + name + `

## Problem

## Outcome
`
}

// provisionDirectory authors the directory fixture branches above.
func provisionDirectory(storeRoot string) error {
	drafts := []struct{ name, story string }{
		{dirLocalDraftName, "jira:LOAN-2301"},
		{dirRemoteDraftName, "jira:LOAN-2302"},
		{dirDoomedDraftName, "jira:LOAN-2303"},
	}
	for _, d := range drafts {
		branch := "design/" + d.name
		if err := runGit(storeRoot, nil, "checkout", "--quiet", "-b", branch, "main"); err != nil {
			return fmt.Errorf("cutting %s: %w", branch, err)
		}
		rel := filepath.Join(".verdi", "specs", "active", d.name, "spec.md")
		path := filepath.Join(storeRoot, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", filepath.Dir(rel), err)
		}
		if err := os.WriteFile(path, []byte(directoryDraftSpec(d.name, d.story)), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", rel, err)
		}
		if err := runGit(storeRoot, nil, "add", rel); err != nil {
			return err
		}
		if err := runGit(storeRoot, nil, "commit", "--quiet", "--no-verify", "-m", "design: "+d.name+" directory fixture"); err != nil {
			return err
		}
	}

	// The remote-only shape: push (creating the remote-tracking ref), then
	// delete the local branch once we are off it.
	if err := runGit(storeRoot, nil, "push", "--quiet", "origin", "design/"+dirRemoteDraftName); err != nil {
		return err
	}

	// The empty-branch shape (ac-3): a branch cut from main that never
	// received a draft spec — a plain ref creation, no checkout needed.
	if err := runGit(storeRoot, nil, "branch", "design/"+dirEmptyBranchName, "main"); err != nil {
		return err
	}

	// Restore the board suite's serving checkout, then drop the local half
	// of the remote-only fixture.
	if err := runGit(storeRoot, nil, "checkout", "--quiet", designBranch); err != nil {
		return err
	}
	if err := runGit(storeRoot, nil, "branch", "-D", "design/"+dirRemoteDraftName); err != nil {
		return err
	}
	return nil
}
