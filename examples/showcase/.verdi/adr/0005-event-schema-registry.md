---
id: adr/0005-event-schema-registry
kind: adr
title: "Central schema registry for outbox event payloads"
status: accepted
owners: [platform-team]
decided: 2026-04-02
links:
  - { type: depends-on, ref: adr/0002-outbox-events }
frozen: { at: 2026-04-02, commit: 38cc28c9f7bdf4098bccc724caddd0acdc2d17f6 }
---
# Central schema registry for outbox event payloads

## Context

By early 2026, six services published or consumed the outbox events
`adr/0002` introduced — loansvc, notification-svc, escrow-svc,
rate-engine, doc-vault, and borrower-portal — each with its own ad hoc
notion of what an event class's payload actually contained. A field
renamed on the publishing side silently broke any consumer that read it
positionally rather than by name; the break was discovered when
notification-svc started dropping decline notices missing a field it
could no longer find, not before. There was no single place to look up
an event class's current shape, or to prove a downstream consumer's
expectations still matched a publisher's current output before shipping
a change.

## Decision

Every outbox event class registers a versioned schema in a central,
append-only registry that loansvc's boundary-contract publishes for
other services to depend on. A publisher cannot ship an event whose
payload deviates from its class's registered schema; a consumer declares
which schema version it was built against, so a publisher-side schema
bump becomes a computable compatibility check rather than a silent
runtime surprise. `adr/0004`'s redacted-field set is itself part of the
registered schema — a schema change that would newly expose a
PII-tagged field fails the registry check closed, not open.

## Consequences

This is the backbone the loansvc topology diagram documents: seven
services (payments-gw joined the graph after the registry existed),
each edge annotated with the event class and schema version it depends
on. The cost is process, not code: a publisher can no longer change a
payload shape unilaterally — it bumps the registered schema and lets
dependents catch up on their own schedule — which is the point. The
six-way ad hoc coordination this ADR replaces was strictly worse once a
seventh service joined the graph than the process cost of registering a
schema bump up front.
