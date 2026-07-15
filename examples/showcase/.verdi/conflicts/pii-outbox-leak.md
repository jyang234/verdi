---
id: conflict/pii-outbox-leak
kind: conflict
title: "Unredacted PII in the outbox event log (fixture, superseded)"
status: superseded
owners: [platform-team]
links:
  - { type: challenges, ref: adr/0002-outbox-events }
frozen: { at: 2026-03-12, commit: 89f9926e9739b97e23eb52efb16206d0ff10ff4f }
---
# Conflict: unredacted PII in the outbox event log

Filed 2026-02-18 against `adr/0002-outbox-events`: a security review of
loansvc's outbox table found borrower SSN and date-of-birth fields
readable in plain text by anyone holding the outbox's read grant —
including notification-svc's own retry-replay tooling, which was never
designed to handle raw PII. `adr/0002`'s transactional guarantee (the
write and its outbox row are atomic) was never in dispute; the payload's
unfiltered shape was.

Superseded 2026-03-12, the same day `adr/0004-pii-redaction-at-ingest`
was accepted: publishers now redact identified-PII fields before an
event ever reaches the outbox table, closing the exposure this conflict
raised without touching `adr/0002`'s own mechanics.
