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
  - { type: implements, ref: "spec/rate-lock#ac-1" }
frozen: { at: 2026-07-11, commit: 7248a3f6d1322f7df24a65b774ac334fd01e4274 }
---
# Escrow notify (fixture, superseded story)

**Story-rung supersession fixture, v1** (spec/feature-supersession-state
dc-4). It mirrors the corpus's real superseded story `spec/disclosure-seam`
(flipped to `superseded` when `disclosure-seam-v2` was accepted): superseded
when its own successor `spec/escrow-notify-v2` was accepted. It exists so the
board and dex surfaces can be proven to render the terminal `superseded` state
at the STORY rung, hermetically.

## Problem

A borrower learns about an escrow shortfall only on their next statement.

## Outcome

A borrower is notified within a day of an escrow shortfall.

## AC-1

An escrow shortfall notifies the borrower within 24 hours.
