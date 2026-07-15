---
id: waiver/jira-loan-1482--ac-3
kind: waiver
title: "ac-3 golden-flow evidence gap (expired)"
status: expired
owners: [platform-team]
reason: "golden flow pending test-data fixture"
expiry: 2026-06-01
frozen: { at: 2026-05-01, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# Waiver: ac-3 golden-flow evidence gap (expired)

`spec/stale-decline#ac-3` (a partial refund against a stale-declined
loan reconciles correctly) is behavioral-only — its only credible
evidence is running the golden flow and observing the refund apply
correctly. At freeze time (2026-05-01) the partial-refunds test-data
fixture that flow depends on did not exist yet, so ac-3 waived its
evidence requirement for a bounded window rather than block the story's
freeze on unrelated fixture work.

Expired 2026-06-01, matching the date the fixture landed and ac-3's
behavioral evidence started flowing for real (`spec/stale-decline`'s own
AC-rationale section names this waiver by id). product-lead signed off
the waiver at grant time; no extension was requested or needed once the
fixture shipped on schedule.
