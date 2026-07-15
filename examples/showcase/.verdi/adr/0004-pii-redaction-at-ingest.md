---
id: adr/0004-pii-redaction-at-ingest
kind: adr
title: "Redact PII fields at outbox ingest"
status: accepted
owners: [platform-team]
decided: 2026-03-12
links:
  - { type: depends-on, ref: adr/0002-outbox-events }
frozen: { at: 2026-03-12, commit: 09ed3760a09cc1ec9b0c5ccf78cebc3b1ca93fa5 }
---
# Redact PII fields at outbox ingest

## Context

`adr/0002`'s outbox pattern commits an event's full payload to the
outbox table inside the same transaction as the write that produced it —
by design, so the write and its consequence are atomic. That payload
inherited whatever fields the originating row carried, unfiltered: for
loansvc's borrower-facing writes, that meant SSN and date-of-birth flowed
into the outbox table and from there to every downstream reader —
notification-svc's consumer, and any operator with outbox read access
for on-call debugging — whether or not that reader needed to see them.
`conflict/pii-outbox-leak`, filed 2026-02-18 against `adr/0002`, made
this concrete: a security review of the outbox table found both fields
readable in plain text by anyone holding the same read grant
notification-svc's own retry-replay tooling already required.

## Decision

Every outbox publisher redacts a fixed set of identified-PII fields
(SSN, date-of-birth, and any field a service's own schema tags
`pii: true`) from an event's payload BEFORE the row is written to the
outbox table — never after. Redaction happens inside the same
transaction as the write, so there is no window in which an unredacted
row is visible even transiently. A consumer that legitimately needs a
redacted field (a compliance export, a legal hold) requests it through a
separate, audited read path — never by reading the outbox table
directly.

## Consequences

This closes the exposure `conflict/pii-outbox-leak` raised without
touching `adr/0002`'s own transactional guarantee — the write and its
outbox row are still atomic, only the row's shape changes. The cost
falls on write paths that pre-date this rule and cannot redact at the
point of write without a schema change of their own. One such path is
active today: the legacy loan-import bridge job escrow-autopay's mandate
creation reads through (`spec/escrow-autopay#co-1`: "must not touch the
legacy schema" directly) still stages raw SSN and date-of-birth fields
ahead of its own Q3 remediation. `spec/escrow-autopay#dc-3` and `#dc-4`
record that exemption and its sign-off, audited per the exemption-count
mechanism (`verdi audit`, `verdi.yaml`'s
`audit.exempts_conflict_threshold`) rather than left to convention.
