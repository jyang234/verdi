---
id: conflict/stale-decline-incident
kind: conflict
title: "2026-02 retry-budget exhaustion contradicts ac-2 (fixture, open)"
status: open
owners: [platform-team]
links:
  - { type: challenges, ref: spec/stale-decline }
  - { type: challenges, ref: adr/0002-outbox-events }
  - { type: annotates, ref: adr/0003-retry-policy }
---
# Conflict: stale-decline retry-budget exhaustion

Filed 2026-02-14 against `spec/stale-decline#ac-2`'s retry behavior and,
by extension, `adr/0002-outbox-events`'s shared publisher retry budget:
a burst of stale declines exhausted the outbox publisher's fixed
three-attempt budget before every decline's retried charge cleared. The
underlying retries eventually succeeded — nothing was lost — but several
borrowers received their retry confirmation late enough that it read as
a contradiction of ac-2's "retried exactly once" guarantee rather than a
timing artifact of a shared, saturated budget.

Not yet resolved, so no `frozen` stamp (conflicts freeze only at
resolution). This is the live debate `adr/0003-retry-policy` (proposed,
still under quorum) exists to settle — see `#annotates` above and
`spec/stale-decline#oq-1`, which explicitly defers its own
partial-refund retry-budget question to the same ADR rather than
resolving it locally. Resolving `adr/0003` is what closes this record,
either by superseding `adr/0002`'s shared-budget behavior or by
dismissing this filing if the incident's cause turns out to be
unrelated to budget sizing.
