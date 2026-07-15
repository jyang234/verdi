---
id: conflict/false-alarm
kind: conflict
title: "False alarm on stale-decline evidence (fixture, dismissed)"
status: dismissed
owners: [platform-team]
links:
  - { type: challenges, ref: spec/stale-decline }
  - { type: annotates, ref: spec/stale-decline }
frozen: { at: 2026-05-12, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# Conflict: false alarm

Filed 2026-05-09 against `spec/stale-decline#ac-2`: a support ticket
reported a borrower charged twice for the same stale-declined loan,
structurally identical to the failure mode `adr/0001`'s dual-write once
produced. Investigated and dismissed 2026-05-12 — the outbox log showed
exactly one retried charge; the second line item on the borrower's
statement was an unrelated scheduled payment that happened to post the
same day. **Dismissal reason:** the retry path performed correctly;
the report was a coincidental-timing misread, not a defect, and
`adr/0002`'s duplicate-delivery guarantee held. Filing stood anyway,
per `03 §Challenging closed decisions`'s "filing is mandatory even when
the resolution is obvious" — the record is the evidence that
yesterday's ac-2 behavior was contested and cleared, not silently
assumed correct.

Exercises the `annotates` link type alongside the mandatory `challenges`
link.
