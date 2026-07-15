---
id: spec/escrow-autopay-v2
kind: spec
class: feature
title: "Escrow autopay v2 (open supersession MR fixture)"
status: draft
owners: [platform-team]
problem: { text: "a borrower who wants their escrow payment collected automatically has to call servicing to set it up, and every mandate change after that is a manual back-office edit", anchor: "#problem" }
outcome: { text: "a borrower can enroll an escrow account in autopay, edit the mandate themselves, and see the change reflected immediately, not merely within the session", anchor: "#outcome" }
links:
  - { type: supersedes, ref: spec/escrow-autopay }
acceptance_criteria:
  - { id: ac-1, text: "an autopay mandate is created against a submitted application's escrow account, tied to the payment method already on file", evidence: [static, behavioral, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "a borrower who edits an existing autopay mandate sees the change reflected in their account immediately, not merely within the session", evidence: [behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "a scheduled autopay charge that fails retries according to the declared retry policy instead of silently dropping", evidence: [static, behavioral], anchor: "#ac-3" }
constraints:
  - { id: co-1, text: "must not touch the legacy schema", anchor: "#co-1" }
supersession:
  carried: [ac-1, ac-3, co-1, dc-1, dc-2, oq-1]
  amended: [ { id: ac-2, note: "tightened reflection from within-the-session to immediate" } ]
  amended_advisory: []
  removed: []
  added: []
---
# Escrow autopay v2 (open supersession MR fixture)

The V1-P8 dex-overlay fixture: the candidate v2 spec a still-OPEN
supersession MR carries (03 §The amendment ladder: "the fold's input set
includes open supersession MRs"). Served only through the fake forge's
`FetchFileAtRef` at `.verdi/specs/active/escrow-autopay-v2/spec.md`
— never written into a store. Its manifest amends `ac-2` only, so the
`pending-supersession` flag lands exactly on the story whose edges touch
`ac-2` (`spec/borrower-update-mobile`) and not on the stub-matched
`spec/borrower-update-api`.

## Problem

A borrower who wants their escrow payment collected automatically has to
call servicing to set it up, and every mandate change after that is a
manual back-office edit.

## Outcome

A borrower can enroll an escrow account in autopay, edit the mandate
themselves, and see the change reflected immediately, not merely within
the session.

## AC-1

Unchanged from v1.

## AC-2

Tightened: immediate reflection, not merely within the session — the
proposal drops the session-scoped window v1 accepted and requires the
reflected state to be visible the moment the mandate write commits.

## AC-3

Unchanged from v1.

## CO-1

Unchanged from v1.
