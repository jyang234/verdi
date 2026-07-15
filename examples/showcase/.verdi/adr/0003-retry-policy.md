---
id: adr/0003-retry-policy
kind: adr
title: "Per-event-class retry budgets for outbox publishers"
status: proposed
owners: [platform-team]
links:
  - { type: depends-on, ref: adr/0002-outbox-events }
---
# Per-event-class retry budgets for outbox publishers

## Context

`adr/0002`'s outbox publisher retries every event class against the same
fixed budget — three attempts with exponential backoff, then
dead-letter. That budget was sized around payments-gw's charge-retry
traffic, the highest-consequence class, and has held up fine there. It
has not held up for notification-svc's decline-notice traffic:
`spec/stale-decline#ac-2`'s own retry path shares the same shared
budget, and a 2026-02 incident (`conflict/stale-decline-incident`, filed
against `adr/0002`) showed a burst of stale declines exhausting the
shared budget before every notice cleared — the retries eventually
succeeded, but late enough that a borrower who received one saw it well
after the decline itself, which read as wrong even though nothing was
actually lost. `spec/stale-decline#oq-1` raises the adjacent, still-open
question of whether a partial refund's retry should draw from that same
budget, and explicitly defers to this ADR rather than deciding it
locally.

## Decision (proposed — under live debate)

Two shapes are on the table. **Shape A** splits the retry budget per
event class — declines, refunds, and charges each get an independently
sized budget, tuned to that class's own volume and consequence profile —
at the cost of more configuration surface to keep tuned as new classes
appear. **Shape B** keeps one shared budget but makes it elastic (grows
under burst load, shrinks back once the burst clears), which needs no
per-class tuning but is harder to reason about under a genuinely
correlated failure — the kind that caused the 2026-02 incident in the
first place, where growing the shared budget would have helped the
declines but starved whatever else was competing for it at the same
moment. platform-team has not reached quorum between the two; the
tradeoff is real on both sides and the debate is live.

## Consequences (pending acceptance)

Whichever shape is chosen, accepting this ADR is what resolves
`spec/stale-decline#oq-1` and closes `conflict/stale-decline-incident` —
both currently point here rather than being decided piecemeal against
one story. Until then, `spec/stale-decline`'s retry path keeps its
current shared-budget behavior; no consumer is blocked on this ADR's
outcome, only the open conflict and the open question are.
