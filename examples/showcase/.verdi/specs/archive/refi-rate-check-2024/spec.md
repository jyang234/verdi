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
frozen: { at: 2026-07-01, commit: 5507c6d963bd78d9eabed2324c3d380e678f891e }
---
# Refinance rate check 2024

The V1-P8 dex-overlay fixture: a CLOSED round-four story archived as the
round-four quartet — spec, `layout.json` in the board-artifact slot
(00 §Glossary "the quartet": "new specs archive layout.json … in place of
v0's frozen board.json"), rollup, deviation report. The dex by-story axis
renders this quartet's board slot from the coordinate sidecar, while
`loan-refi-2023` (testdata/corpus) keeps exercising the grandfathered
`board.json` form.

## Problem

Refinance rate changes were verified by hand before each rollout.

## Outcome

Rate changes are verified automatically against the published table.

## AC-1

A rate change is verified against the published table before rollout.
