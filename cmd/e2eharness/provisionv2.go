package main

// The v1 board's fixture provisioning (e2e/tests-v1/README.md "Harness
// obligations"): a draft spec on a design branch (the board opens it in
// AUTHORING mode), a spec under MR review whose comment feed is a canned
// local file (REVIEW mode, no network), the ADR the design spec's
// decision exempts, and a bare local "origin" so the board's
// commit-and-push affordance round-trips hermetically. Every name and
// body below is bound by e2e/tests-v1/fixtures.ts — change them together.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	designSpecName = "refi-decline-flow"
	designBranch   = "design/" + designSpecName
	reviewSpecName = "stale-decline-notices"
)

// designSpec is DESIGN_SPEC: the object model fixtures.ts binds (3 ACs,
// 1 constraint, dc-1 carrying the declared exempts edge to ADR_REF, dc-2
// plain), with problem/outcome texts containing PROBLEM_SNIPPET
// ("stale decline") and OUTCOME_SNIPPET ("declined applicants").
const designSpec = `---
id: spec/refi-decline-flow
kind: spec
class: feature
title: "Refinancing decline flow"
status: draft
owners: [platform-team]
problem: { text: "applicants keep acting on stale decline reasons after the underlying data changes", anchor: "#problem" }
outcome: { text: "declined applicants see the current decline state and a concrete next step", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a declined applicant sees the current decline reason within a minute of a data change", evidence: [behavioral, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "a reversed decline clears the notice everywhere it was shown", evidence: [behavioral, attestation], anchor: "#ac-2" }
  - { id: ac-3, text: "support can audit every decline notice ever shown", evidence: [static, attestation], anchor: "#ac-3" }
constraints:
  - { id: co-1, text: "decline notices never expose internal model scores", anchor: "#co-1" }
decisions:
  - { id: dc-1, text: "excuse decline events from the synchronous-write rule", anchor: "#dc-1",
      links: [ { type: exempts, ref: adr/0012-outbox-loansvc-events, note: "decline events are already async via the outbox" } ] }
  - { id: dc-2, text: "reuse the existing notification channel for decline updates", anchor: "#dc-2" }
---
# Refinancing decline flow

## Problem

Declined applicants act on stale reasons.

## Outcome

The decline state a borrower sees is always current.

## ac-1

Currency of the visible decline reason.

## ac-2

Reversal clears every surface.

## ac-3

Auditability of shown notices.

## co-1

Model scores stay internal.

## dc-1

The outbox already decouples decline events.

## dc-2

No second channel.
`

// designSpecLayout stores positions for a SUBSET of the objects (ac-1,
// dc-1), proving both the stored-verbatim path and the zoned fallback.
// The stored pixels sit exactly on their zones' first grid slot, so the
// zoned algorithm's occupancy check routes the unstored siblings to the
// next free slots — no overlap.
const designSpecLayout = `{
  "schema": "verdi.boardlayout/v1",
  "positions": { "ac-1": { "x": 40, "y": 40 }, "dc-1": { "x": 480, "y": 40 } }
}
`

// reviewSpec is REVIEW_SPEC: the board opens it in review mode (its
// canned feed reports an open MR); ac-2 is the anchored comment's
// target.
const reviewSpec = `---
id: spec/stale-decline-notices
kind: spec
class: feature
title: "Stale decline notices"
status: draft
owners: [platform-team]
problem: { text: "decline notices linger after the decline is stale", anchor: "#problem" }
outcome: { text: "notices retract themselves when a decline goes stale", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a stale decline retracts its notice", evidence: [behavioral, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "the retraction reaches every notified channel", evidence: [behavioral, attestation], anchor: "#ac-2" }
  - { id: ac-3, text: "a retracted notice is audit-visible", evidence: [static, attestation], anchor: "#ac-3" }
---
# Stale decline notices

## Problem

Notices outlive their declines.

## Outcome

Self-retracting notices.

## ac-1

Retraction on staleness.

## ac-2

Channel completeness.

## ac-3

Audit visibility.
`

