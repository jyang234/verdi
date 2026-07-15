---
id: spec/loan-workflow
kind: spec
class: feature
title: "Loan workflow (v2 fixture, supersession v1)"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "loan officers cannot see workflow status changes in real time", anchor: "#problem" }
outcome: { text: "loan officers see workflow status changes within one minute", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "workflow status changes are visible within one minute", evidence: [runtime, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "workflow history is queryable by loan id", evidence: [static, attestation], anchor: "#ac-2" }
constraints:
  - { id: co-1, text: "must not add new synchronous cross-service calls", anchor: "#co-1" }
frozen: { at: 2026-06-01, commit: b5117ecc69b6779ad75cde60d4aec206ece0950b }
---
# Loan workflow (v2 fixture, supersession v1)

**Rung-4 supersession pair fixture, v1** (03 §The amendment ladder rung 4):
frozen, later superseded by `spec/loan-workflow-v2` (below), which carries
the `supersession:` block classifying every one of this revision's objects
exactly once. `frozen.commit` is a real fixturegit-built commit — this
fixture's own small, dedicated git history (see
`internal/artifact/v2fixture_test.go`), not the v0 corpus's history.

Unlike `spec/rate-lock` (this corpus's other feature-rung supersession
pair, `status: superseded`), this predecessor's own `status` stays
`accepted-pending-build` rather than being flipped: the two fixtures
deliberately cover different points in the amendment ladder's lifecycle —
rate-lock demonstrates the terminal, already-flipped state a real accept
ritual produces, while this pair demonstrates a `supersession:` manifest
existing ahead of that flip (a real MR can be accepted with its
predecessor's status update landing in the same commit or a follow-up
one; VL-015's fidelity check does not itself require the flip to have
happened yet). `spec/borrower-update-mobile` still carries a live
`implements` edge into this revision's `ac-1` even though a successor
exists — exactly the `spec-stale`/cascade scenario
`internal/evidence/cascade.go` exists to detect (see its own
`reaffirmations/jira-loan-1483/ac-1.md` record).

## Problem

Loan officers only see workflow status changes on their next manual
refresh — a dispatcher checking a loan's stage has to reload the queue
page to notice anything moved, which on a busy morning means status
changes routinely go unnoticed for tens of minutes at a time.

## Outcome

Loan officers see workflow status changes within one minute of the
change, without reloading.

## AC-1

Workflow status changes are visible within one minute.

## AC-2

Workflow history is queryable by loan id.

## CO-1

Must not add new synchronous cross-service calls.
