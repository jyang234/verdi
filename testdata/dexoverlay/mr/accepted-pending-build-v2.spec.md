---
id: spec/accepted-pending-build-v2
kind: spec
class: feature
title: "Accepted pending build v2 (open supersession MR fixture)"
status: draft
owners: [platform-team]
problem: { text: "borrowers cannot self-serve an update to a submitted application", anchor: "#problem" }
outcome: { text: "a borrower updates their application and sees the change reflected immediately", anchor: "#outcome" }
links:
  - { type: supersedes, ref: spec/accepted-pending-build }
acceptance_criteria:
  - { id: ac-1, text: "a borrower can update their application", evidence: [attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "a borrower sees the change reflected immediately, not merely within the session", evidence: [behavioral, attestation], anchor: "#ac-2" }
  - { id: ac-3, text: "support can audit every update", evidence: [static, attestation], anchor: "#ac-3" }
constraints:
  - { id: co-1, text: "must not touch the legacy schema", anchor: "#co-1" }
supersession:
  carried: [ac-1, ac-3, co-1, dc-1, dc-2, oq-1]
  amended: [ { id: ac-2, note: "tightened reflection from within-the-session to immediate" } ]
  amended_advisory: []
  removed: []
  added: []
---
# Accepted pending build v2 (open supersession MR fixture)

The V1-P8 dex-overlay fixture: the candidate v2 spec a still-OPEN
supersession MR carries (03 §The amendment ladder: "the fold's input set
includes open supersession MRs"). Served only through the fake forge's
`FetchFileAtRef` at `.verdi/specs/active/accepted-pending-build-v2/spec.md`
— never written into a store. Its manifest amends `ac-2` only, so the
`pending-supersession` flag lands exactly on the story whose edges touch
`ac-2` (`spec/borrower-update-mobile`) and not on the stub-matched
`spec/borrower-update-api`.

## Problem

Borrowers cannot self-serve an update to a submitted application.

## Outcome

A borrower updates their application and sees the change reflected
immediately.

## AC-1

Unchanged from v1.

## AC-2

Tightened: immediate reflection.

## AC-3

Unchanged from v1.

## CO-1

Unchanged from v1.
