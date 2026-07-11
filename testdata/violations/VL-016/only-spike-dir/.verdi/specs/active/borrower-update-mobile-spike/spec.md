---
id: spec/borrower-update-mobile-spike
kind: spec
class: story
title: "Borrower update, mobile app: PUT vs PATCH spike"
status: draft
owners: [platform-team]
problem: { text: "we don't know whether the mobile update route should be PUT or PATCH", anchor: "#problem" }
outcome: { text: "a recommendation with tradeoffs recorded", anchor: "#outcome" }
spike: true
story: jira:LOAN-1484
links:
  - { type: resolves, ref: "spec/stale-decline#ac-1" }
---
# Borrower update, mobile app: PUT vs PATCH spike

**Spike variant fixture** (02 §Kind registry: "Spike variant"), sibling to
`spec/borrower-update-mobile` (the deviating story above): `spike: true`,
≥1 `resolves` edge to an open-question fragment (here targeting
`spec/stale-decline#ac-1` purely so this fixture resolves cleanly inside
`internal/lint`'s own fixturegit corpus, which declares no open_questions
of its own — VL-003 does not enforce that a resolves edge's target is
specifically an open-question object, only that the fragment resolves),
no `implements` edges — E3. Exempt from the evidence model and path-fenced
from product source (03 §Ceremony pricing, VL-016) — see the path-fence
violation twin under `testdata/violations/VL-016/touched-outside-fence/`.

## Problem

We don't know whether the mobile update route should be PUT or PATCH.

## Outcome

A recommendation with tradeoffs recorded.
