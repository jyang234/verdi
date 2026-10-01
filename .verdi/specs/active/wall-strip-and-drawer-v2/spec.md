---
id: spec/wall-strip-and-drawer-v2
kind: spec
title: "Wall strip and drawer"
owners: [platform-team]
class: story
story: jira:VERDI-WR-4
problem: { text: "The case file stands as placards above the wall, committing gives no view of what will be committed, and the reading aids (provenance, review, context, repository details) are on-demand panels in a rail that print raw JSON.", anchor: problem }
outcome: { text: "A one-line case-file strip, a Commit and push that shows exactly what is uncommitted, a readiness pill, and a record drawer that renders every projection as prose and tables. The rail is gone and each of its items has a home.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "The case-file strip shows the problem and the outcome on one line each, each editable in place as one typed operation (set-problem or set-outcome; Enter applies, Escape cancels), with the full case file one action away. Case-file badges, flags, and disclosures render as chips in the strip.", evidence: [behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "The strip keeps every property parent dc-6 restates: card badges stay compact chips in the card's receipt-row vocabulary; badges render in every board mode and never block a write; every badge element is a button carrying data-badge-source and its serialized derivation record, the derivation drawer's opener; the flags are computed by the same exported entry points the dex story-lens calls, through the badge compute layer, three-valued (flagged with a witness, proven unflagged, or disclosed unproven when open merge requests cannot be enumerated); ladder flags appear on story walls and size-smell on any spec wall that declares acceptance criteria; the flags wear the same names on every surface (spec-stale, pending-supersession); and a disclosed-unproven value is drawn in the disclosure style, distinct from a flag, never dressed as a verdict. Disclosures render as chips in the strip in that disclosure style.", evidence: [behavioral, static], anchor: ac-2 }
  - { id: ac-3, text: "Commit and push shows the uncommitted changes from the wall snapshot (wall-changes) in three distinct states: typed changes with their added, changed, or edited badges; unclassified changes; and an unreadable comparison, disclosed. A comparison with zero recognized operations never clears the uncommitted indicator while any other change remains.", evidence: [behavioral], anchor: ac-3 }
  - { id: ac-4, text: "The branch switcher is a menu on the branch text in the top bar and keeps the branch-switch guard. The readiness pill opens the drawer's Readiness tab, and a menu opens the other tabs, with counts loaded only when the menu opens.", evidence: [behavioral], anchor: ac-4 }
  - { id: ac-5, text: "The record drawer's tabs (Readiness, Provenance, Review, Context, Repo, Moves, and Keys) render their projections as prose and tables, never raw JSON, each with an honest empty state and the posture reason when a projection is unavailable. The Readiness tab keeps guidance first: each item's guidance sentence is its primary line, with its fact, timing, and blocking flag in the technical disclosure (parent dc-8; spec-documents ac-12). Clicking a Readiness item selects its card on the wall. The Review tab names the branch and the command that opens a pull request, and opens none.", evidence: [behavioral], anchor: ac-5 }
  - { id: ac-6, text: "The rail is gone and every item it held has a home: the review-mode inbox tray stays docked and visible in review mode; Revise and New story are the top bar's primary action on a sealed wall; Instantiate is on the stub card's toolbar; the policy setup guide is in the drawer's Readiness tab; and every readiness target that pointed at a rail anchor points at its new home.", evidence: [behavioral], anchor: ac-6 }
  - { id: ac-7, text: "The drawer's Readiness tab renders the per-request readiness facts the readiness page renders, so the wall shell's own derivation is retired; the committed wall-shell versus loader parity witness (readiness-recovery ac-5) then shows no gap for the e2e harness fixture, each listed gap closed rather than hidden.", evidence: [static], anchor: ac-7 }
constraints:
  - { id: co-1, text: "Each drawer tab renders one existing on-demand projection, loaded when the tab opens; nothing is fetched to compute a count until the menu opens (parent co-7: provenance stays on demand).", anchor: co-1 }
  - { id: co-2, text: "The derivation drawer keeps its dialog semantics beside the record drawer, and the workbench performs no forge write (parent co-7).", anchor: co-2 }
decisions:
  - { id: dc-1, text: "The strip's badge and flag chips replace the superseded case-file stamps (parent dc-6, which supersedes badge-computes ac-5 and dc-4 and case-file-flags ac-1, dc-3, and dc-4). Every property parent dc-6 keeps is restated verbatim in ac-2, so nothing kept is lost with the stamps. The case-file stamp that derivation-drawer ac-1 names as an opener is now the case-file badge chip in the strip: activating it opens the derivation drawer, and derivation-drawer's evidence locates the chip while still proving that behavior (parent co-6).", anchor: dc-1 }
  - { id: dc-2, text: "The drawer replaces the ASD panels, the board guide, the yarn key, and the on-wall readiness shell. The readiness technical facts move into the Readiness tab's technical disclosure, guidance first (parent dc-8), and the repository facts into the Repo tab. The Moves tab rewrites the four-move guide for the new gestures and describes parent dc-13's kept gestures as they are, and the Keys tab carries the keyboard table and the yarn key.", anchor: dc-2 }
  - { id: dc-3, text: "This story consumes wall-changes' snapshot contract and adds no git read of its own. It owns the strip, rail, drawer, and dialog regions of boardspecrender.go (plan step 2, shared files).", anchor: dc-3 }
  - { id: dc-4, text: "Parent dc-7 absorbs the wall-shell versus loader gap list that readiness-recovery ac-5 hands to the post-design lane. This story absorbs it by retiring the wall shell's own derivation: the Readiness tab renders the same per-request readiness facts as the readiness page (readiness-page dc-2), so each listed gap closes, and none is reported as parity until the witness proves it (ac-7). The recovery view and attestation authoring stay out.", anchor: dc-4 }
links:
  - { type: implements, ref: "spec/workbench-redesign#ac-3" }
  - { type: supersedes, ref: "spec/wall-strip-and-drawer" }
---
# Wall strip and drawer

## Problem

The case file stands as placards above the wall, committing gives no view of what will be committed, and the reading
aids (provenance, review, context, repository details) are on-demand panels in a rail that print raw JSON.

## Outcome

A one-line case-file strip, a Commit and push that shows exactly what is uncommitted, a readiness pill, and a record
drawer that renders every projection as prose and tables. The rail is gone and each of its items has a home.

## What changed from v1

v2 revises spec/wall-strip-and-drawer through rung 3 of the amendment ladder
(conflict/wall-strip-and-drawer-playwright-producers). v1 declared its 6 Playwright-backed behavioral obligations as
unresolved design debt (SI-288), because no producer grammar existed for a Playwright test, and GLG v3 keeps design debt
from crossing verdi build start. The Playwright producer now exists (SI-292 to SI-294), so each of those obligations
names its test as an elaborated producer, titled `wall-strip-and-drawer › <obligation title>` in the file v1 named. The
criteria, constraints, and decisions are v1's, unchanged.

## ac-1

The case-file strip shows the problem and the outcome on one line each, each editable in place as one typed operation
(set-problem or set-outcome; Enter applies, Escape cancels), with the full case file one action away. Case-file badges,
flags, and disclosures render as chips in the strip.

## ac-2

The strip keeps every property parent dc-6 restates: card badges stay compact chips in the card's receipt-row
vocabulary; badges render in every board mode and never block a write; every badge element is a button carrying
data-badge-source and its serialized derivation record, the derivation drawer's opener; the flags are computed by the
same exported entry points the dex story-lens calls, through the badge compute layer, three-valued (flagged with a
witness, proven unflagged, or disclosed unproven when open merge requests cannot be enumerated); ladder flags appear on
story walls and size-smell on any spec wall that declares acceptance criteria; the flags wear the same names on every
surface (spec-stale, pending-supersession); and a disclosed-unproven value is drawn in the disclosure style, distinct
from a flag, never dressed as a verdict. Disclosures render as chips in the strip in that disclosure style.

## ac-3

Commit and push shows the uncommitted changes from the wall snapshot (wall-changes) in three distinct states: typed
changes with their added, changed, or edited badges; unclassified changes; and an unreadable comparison, disclosed. A
comparison with zero recognized operations never clears the uncommitted indicator while any other change remains.

## ac-4

The branch switcher is a menu on the branch text in the top bar and keeps the branch-switch guard. The readiness pill
opens the drawer's Readiness tab, and a menu opens the other tabs, with counts loaded only when the menu opens.

## ac-5

The record drawer's tabs (Readiness, Provenance, Review, Context, Repo, Moves, and Keys) render their projections as
prose and tables, never raw JSON, each with an honest empty state and the posture reason when a projection is
unavailable. The Readiness tab keeps guidance first: each item's guidance sentence is its primary line, with its fact,
timing, and blocking flag in the technical disclosure (parent dc-8; spec-documents ac-12). Clicking a Readiness item
selects its card on the wall. The Review tab names the branch and the command that opens a pull request, and opens none.

## ac-6

The rail is gone and every item it held has a home: the review-mode inbox tray stays docked and visible in review mode;
Revise and New story are the top bar's primary action on a sealed wall; Instantiate is on the stub card's toolbar; the
policy setup guide is in the drawer's Readiness tab; and every readiness target that pointed at a rail anchor points at
its new home.

## ac-7

The drawer's Readiness tab renders the per-request readiness facts the readiness page renders, so the wall shell's own
derivation is retired; the committed wall-shell versus loader parity witness (readiness-recovery ac-5) then shows no gap
for the e2e harness fixture, each listed gap closed rather than hidden.

## co-1

Each drawer tab renders one existing on-demand projection, loaded when the tab opens; nothing is fetched to compute a
count until the menu opens (parent co-7: provenance stays on demand).

## co-2

The derivation drawer keeps its dialog semantics beside the record drawer, and the workbench performs no forge write
(parent co-7).

## dc-1

The strip's badge and flag chips replace the superseded case-file stamps (parent dc-6, which supersedes badge-computes
ac-5 and dc-4 and case-file-flags ac-1, dc-3, and dc-4). Every property parent dc-6 keeps is restated verbatim in ac-2,
so nothing kept is lost with the stamps. The case-file stamp that derivation-drawer ac-1 names as an opener is now the
case-file badge chip in the strip: activating it opens the derivation drawer, and derivation-drawer's evidence locates
the chip while still proving that behavior (parent co-6).

## dc-2

The drawer replaces the ASD panels, the board guide, the yarn key, and the on-wall readiness shell. The readiness
technical facts move into the Readiness tab's technical disclosure, guidance first (parent dc-8), and the repository
facts into the Repo tab. The Moves tab rewrites the four-move guide for the new gestures and describes parent dc-13's
kept gestures as they are, and the Keys tab carries the keyboard table and the yarn key.

## dc-3

This story consumes wall-changes' snapshot contract and adds no git read of its own. It owns the strip, rail, drawer,
and dialog regions of boardspecrender.go (plan step 2, shared files).

## dc-4

Parent dc-7 absorbs the wall-shell versus loader gap list that readiness-recovery ac-5 hands to the post-design lane.
This story absorbs it by retiring the wall shell's own derivation: the Readiness tab renders the same per-request
readiness facts as the readiness page (readiness-page dc-2), so each listed gap closes, and none is reported as parity
until the witness proves it (ac-7). The recovery view and attestation authoring stay out.
