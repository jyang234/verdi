---
id: spec/borrower-update-mobile-spike
kind: spec
class: story
title: "Borrower update, mobile app: PUT vs PATCH spike"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "we don't know whether the mobile update route should be PUT or PATCH", anchor: "#problem" }
outcome: { text: "a recommendation with tradeoffs recorded", anchor: "#outcome" }
spike: true
story: jira:LOAN-1484
links:
  - { type: resolves, ref: "spec/accepted-pending-build#oq-1" }
frozen: { at: 2026-07-12, commit: 5507c6d963bd78d9eabed2324c3d380e678f891e }
---
# Borrower update, mobile app: PUT vs PATCH spike

**Spike variant fixture** (02 §Kind registry: "Spike variant"), sibling to
`spec/borrower-update-mobile` (the deviating story above): `spike: true`,
≥1 `resolves` edge to an open-question fragment, no `implements` edges — E3.
Exempt from the evidence model and path-fenced from product source
(03 §Ceremony pricing, VL-016) — see the path-fence violation twin under
`testdata/violations/VL-016/`.

## Problem

We don't know whether the mobile update route should be PUT or PATCH.

## Outcome

A recommendation with tradeoffs recorded.
