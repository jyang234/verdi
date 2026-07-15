---
id: spec/refi-rate-check-2024
kind: spec
class: story
title: "Refinance rate check 2024 (fixture, closed round-four story)"
status: closed
owners: [platform-team]
problem: { text: "refinance rate changes were verified by hand before each rollout", anchor: "#problem" }
outcome: { text: "rate changes are verified automatically against the published table", anchor: "#outcome" }
story: jira:LOAN-2024
links:
  - { type: implements, ref: "spec/loan-refi-2023#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "a rate change is verified against the published table before rollout", evidence: [static, behavioral], anchor: "#ac-1" }
frozen: { at: 2026-07-01, commit: 16219044c9d6d41de9a0de9464ed24d49283b40c }
---
# Refinance rate check 2024

Days after `spec/loan-refi-2023` shipped the first automated rate check,
the published-table format changed underneath it — a new column for
promotional-rate expirations broke the parser's column-position
assumptions and let two stale promotional rates through before anyone
noticed. This story rebuilds the check against the new table format and
closes it as a round-four story rather than the grandfathered v0 shape
its predecessor used, so it archives `layout.json` in the board-artifact
slot instead of a frozen `board.json` (00 §Glossary "the quartet": "new
specs archive layout.json … in place of v0's frozen board.json") — the
dex by-story axis renders this quartet's board slot from that coordinate
sidecar, while `loan-refi-2023` keeps exercising the grandfathered
`board.json` form it closed under.

## Problem

Refinance rate changes were verified by hand before each rollout.

## Outcome

Rate changes are verified automatically against the published table.

## AC-1

A rate change is verified against the published table before rollout.
