---
id: spec/accepted-pending-build
kind: spec
class: feature
title: "Accepted pending build (v2 fixture)"
status: accepted-pending-build
owners: [platform-team]
story: okr:LOAN-Q3
bogus_extra_field: surprise
problem: { text: "borrowers cannot self-serve an update to a submitted application", anchor: "#problem" }
outcome: { text: "a borrower can update their application and see the change reflected", anchor: "#outcome" }
impacts: [loansvc, notification-svc]
context:
  - adr/0002-outbox-events@c5e360a9ee5e9eb6089e54b772fa16959ada4662
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
stubs:
  - { slug: borrower-update-api, acceptance_criteria: [ac-1] }
  - { slug: borrower-update-ui, acceptance_criteria: [ac-1, ac-2] }
  - { slug: borrower-update-audit-log, acceptance_criteria: [ac-3] }
frozen: { at: 2026-07-11, commit: 93ddc5bbbb398cf747151e1c466afb83114398df }
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
attestation artifact (`attestations/accepted-pending-build/ac-1.md`).

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
