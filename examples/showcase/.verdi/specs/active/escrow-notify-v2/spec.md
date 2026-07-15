---
id: spec/escrow-notify-v2
kind: spec
title: "Escrow notify v2 (fixture, supersedes escrow-notify)"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:ESCROW-2
problem: { text: "a borrower learns about an escrow shortfall only on their next statement", anchor: "#problem" }
outcome: { text: "a borrower is notified within an hour of an escrow shortfall", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "an escrow shortfall notifies the borrower within one hour", evidence: [behavioral], anchor: "#ac-1" }
links:
  - { type: implements, ref: "spec/rate-lock-v2#ac-1" }
  - { type: supersedes, ref: "spec/escrow-notify" }
frozen: { at: 2026-07-12, commit: 5507c6d963bd78d9eabed2324c3d380e678f891e }
---
# Escrow notify v2 (fixture, supersedes escrow-notify)

**Story-rung supersession fixture, v2** (spec/feature-supersession-state
dc-4). Supersedes `spec/escrow-notify`; its acceptance is what flips the
predecessor story's `status` to `superseded` (the rung-3 flip D-12 shipped).
It is the source of the predecessor's computed `superseded-by` backlink on
dex, exactly as the feature-rung `rate-lock-v2` pair is.

## Problem

A borrower learns about an escrow shortfall only on their next statement.

## Outcome

A borrower is notified within an hour of an escrow shortfall.

## AC-1

An escrow shortfall notifies the borrower within one hour.
