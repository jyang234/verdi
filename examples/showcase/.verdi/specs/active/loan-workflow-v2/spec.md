---
id: spec/loan-workflow-v2
kind: spec
class: feature
title: "Loan workflow v2 (supersedes v1)"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "loan officers cannot see workflow status changes in real time", anchor: "#problem" }
outcome: { text: "loan officers see workflow status changes within thirty seconds", anchor: "#outcome" }
links:
  - { type: supersedes, ref: spec/loan-workflow }
acceptance_criteria:
  - { id: ac-1, text: "workflow status changes are visible within thirty seconds", evidence: [runtime, attestation], anchor: "#ac-1" }
  - { id: ac-3, text: "workflow status changes emit an audit event", evidence: [static, attestation], anchor: "#ac-3" }
constraints:
  - { id: co-1, text: "must not add new synchronous cross-service calls", anchor: "#co-1" }
supersession:
  carried: [co-1]
  amended: [ { id: ac-1, note: "tightened the visibility threshold from one minute to thirty seconds" } ]
  amended_advisory: []
  removed: [ { id: ac-2, note: "workflow-history query moved to a separate reporting feature" } ]
  added: [ac-3]
frozen: { at: 2026-07-10, commit: 06a3f4cabb226fe9344e1645e27c344493b6b62b }
---
# Loan workflow v2 (supersedes v1)

**Rung-4 supersession pair fixture, v2** (03 §The amendment ladder rung 4,
R4-I-4): `supersedes` v1, and the `supersession:` block above classifies
every one of v1's three objects (ac-1, ac-2, co-1) exactly once — `co-1`
carried (byte-identical text to v1's `co-1`, required by VL-015),
`ac-1` amended (tightened wording), `ac-2` removed, plus `ac-3` newly
added. `spec/borrower-update-mobile` (a story on `spec/accepted-pending-build`)
also carries an `implements` edge into `spec/loan-workflow#ac-1` — the
amended object — and files a re-affirmation
(`reaffirmations/jira-loan-1483/ac-1.md`) recording the old→new content
hash. See also the two VL-015 negative-case twins under
`testdata/violations/VL-015/`.

## Problem

Loan officers only see workflow status changes on their next manual
refresh.

## Outcome

Loan officers see workflow status changes within thirty seconds of the
change.

## AC-1

Workflow status changes are visible within thirty seconds.

## AC-3

Workflow status changes emit an audit event.

## CO-1

Must not add new synchronous cross-service calls.
