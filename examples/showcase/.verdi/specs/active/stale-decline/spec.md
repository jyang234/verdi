---
id: spec/stale-decline
kind: spec
class: feature
title: "Stale decline handling (fixture)"
status: accepted-pending-build
owners: [platform-team]
story: jira:LOAN-1482
problem: { text: "a loan-payment decline can reach notification-svc and payments-gw after the borrower's underlying state has already moved on — a retried charge that cleared, an escrow adjustment, a same-day payoff — so acting on the decline as if it were still current risks a duplicate notice and a charge retry against a balance that no longer needs it", anchor: "#problem" }
outcome: { text: "loansvc detects a stale decline before dispatching its consequences, retries the charge exactly once through the outbox, and only notifies the borrower when the decline still reflects their current account state", anchor: "#outcome" }
links:
  - { type: implements, ref: adr/0002-outbox-events }
  - { type: story, ref: jira:LOAN-1482 }
  - { type: impacts, ref: svc/loansvc/boundary-contract }
impacts: [loansvc, notification-svc, payments-gw]
context:
  - adr/0002-outbox-events@66588948af8b36c02c8fb8f423645afa0a58dbe4
declares:
  boundaries:
    - { from: loansvc, to: notification-svc, via: events }
    - { from: loansvc, to: payments-gw, via: events }
