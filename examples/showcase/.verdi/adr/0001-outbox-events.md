---
id: adr/0001-outbox-events
kind: adr
title: "Synchronous dual-write for domain events"
status: superseded
owners: [platform-team]
decided: 2025-08-20
frozen: { at: 2025-08-20, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# Synchronous dual-write for domain events

## Context

loansvc's earliest downstream consumers — notification-svc (borrower
notices) and payments-gw (charge retries) — needed to react to a loan
state change the moment it happened. The team's first answer was the
obvious one: the request handler that wrote the state change also called
notification-svc or payments-gw inline, in the same request, before
returning to the caller.

## Decision

Every loansvc write with a downstream consequence for notification-svc or
payments-gw performs a synchronous call to that service in the same
request, before the write's own response is returned. There is no queue
and no intermediate record — the synchronous call itself is the delivery
mechanism, and its HTTP response is the only signal of success or
failure.

## Consequences

This held up under low volume and fell apart under real failure modes,
both structural: a downstream call that timed out left the primary write
committed with its consequence undelivered, discoverable only by an
operator noticing and replaying it by hand; a downstream call that
succeeded but whose response was lost (a connection reset after the
remote side had already committed) caused the caller's own retry to
re-send a request whose first attempt had, unknown to it, already landed
— delivering the consequence twice. Neither failure mode was rare enough
to ignore, and both trace to the same root cause: a write and its
consequence were never atomic, only adjacent in the same request.

Superseded by `adr/0002-outbox-events` on 2025-11-05, following the
2025-10 dual-write incident: a mid-request failover on notification-svc
left a batch of stale-decline notices in exactly the ambiguous state
this section describes, and the retrying caller's re-send duplicated
every one of them. The incident made the structural problem impossible
to keep treating as a tail risk.
