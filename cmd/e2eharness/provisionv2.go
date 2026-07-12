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
	emptySpecName  = "income-verification"
)

// designSpec is DESIGN_SPEC: the object model fixtures.ts binds (3 ACs,
// 1 constraint, dc-1 carrying the declared exempts edge to ADR_REF, dc-2
// plain, and oq-1 — the open question the scoping-canvas spike journey
// draws its resolution yarn to), with problem/outcome texts containing
// PROBLEM_SNIPPET ("stale decline") and OUTCOME_SNIPPET ("declined
// applicants").
//
// The problem text is deliberately long (several sentences): at the e2e
// viewport its case-file placard overflows the 3-line clamp, so it is the
// fixture for the board's click-to-expand affordance (33-board-expand) —
// a truncated placard shows the hint and opens the read-only expand
// dialog. Its "## Problem" body section is intentionally EMPTY, so the
// problem placard carries no hidden placard-full and its dialog falls back
// to the (long) headline text — the no-body path. Its "## Outcome" body,
// by contrast, is a RICHER-THAN-THE-HEADLINE section (a paragraph, a
// bulleted list, emphasis): the outcome placard's headline is short (it
// does NOT clamp at the wide e2e viewport), yet the placard is still
// expandable and its dialog renders that body HTML — the board-polish
// pass's always-expandable + show-body behavior, and the width-independence
// proof. EMPTY_SPEC (income-verification) keeps a short one-line problem
// headline AND an empty body section: no body, no clamp — the degenerate
// case (a short placard gets no affordance).
const designSpec = `---
id: spec/refi-decline-flow
kind: spec
class: feature
title: "Refinancing decline flow"
status: draft
owners: [platform-team]
problem: { text: "applicants keep acting on stale decline reasons after the underlying data changes, and the cost of that gap compounds at every touchpoint: they re-apply against a rule that no longer holds, they call support to contest a decision that has already been reversed, and some abandon the product entirely in the belief that a hard block still stands. Each of those paths generates avoidable rework for the servicing team, and every repetition erodes the applicant's trust that the decline they were shown is the decline that is actually real.", anchor: "#problem" }
outcome: { text: "declined applicants see the current decline state and a concrete next step", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a declined applicant sees the current decline reason within a minute of a data change", evidence: [behavioral, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "a reversed decline clears the notice everywhere it was shown", evidence: [behavioral, attestation], anchor: "#ac-2" }
  - { id: ac-3, text: "support can audit every decline notice ever shown", evidence: [static, attestation], anchor: "#ac-3" }
constraints:
  - { id: co-1, text: "decline notices never expose internal model scores", anchor: "#co-1" }
decisions:
  - { id: dc-1, text: "excuse decline events from the synchronous-write rule", anchor: "#dc-1",
      links: [ { type: exempts, ref: adr/0001-outbox-events, note: "decline events are already async via the outbox" } ] }
  - { id: dc-2, text: "reuse the existing notification channel for decline updates", anchor: "#dc-2" }
open_questions:
  - { id: oq-1, text: "which decline reasons can legally be shown verbatim?", anchor: "#oq-1" }
---
# Refinancing decline flow

## Problem

## Outcome

The fix is a single source of decline truth: a borrower, a support agent, and
the audit log all read the *same* current decline state, sourced from the
servicing system of record rather than any cached copy. Concretely:

- a **reversal** propagates to every surface that showed the original
  decline, inside one refresh window;
- a **stale** decline retracts itself instead of standing until a human
  happens to notice;
- every notice ever shown stays **audit-visible**, so support can
  reconstruct exactly what the borrower saw, and when.

The wall below exists to close that gap.

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

## oq-1

Legal review pending.
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

// emptySpec is EMPTY_SPEC (fixtures.ts): the leanest VALID draft on the
// design branch — a story spec (class story requires problem, outcome,
// a tracker ref, and >=1 implements edge; no class permits zero of
// everything) with NOT ONE declared object. The newcomer's first board:
// its wall holds only the implements thread and must render the
// teaching empty-wall state rather than a void (the board-legibility
// contract). The implements target is the v2 corpus feature's ac-1,
// real on main.
const emptySpec = `---
id: spec/income-verification
kind: spec
class: story
title: "Income verification"
status: draft
owners: [platform-team]
story: jira:LOAN-2201
problem: { text: "income documents are verified by hand and applicants wait days", anchor: "#problem" }
outcome: { text: "verification completes the day the documents arrive", anchor: "#outcome" }
links:
  - { type: implements, ref: spec/accepted-pending-build#ac-1 }
---
# Income verification

## Problem

## Outcome
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
problem: { text: "decline notices linger long after the decline that produced them has gone stale, and because the mirror board that reviews this work is a non-authoring room, its placards must clamp and expand exactly like the live wall does: a reviewer who opens this spec sees the same three-line case file, the same fade-and-mark on an overflowing problem, and the same read-only expand dialog on a click — nothing about legibility depends on which room you are standing in.", anchor: "#problem" }
outcome: { text: "notices retract themselves when a decline goes stale", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a stale decline retracts its notice", evidence: [behavioral, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "the retraction reaches every notified channel", evidence: [behavioral, attestation], anchor: "#ac-2" }
  - { id: ac-3, text: "a retracted notice is audit-visible", evidence: [static, attestation], anchor: "#ac-3" }
---
# Stale decline notices

## Problem

## Outcome

## ac-1

Retraction on staleness.

## ac-2

Channel completeness.

## ac-3

Audit visibility.
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
	// VL-004); the ADR dc-1 exempts is the corpus's own adr/0001-outbox-events.
	if err := git("checkout", "--quiet", "-b", designBranch); err != nil {
		return "", err
	}
	// ADR_REF's target (adr/0001-outbox-events, V1-P8's fixtures.ts
	// finalization) is the corpus's own real ADR — already on main and so
	// on this branch; nothing to author here.
	files := map[string]string{
		filepath.Join(".verdi", "specs", "active", designSpecName, "spec.md"):     designSpec,
		filepath.Join(".verdi", "specs", "active", designSpecName, "layout.json"): designSpecLayout,
		filepath.Join(".verdi", "specs", "active", reviewSpecName, "spec.md"):     reviewSpec,
		filepath.Join(".verdi", "specs", "active", emptySpecName, "spec.md"):      emptySpec,
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
