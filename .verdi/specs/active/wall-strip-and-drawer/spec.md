---
id: spec/wall-strip-and-drawer
kind: spec
title: "Wall strip and drawer"
owners: [platform-team]
class: story
story: jira:VERDI-WR-4
problem: { text: "The case file stands as placards above the wall, committing gives no view of what will be committed, and the reading aids (provenance, review, context, repository details) are on-demand panels in a rail that print raw JSON.", anchor: problem }
outcome: { text: "A one-line case-file strip, a Commit and push that shows exactly what is uncommitted, a readiness pill, and a record drawer that renders every projection as prose and tables. The rail is gone and each of its items has a home.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "The case-file strip shows the problem and the outcome on one line each, each editable in place as one typed operation (set-problem or set-outcome; Enter applies, Escape cancels), with the full case file one action away. Case-file badges, flags, and disclosures render as chips in the strip.", evidence: [behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "The strip's chips keep every property parent dc-6 restates: card badges stay compact chips in the receipt-row vocabulary; badges render in every board mode and never block a write; each badge is a button carrying data-badge-source and its serialized derivation record, which opens the derivation drawer; the flags come from the same exported entry points through the badge compute layer, three-valued; ladder flags appear on story walls and size-smell on any wall that declares acceptance criteria; the flags wear the same names on every surface; and a disclosed-unproven value is drawn in the disclosure style, distinct from a flag.", evidence: [behavioral, static], anchor: ac-2 }
  - { id: ac-3, text: "Commit and push shows the uncommitted changes from the wall snapshot (wall-changes) in three distinct states: typed changes with their added, changed, or edited badges; unclassified changes; and an unreadable comparison, disclosed. A comparison with zero recognized operations never clears the uncommitted indicator while any other change remains.", evidence: [behavioral], anchor: ac-3 }
  - { id: ac-4, text: "The branch switcher is a menu on the branch text in the top bar and keeps the branch-switch guard. The readiness pill opens the drawer's Readiness tab, and a menu opens the other tabs, with counts loaded only when the menu opens.", evidence: [behavioral], anchor: ac-4 }
  - { id: ac-5, text: "The record drawer's tabs (Readiness, Provenance, Review, Context, Repo, Moves, and Keys) render their projections as prose and tables, never raw JSON, each with an honest empty state and the posture reason when a projection is unavailable. Clicking a Readiness item selects its card on the wall. The Review tab names the branch and the command that opens a pull request, and opens none.", evidence: [behavioral], anchor: ac-5 }
  - { id: ac-6, text: "The rail is gone and every item it held has a home: the review-mode inbox tray stays docked and visible in review mode; Revise and New story are the top bar's primary action on a sealed wall; Instantiate is on the stub card's toolbar; the policy setup guide and the wall-shell gap list are in the drawer's Readiness tab; and every readiness target that pointed at a rail anchor points at its new home.", evidence: [behavioral], anchor: ac-6 }
constraints:
  - { id: co-1, text: "Each drawer tab renders one existing on-demand projection, loaded when the tab opens; nothing is fetched to compute a count until the menu opens (parent co-7: provenance stays on demand).", anchor: co-1 }
  - { id: co-2, text: "The derivation drawer keeps its dialog semantics beside the record drawer, and the workbench performs no forge write (parent co-7).", anchor: co-2 }
decisions:
  - { id: dc-1, text: "The strip's badge and flag chips replace the superseded case-file stamps (parent dc-6, which supersedes badge-computes ac-5 and dc-4 and case-file-flags ac-1, dc-3, and dc-4). Every property parent dc-6 keeps is a criterion here (ac-2), so nothing kept is lost with the stamps.", anchor: dc-1 }
  - { id: dc-2, text: "The drawer replaces the ASD panels, the board guide, the yarn key, the readiness technical facts, and the on-wall readiness shell. The Moves tab rewrites the four-move guide for the new gestures, and the Keys tab carries the keyboard table and the yarn key.", anchor: dc-2 }
  - { id: dc-3, text: "This story consumes wall-changes' snapshot contract and adds no git read of its own. It owns the strip, rail, drawer, and dialog regions of boardspecrender.go (plan step 2, shared files).", anchor: dc-3 }
  - { id: dc-4, text: "The wall-shell versus loader gap list that readiness-recovery ac-5 assigns to the post-design lane lands in the drawer's Readiness tab (parent dc-7); the recovery view and attestation authoring stay out.", anchor: dc-4 }
links:
  - { type: implements, ref: "spec/workbench-redesign#ac-3" }
---
# Wall strip and drawer

## Problem

The case file stands as placards above the wall, committing gives no view of what will be committed, and the reading
aids (provenance, review, context, repository details) are on-demand panels in a rail that print raw JSON.

## Outcome

A one-line case-file strip, a Commit and push that shows exactly what is uncommitted, a readiness pill, and a record
drawer that renders every projection as prose and tables. The rail is gone and each of its items has a home.

## ac-1

The case-file strip shows the problem and the outcome on one line each, each editable in place as one typed operation
(set-problem or set-outcome; Enter applies, Escape cancels), with the full case file one action away. Case-file badges,
flags, and disclosures render as chips in the strip.

## ac-2

The strip's chips keep every property parent dc-6 restates: card badges stay compact chips in the receipt-row
vocabulary; badges render in every board mode and never block a write; each badge is a button carrying data-badge-source
and its serialized derivation record, which opens the derivation drawer; the flags come from the same exported entry
points through the badge compute layer, three-valued; ladder flags appear on story walls and size-smell on any wall that
declares acceptance criteria; the flags wear the same names on every surface; and a disclosed-unproven value is drawn in
the disclosure style, distinct from a flag.

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
unavailable. Clicking a Readiness item selects its card on the wall. The Review tab names the branch and the command
that opens a pull request, and opens none.

## ac-6

The rail is gone and every item it held has a home: the review-mode inbox tray stays docked and visible in review mode;
Revise and New story are the top bar's primary action on a sealed wall; Instantiate is on the stub card's toolbar; the
policy setup guide and the wall-shell gap list are in the drawer's Readiness tab; and every readiness target that
pointed at a rail anchor points at its new home.

## co-1

Each drawer tab renders one existing on-demand projection, loaded when the tab opens; nothing is fetched to compute a
count until the menu opens (parent co-7: provenance stays on demand).

## co-2

The derivation drawer keeps its dialog semantics beside the record drawer, and the workbench performs no forge write
(parent co-7).

## dc-1

The strip's badge and flag chips replace the superseded case-file stamps (parent dc-6, which supersedes badge-computes
ac-5 and dc-4 and case-file-flags ac-1, dc-3, and dc-4). Every property parent dc-6 keeps is a criterion here (ac-2), so
nothing kept is lost with the stamps.

## dc-2

The drawer replaces the ASD panels, the board guide, the yarn key, the readiness technical facts, and the on-wall
readiness shell. The Moves tab rewrites the four-move guide for the new gestures, and the Keys tab carries the keyboard
table and the yarn key.

## dc-3

This story consumes wall-changes' snapshot contract and adds no git read of its own. It owns the strip, rail, drawer,
and dialog regions of boardspecrender.go (plan step 2, shared files).

## dc-4

The wall-shell versus loader gap list that readiness-recovery ac-5 assigns to the post-design lane lands in the drawer's
Readiness tab (parent dc-7); the recovery view and attestation authoring stay out.
