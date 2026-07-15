---
id: spec/rate-lock
kind: spec
class: feature
title: "Rate lock (fixture, superseded feature)"
owners: [platform-team]
status: superseded
problem: { text: "borrowers lose a good quoted rate the moment they pause the application", anchor: "#problem" }
outcome: { text: "borrowers can lock a quoted rate for a fixed window and finish later", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a borrower can lock a quoted rate for 30 days", evidence: [static, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "a locked rate survives a session restart", evidence: [static], anchor: "#ac-2" }
constraints:
  - { id: co-1, text: "must not lock a rate the pricing service has already retired", anchor: "#co-1" }
frozen: { at: 2026-07-11, commit: 620ade86bbd810b440a0d995859745d4402d7be8 }
---
# Rate lock (fixture, superseded feature)

**Feature-rung supersession fixture, v1** (spec/feature-supersession-state
dc-4). A feature predecessor whose `status` was flipped to `superseded`
when its successor `spec/rate-lock-v2` was accepted — the same-ritual
accept flip ac-1 performs (03 §rung 3). This pair carries a real
`supersession:` block (below, on the successor) and VL-015-cleanly does
so via its own small, dedicated fixturegit history
(`internal/artifact/v2fixture_test.go`'s `goldenShaC`/`goldenShaD`,
chained after the loan-workflow pair's own — see that file's doc comment)
rather than `layers.txt`'s shared corpus history: VL-015 reads this
predecessor's content back through git history at the exact commit named
below, which only a dedicated draft-then-frozen sub-history can satisfy
for a file introduced fresh in a shared layer.

## Why superseded

The fixed 30-day window shipped fast but didn't survive contact with the
loan programs it served: a first-time-buyer program routinely runs
45-day underwriting cycles, and a 30-day lock expired mid-review on a
predictable share of those applications, forcing a borrower to requote
at whatever the market did in the meantime. `spec/rate-lock-v2` replaces
the fixed 30-day window with a window configurable per loan program,
carrying `co-1` unchanged (a locked rate still can never survive the
pricing service retiring it) and dropping `ac-2`'s session-restart
guarantee — durability across a restart turned out to matter far less
than the window length itself, and the successor's own persistence
layer makes the guarantee structurally true regardless, so restating it
as a feature-level AC no longer earned its place.

## Problem

Borrowers lose a good quoted rate the moment they pause the application.

## Outcome

Borrowers can lock a quoted rate for a fixed window and finish later.

## AC-1

A borrower can lock a quoted rate for 30 days.

## AC-2

A locked rate survives a session restart.

## CO-1

Must not lock a rate the pricing service has already retired.
