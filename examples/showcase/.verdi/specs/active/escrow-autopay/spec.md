---
id: spec/escrow-autopay
kind: spec
class: feature
title: "Escrow autopay enrollment"
status: accepted-pending-build
owners: [platform-team]
story: jira:LOAN-1519
problem: { text: "a borrower who wants their escrow payment collected automatically has to call servicing to set it up, and every mandate change after that is a manual back-office edit", anchor: "#problem" }
outcome: { text: "a borrower can enroll an escrow account in autopay, edit the mandate themselves, and trust that a failed scheduled charge is retried instead of silently dropped", anchor: "#outcome" }
impacts: [loansvc, notification-svc, payments-gw]
context:
  - adr/0002-outbox-events@78e3161594fb31fdad17f2ea8a96b52f33dbf0f3
declares:
  boundaries:
    - { from: loansvc, to: notification-svc, via: events }
    - { from: loansvc, to: payments-gw, via: events }
acceptance_criteria:
  - { id: ac-1, text: "an autopay mandate is created against a submitted application's escrow account, tied to the payment method already on file", evidence: [static, behavioral, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "a borrower who edits an existing autopay mandate sees the change reflected in their account before they leave the session", evidence: [behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "a scheduled autopay charge that fails retries according to the declared retry policy instead of silently dropping", evidence: [static, behavioral], anchor: "#ac-3" }
constraints:
  - { id: co-1, text: "must not touch the legacy schema", anchor: "#co-1" }
decisions:
  - { id: dc-1, text: "excuse this feature from ADR-0001's synchronous-write rule", anchor: "#dc-1",
      links: [ { type: exempts, ref: adr/0001-outbox-events, note: "outbox pattern already async by design" } ] }
  - { id: dc-2, text: "use the outbox pattern for mandate and retry events", anchor: "#dc-2" }
  - { id: dc-3, text: "excuse the legacy loan-import bridge job's PII fields from ADR-0004's redact-at-ingest rule", anchor: "#dc-3",
      links: [ { type: exempts, ref: adr/0004-pii-redaction-at-ingest, note: "legacy loan-import bridge job still stages raw SSN/DOB ahead of its own redaction rollout; product-lead sign-off 2026-03-12, remediation tracked against the importer's own backlog, targeted Q3 2026" } ] }
  - { id: dc-4, text: "excuse the escrow-account backfill reconciliation job from the same rule, for the same importer dependency", anchor: "#dc-4",
      links: [ { type: exempts, ref: adr/0004-pii-redaction-at-ingest, note: "reconciliation job reads the bridge job's staging table directly, so it inherits the same unredacted fields; same product-lead sign-off 2026-03-12, same Q3 2026 remediation timeline" } ] }
open_questions:
  - { id: oq-1, text: "should the mobile app use PUT or PATCH for the mandate update route?", anchor: "#oq-1" }
stubs:
  - { slug: autopay-mandate-api, acceptance_criteria: [ac-1, ac-2] }
  - { slug: autopay-retry-policy, acceptance_criteria: [ac-2, ac-3] }
frozen: { at: 2026-06-30, commit: 16219044c9d6d41de9a0de9464ed24d49283b40c }
---
# Escrow autopay enrollment

## Problem

A borrower who wants their escrow payment collected automatically today has
to call servicing, wait for a back-office agent to key the mandate in by
hand, and call again for every change afterward — a mandate amount update,
a payment-method swap, a pause. None of it is self-service, and a failed
scheduled charge (an expired card, an insufficient-funds day) currently
just fails; nothing retries it, so the borrower finds out only when the
next statement shows a missed payment.

## Outcome

A borrower can enroll an escrow account in autopay and edit the mandate
themselves, seeing the change reflected in their own account view before
they leave the session. A scheduled charge that fails is retried according
to a declared policy instead of silently dropping, the same outbox
discipline `spec/stale-decline` already established for this codebase's
other retry-shaped problem.

## Design notes

Autopay reuses the outbox pattern (adr/0002) for both halves of its
consequence surface: a mandate change publishes an event loansvc's
own view and notification-svc both consume, and a scheduled charge is
issued to payments-gw the same way a stale-decline retry is — never a
synchronous call made from inside the request that just accepted the
mandate edit. This is deliberate reuse, not coincidence: autopay is the
second feature (after stale-decline) to lean on adr/0002's guarantee that
a failure between "decide to act" and "the action lands" can never
duplicate or drop the action.

## Boundaries

- loansvc -> notification-svc, via events: a mandate created or edited
  notifies the borrower and, where the change is material (payment method,
  amount), support — the same event-carried decline id discipline
  `spec/stale-decline` uses to dedupe on redelivery applies here to the
  mandate id.
- loansvc -> payments-gw, via events: a scheduled charge, and any retry of
  a failed one, is issued through the outbox rather than a synchronous
  payments-gw call made at the moment the schedule fires — the same
  reasoning `spec/stale-decline`'s own payments-gw boundary documents: a
  synchronous retry issued inline would reintroduce the dual-write risk
  the outbox pattern exists to close.

## AC-1

An autopay mandate is created against a submitted application's escrow
account, tied to the payment method already on file — not a new
payment-method entry, since that is a separate, already-shipped flow this
feature does not re-implement. Outcome-level; the outcome floor is
satisfied by a bound outcome attestation
(`attestations/escrow-autopay/ac-1.md`) alongside static and behavioral
evidence once the mandate API stub is realized.

## AC-2

A borrower who edits an existing autopay mandate — the amount, the payment
method, a pause — sees the change reflected in their own account view
before they leave the session, the same optimistic-update contract the
mobile client already provides for a submitted-application edit
(`spec/borrower-update-mobile#ac-2`). Behavioral only: there is no
structural property to check statically here, only whether the reflected
state actually matches what was submitted.

## AC-3

A scheduled autopay charge that fails — an expired card, an
insufficient-funds day — retries according to the declared retry policy
(`autopay-retry-policy`) instead of silently dropping and waiting for the
borrower to notice on their next statement. Static: the retry path is
outbox-routed, not a direct payments-gw call; behavioral: a failed charge
actually produces a retry, not silence. `autopay-retry-policy` also plans
against ac-2: a retry's own success or exhaustion is itself a mandate-
adjacent state change the borrower's account view must reflect, the same
in-session guarantee ac-2 already promises for a manual mandate edit.

## CO-1

Must not touch the legacy schema. Historical escrow balances predating
the 2024 servicing-system migration still live in loansvc's legacy
schema; autopay's mandate creation reads that history through a
read-only bridge job rather than querying the legacy tables directly, so
a mandate can be backdated against a balance history this feature never
has to understand the shape of.

## DC-1

Excuse this feature from ADR-0001's synchronous-write rule: the outbox
pattern is already asynchronous by design, so the rule does not bind here.

## DC-2

Use the outbox pattern for mandate and retry events.

## DC-3

Excuse the legacy loan-import bridge job's PII fields from
`adr/0004-pii-redaction-at-ingest`'s redact-at-ingest rule. The bridge
job CO-1 references was written years before ADR-0004 existed and still
stages raw SSN and date-of-birth fields into its staging table ahead of
loansvc's own write path — it predates the rule it cannot yet meet.
product-lead signed off this exemption on 2026-03-12, the same day
ADR-0004 was accepted, on the condition that the bridge job's own
remediation lands within Q3 2026; the exemption is audited per
`adr/0004`'s exemption-count mechanism (`verdi audit`, counted against the
audit exemption threshold, default 3), not left as a standing carve-out.

## DC-4

Excuse the escrow-account backfill reconciliation job from the same
rule, for the same reason: it reads the bridge job's staging table
directly rather than loansvc's redacted write path, so it inherits
exactly the same unredacted fields DC-3 excuses. Same product-lead
sign-off date, same Q3 2026 remediation timeline — the two jobs share
one root cause and will clear together once the bridge job redacts at
its own point of write.

## OQ-1

Should the mobile app use PUT or PATCH for the mandate update route? The
same offline-staleness question `spec/borrower-update-mobile-spike`
already investigated for the submitted-application update route applies
identically to a mandate edit made from the mobile app — spiked by that
same spec's `resolves` edge (R4-I-16, 02 §Object model).
