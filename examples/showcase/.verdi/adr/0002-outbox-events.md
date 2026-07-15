---
id: adr/0002-outbox-events
kind: adr
title: "Transactional outbox for domain events"
status: accepted
owners: [platform-team]
decided: 2025-11-05
links:
  - { type: supersedes, ref: adr/0001-outbox-events }
frozen: { at: 2025-11-05, commit: 89f9926e9739b97e23eb52efb16206d0ff10ff4f }
---
# Transactional outbox for domain events

## Context

`adr/0001`'s synchronous dual-write (loansvc calling notification-svc or
payments-gw inline, in the same request as the state-changing write)
produces exactly the failure mode a dual-write always produces: the
primary write and the downstream call are two separate operations, not
one atomic one, so a fault between them either drops the consequence or
delivers it twice. This stopped being theoretical on 2025-10-xx: a
mid-request failover on notification-svc left a batch of stale-decline
notices in an ambiguous delivered/not-delivered state, and the
request-level retry that followed re-sent every one of them — borrowers
received duplicate decline notices for the same underlying event, some
hours apart, because the synchronous call's own success or failure was
invisible to the caller that was retrying it.

## Decision

Every downstream consequence of a loansvc write is recorded as a row in
an outbox table, inside the SAME database transaction as the write
itself, so the write and its consequence are atomic by construction and
can never be partially applied. A separate publisher process reads
unpublished outbox rows and delivers them to notification-svc or
payments-gw, retrying delivery on failure without re-running the
original request. Each event carries a stable id so a redelivered event
can be deduplicated by the consumer rather than trusted to arrive
exactly once.

## Consequences

The dual-write hole `adr/0001` opened is closed: a crash or failover
between the write and its delivery can no longer leave the write
"happened" while its consequence silently vanishes or duplicates,
because the consequence is committed alongside the write rather than
sent independently of it. The tradeoff is latency: a consequence is now
only as fresh as the publisher's poll/flush interval, not delivered
synchronously with the request. That has been acceptable for every
consumer built against it so far — `spec/stale-decline`'s retry path and
`spec/escrow-autopay`'s mandate and retry events both read the outbox's
eventual delivery as fast enough.

The publisher's retry budget is fixed and shared across every event
class, and that is no longer settled: `conflict/stale-decline-incident`
and `adr/0003-retry-policy` both trace back to whether one shared budget
can keep serving every consumer or needs to vary by event class. A
second gap surfaced independently in 2026-02: the outbox row's payload
inherited whatever fields the originating write carried, unfiltered,
which is `adr/0004`'s subject, not this one.
