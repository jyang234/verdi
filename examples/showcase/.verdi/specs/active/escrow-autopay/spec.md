---
id: spec/escrow-autopay
kind: spec
class: feature
title: "Escrow autopay enrollment"
status: accepted-pending-build
owners: [platform-team]
story: jira:LOAN-1519
problem: { text: "borrowers cannot self-serve an update to a submitted application", anchor: "#problem" }
outcome: { text: "a borrower can update their application and see the change reflected", anchor: "#outcome" }
impacts: [loansvc, notification-svc]
context:
  - adr/0002-outbox-events@f80b677cac43645416a4a1441a258234e2ef763d
declares:
  boundaries:
    - { from: loansvc, to: notification-svc, via: events }
acceptance_criteria:
  - { id: ac-1, text: "a borrower can update their application", evidence: [attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "a borrower can see the change reflected", evidence: [behavioral, attestation], anchor: "#ac-2" }
  - { id: ac-3, text: "support can audit every update", evidence: [static, attestation], anchor: "#ac-3" }
constraints:
  - { id: co-1, text: "must not touch the legacy schema", anchor: "#co-1" }
decisions:
  - { id: dc-1, text: "excuse this feature from ADR-0001's synchronous-write rule", anchor: "#dc-1",
      links: [ { type: exempts, ref: adr/0001-outbox-events, note: "outbox pattern already async by design" } ] }
  - { id: dc-2, text: "use the outbox pattern for update events", anchor: "#dc-2" }
open_questions:
  - { id: oq-1, text: "should the mobile app use PUT or PATCH for the update route?", anchor: "#oq-1" }
stubs:
  - { slug: borrower-update-api, acceptance_criteria: [ac-1] }
  - { slug: borrower-update-ui, acceptance_criteria: [ac-1, ac-2] }
  - { slug: borrower-update-audit-log, acceptance_criteria: [ac-3] }
frozen: { at: 2026-07-11, commit: 7248a3f6d1322f7df24a65b774ac334fd01e4274 }
---
# Accepted pending build (v2 fixture)

The v2 contract-surface fixture: a feature spec exercising the round-four
object model end to end (V1-P1). Text below exists only to give every
frontmatter-declared anchor a real heading to resolve against.

## Problem

Borrowers cannot self-serve an update to a submitted application; every
change requires a support ticket.

## Outcome

A borrower can update their application and see the change reflected in
their own view within the same session.

## AC-1

A borrower can update their application. Outcome-level, implementation-blind
— the outcome floor is an attestation, satisfied by a bound outcome
attestation artifact (`attestations/escrow-autopay/ac-1.md`).

## AC-2

A borrower can see the change reflected. Also outcome-level; satisfied by a
bound behavioral record plus the attestation floor (the outcome-level
evidence-record-via-sidecar-seam path described in this phase's fixture
design — the sidecar itself is evidence-model territory, out of this
phase's decode-only scope).

## AC-3

Support can audit every update. Outcome-level; the attestation floor
applies here too.

## CO-1

Must not touch the legacy schema.

## DC-1

Excuse this feature from ADR-0001's synchronous-write rule: the outbox
pattern is already asynchronous by design, so the rule does not bind here.

## DC-2

Use the outbox pattern for update events.

## OQ-1

Should the mobile app use PUT or PATCH for the update route? Spiked by
`spec/borrower-update-mobile-spike`'s `resolves` edge (R4-I-16, 02 §Object
model).