// exemptedADR is ADR_REF's target so the reference the board renders
// names a real artifact in the store.
const exemptedADR = `---
id: adr/0012-outbox-loansvc-events
kind: adr
title: "Synchronous writes for loansvc events"
status: accepted
owners: [platform-team]
decided: 2026-07-01
frozen: { at: 2026-07-01, commit: 1111111111111111111111111111111111111111 }
---
# Synchronous writes for loansvc events

The org-wide rule the refi-decline-flow fixture's dc-1 exempts itself
from (e2e fixture).
`

// cannedReviewFeed is REVIEW_SPEC's MR comment feed — the three routing
// cases of 02 §Record schemas' comment-token grammar (fixtures.ts:
// anchored, token-free, unresolvable-token). Served to `verdi serve`
// through workbench.LoadCannedCommentFeed (VERDI_REVIEW_FEED).
const cannedReviewFeed = `{
  "stale-decline-notices": [
    { "id": "n-1", "author": "alice", "body": "[vd:ac-2] this outcome AC reads as implementation-scoped — reword?", "resolved": false },
    { "id": "n-2", "author": "bob", "body": "overall direction looks right; one naming nit inline", "resolved": false },
    { "id": "n-3", "author": "carol", "body": "[vd:zz-99] does this still apply after the split?", "resolved": true }
  ]
}
`

// provisionBoardV2 sets the store up for the v1 board specs: a local
// bare origin (push target), the design branch carrying both draft
// specs plus the exempted ADR, and the canned review feed file. It runs
// AFTER the dex site is built, so the static site keeps reflecting
// main. Returns the feed file's path for the serve subprocess's env.
func provisionBoardV2(scratch, storeRoot string) (feedPath string, err error) {
	git := func(args ...string) error {
		cmd := exec.Command("git", args...)
		cmd.Dir = storeRoot
		out, gerr := cmd.CombinedOutput()
		if gerr != nil {
			return fmt.Errorf("git %v: %w\n%s", args, gerr, out)
		}
		return nil
	}

	// The board's commit affordance uses the checkout's own identity.
	if err := git("config", "user.name", "verdi-e2e"); err != nil {
		return "", err
	}
	if err := git("config", "user.email", "e2e@verdi.invalid"); err != nil {
		return "", err
	}
	if err := git("config", "commit.gpgsign", "false"); err != nil {
		return "", err
	}

	// A bare local origin makes "Commit & push" a real round-trip with no
	// network.
	originDir := filepath.Join(scratch, "origin.git")
	if out, oerr := exec.Command("git", "init", "--bare", "--quiet", "--initial-branch=main", originDir).CombinedOutput(); oerr != nil {
		return "", fmt.Errorf("git init --bare: %w\n%s", oerr, out)
	}
	if err := git("remote", "add", "origin", originDir); err != nil {
		return "", err
	}
	if err := git("push", "--quiet", "--set-upstream", "origin", "main"); err != nil {
		return "", err
	}

	// The design branch: both draft specs (draft never lands on main —
	// VL-004) plus the ADR the design spec's dc-1 exempts.
	if err := git("checkout", "--quiet", "-b", designBranch); err != nil {
		return "", err
	}
	files := map[string]string{
		filepath.Join(".verdi", "specs", "active", designSpecName, "spec.md"):     designSpec,
		filepath.Join(".verdi", "specs", "active", designSpecName, "layout.json"): designSpecLayout,
		filepath.Join(".verdi", "specs", "active", reviewSpecName, "spec.md"):     reviewSpec,
		filepath.Join(".verdi", "adr", "0012-outbox-loansvc-events.md"):           exemptedADR,
	}
	for rel, content := range files {
		path := filepath.Join(storeRoot, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return "", err
		}
	}
	if err := git("add", "-A"); err != nil {
		return "", err
	}
	if err := git("commit", "--quiet", "--no-verify", "-m", "design: refi-decline-flow + stale-decline-notices fixtures"); err != nil {
		return "", err
	}
	if err := git("push", "--quiet", "--set-upstream", "origin", designBranch); err != nil {
		return "", err
	}

	feedPath = filepath.Join(scratch, "review-feed.json")
	if err := os.WriteFile(feedPath, []byte(cannedReviewFeed), 0o644); err != nil {
		return "", err
	}
	return feedPath, nil
}
