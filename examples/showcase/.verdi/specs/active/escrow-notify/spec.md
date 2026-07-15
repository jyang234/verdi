---
id: spec/escrow-notify
kind: spec
title: "Escrow notify (fixture, superseded story)"
owners: [platform-team]
class: story
status: superseded
story: jira:ESCROW-1
problem: { text: "a borrower learns about an escrow shortfall only on their next statement", anchor: "#problem" }
outcome: { text: "a borrower is notified within a day of an escrow shortfall", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "an escrow shortfall notifies the borrower within 24 hours", evidence: [behavioral], anchor: "#ac-1" }
links:
  - { type: implements, ref: "spec/stale-decline#ac-4" }
frozen: { at: 2026-07-11, commit: 16219044c9d6d41de9a0de9464ed24d49283b40c }
---
# Escrow notify (fixture, superseded story)

**Story-rung supersession fixture, v1** (spec/feature-supersession-state
dc-4). It mirrors the corpus's real superseded story `spec/disclosure-seam`
(flipped to `superseded` when `disclosure-seam-v2` was accepted): superseded
when its own successor `spec/escrow-notify-v2` was accepted. Its
`implements` edge into `spec/stale-decline#ac-4` is a story-must-implement-
some-feature-AC structural requirement (02 §Kind registry), not a
narrative claim — this fixture's own supersession story is complete
without one (public-rollout-plan Task 1.5: previously targeted
`spec/rate-lock#ac-1`, retargeted once that pair moved to its own
dedicated fixturegit history — see `spec/rate-lock`'s own note).

## Why superseded

A 24-hour notification window was fast enough to ship, but support kept
seeing the same complaint: a borrower who called in about a shortfall
inside that window was told "you'll hear about this soon" by an agent who
already had the shortfall data on screen. `spec/escrow-notify-v2` tightens
the window to one hour, closing the gap between the system already
knowing and the borrower being told.

## Problem

A borrower learns about an escrow shortfall only on their next statement.

## Outcome

A borrower is notified within a day of an escrow shortfall.

## AC-1

An escrow shortfall notifies the borrower within 24 hours.