acceptance_criteria:
  - { id: ac-1, text: "every branch that classifies a decline as stale routes its consequence through the outbox — no direct call to notification-svc or payments-gw", evidence: [static], anchor: "#ac-1" }
  - { id: ac-2, text: "loansvc retries the charge through the outbox exactly once per stale decline", evidence: [static, behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "a partial refund against a stale-declined loan still reconciles correctly before any retried charge is issued", evidence: [behavioral], anchor: "#ac-3" }
  - { id: ac-4, text: "the stale-decline rate for the affected cohort is checked against the pre-change baseline seven days post-deploy", evidence: [runtime], anchor: "#ac-4" }
open_questions:
  - { id: oq-1, text: "should partial refunds share the stale-decline retry budget?", anchor: "#oq-1" }
dispositions:
  - { sticky: a-01J8Z0K3AAAAAAAAAAAAAAAAAA, disposition: incorporated, where: "#design-notes" }
  - { sticky: a-01J8Z0K4BBBBBBBBBBBBBBBBBB, disposition: contradicted, note: "partial refunds are out of scope for this story; tracked separately as oq-1" }
  - { sticky: a-01J8Z0K5CCCCCCCCCCCCCCCCCC, disposition: open-question }
stubs:
  - { slug: borrower-update-api, acceptance_criteria: [ac-2] }
  - { slug: borrower-update-mobile, acceptance_criteria: [ac-1, ac-3] }
frozen: { at: 2026-05-14, commit: 66588948af8b36c02c8fb8f423645afa0a58dbe4 }
---
# Stale decline handling

## Problem

A loan-payment decline can reach notification-svc and payments-gw after the
borrower's underlying state has already moved on — a retried charge that
cleared, an escrow adjustment landing first, a same-day payoff. Acting on a
decline as if it were still current risks a duplicate notice to the
borrower and a charge retry against a balance that no longer needs it.

## Outcome

loansvc detects a stale decline before dispatching its consequences,
retries the charge exactly once through the outbox, and the decline
notification only reaches the borrower when it still reflects their true
account state.

## Design notes

Every stale-decline consequence is routed through the outbox pattern
(adr/0002) instead of a synchronous call issued from inside the same
request that classified the decline as stale. The write that flags a
decline stale and the outbox row that will eventually retry the charge or
notify the borrower commit atomically, in one transaction; a background
publisher drains the outbox at least once, so the consequence is never
lost, but also never fired twice from the same evaluation.

That design exists because of the 2025-10 dual-write incident: during a
loansvc failover, notification-svc dispatched duplicate decline notices to
borrowers, because the pre-outbox code path wrote the decline row and
published the notification event as two independent operations with no
shared transaction — the failover replayed the write without any guarantee
about whether the event had actually gone out. adr/0002 (which supersedes
adr/0001) closed that gap for every publisher, not just this one; this
feature is the second consumer of the pattern after loan-refi-2023, and its
own golden path (ac-2) is exactly the charge retry that pattern exists to
make safe.

## Boundaries

Two service boundaries carry stale-decline's consequences, and both go
through the outbox rather than a synchronous call:

- loansvc -> notification-svc, via events: the borrower- and staff-facing
  decline notice is an outbox event. notification-svc consumes it
  idempotently — each event carries the decline id it dedupes on — so a
  redelivered event never produces a second notice, which is precisely the
  failure mode the 2025-10 incident produced without this guarantee.
- loansvc -> payments-gw, via events: the retried charge attempt is
  likewise outbox-driven, not a direct synchronous payments-gw call. A
  synchronous retry issued from the same request that just detected the
  stale decline would reintroduce the identical dual-write risk on the
  payment side instead of the notification side — the reason both
  boundaries are declared above, so the alignment report's computed
  section can diff the built system against this plan (03 §Alignment
  report).

## AC-1

The retry path's own obligation holds statically: every branch that
decides a decline is stale routes its consequence through the outbox
helper — no direct call to notification-svc or payments-gw exists
anywhere in the stale-decline code path.

## AC-2

loansvc retries the charge API through the outbox when a decline is judged
stale, and the retried charge reaches payments-gw exactly once per decline
(static: the call is outbox-routed; behavioral: a stale decline actually
produces one retried charge, not zero and not two).

## AC-3

Golden flow: a partial refund applied against a stale-declined loan still
reconciles correctly, with the refund amount reflected before any retried
charge is issued.

## AC-4

Seven days post-deploy, the stale-decline rate for the affected loan
cohort is compared against the four-week pre-change baseline — the one AC
here that can only be evidenced after rollout.

## AC rationale

**Static (ac-1).** The retry path's obligation — every stale-decline
branch routes through the outbox, never a direct service call — is a
property of the code, not of any one run. A static check over the call
graph proves it for every future change, not just the one this story
shipped, which is the cheapest and most durable evidence available for a
structural claim like this one.

**Static and behavioral (ac-2).** The charge retry is both a shape claim
(the retry call is present and outbox-routed) and a behavior claim (a
stale decline actually triggers exactly one retried charge). Neither
alone closes the loop from adr/0002's design to a running system; static
evidence without behavioral evidence would only prove the wiring exists,
not that it fires.

**Behavioral (ac-3).** Partial refunds are a golden-flow scenario, not a
structural property of the code — the only credible evidence is running
the flow and observing the refund apply correctly against a stale
decline, which is why ac-3 is behavioral only.

**Runtime (ac-4).** A post-deploy decline-rate check is inherently a
production signal: no pre-deploy static or behavioral evidence can show
the retry logic actually reduces stale-decline noise in the real traffic
mix, so ac-4 is the one AC that can only be satisfied after rollout.
waiver/jira-loan-1482--ac-4 records that the runtime probe was not yet
built at freeze time; waiver/jira-loan-1482--ac-3 covered a shorter gap
while a partial-refunds test fixture was built for ac-3, and expired once
that evidence landed. ac-2 carries a direct attestation
(attestation/jira-loan-1482--ac-2) from the QA lead; an open conflict
(conflict/stale-decline-incident) is still being run down against a
production report that looked like it contradicted ac-2's behavior, and
conflict/false-alarm records an earlier, structurally similar report that
was investigated and dismissed.

## OQ-1

Partial refunds are explicitly out of scope for this story — the board
question above was contradicted for that reason — but the underlying
question is still open: should a partial refund's retry consume the same
outbox retry budget as a full stale-decline retry, or does it need its
own budget so a burst of partial refunds can't starve decline retries?
This overlaps adr/0003-retry-policy, which is itself still proposed and
under live debate; resolving oq-1 is blocked on that ADR landing, not on
anything specific to this story. Decision owner: platform-team, tracked
against adr/0003's own review rather than reopened piecemeal here.
