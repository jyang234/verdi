---
id: spec/rate-lock-v2
kind: spec
class: feature
title: "Rate lock v2 (fixture, supersedes rate-lock)"
owners: [platform-team]
status: accepted-pending-build
problem: { text: "borrowers lose a good quoted rate the moment they pause the application", anchor: "#problem" }
outcome: { text: "borrowers can lock a quoted rate for a configurable window and finish later", anchor: "#outcome" }
links:
  - { type: supersedes, ref: "spec/rate-lock" }
acceptance_criteria:
  - { id: ac-1, text: "a borrower can lock a quoted rate for a configurable window", evidence: [static, attestation], anchor: "#ac-1" }
constraints:
  - { id: co-1, text: "must not lock a rate the pricing service has already retired", anchor: "#co-1" }
supersession:
  carried: [co-1]
  amended: [ { id: ac-1, note: "the fixed 30-day window becomes a window configurable per loan program, after a first-time-buyer program's 45-day underwriting cycle routinely outran the fixed lock" } ]
  amended_advisory: []
  removed: [ { id: ac-2, note: "the session-restart guarantee is subsumed by v2's own persistence layer and no longer earns its place as a feature-level AC" } ]
  added: []
frozen: { at: 2026-07-12, commit: 87c65ef5e70024c112b12e275d550f1ca8584df3 }
---
# Rate lock v2 (fixture, supersedes rate-lock)

**Feature-rung supersession fixture, v2** (spec/feature-supersession-state
dc-4). Supersedes `spec/rate-lock` via the whole-spec `supersedes` edge;
its acceptance is what flips the predecessor's `status` to `superseded`
(ac-1). It is the source of the predecessor's computed `superseded-by`
backlink on dex. The `supersession:` block above classifies every one of
v1's three objects exactly once — `co-1` carried byte-identical (VL-015),
`ac-1` amended (the window widens from a fixed 30 days to a
per-program-configurable length), `ac-2` removed (see v1's own "Why
superseded" section for the reasoning behind both changes). Like
loan-workflow/loan-workflow-v2, this pair lives in its own small,
dedicated fixturegit history rather than `layers.txt`'s shared corpus one
(`internal/artifact/v2fixture_test.go`) — see `spec/rate-lock`'s own note
for why.

## Problem

Borrowers lose a good quoted rate the moment they pause the application.

## Outcome

Borrowers can lock a quoted rate for a configurable window and finish
later.

## AC-1

A borrower can lock a quoted rate for a configurable window: the length
is set per loan program rather than fixed at 30 days, closing the gap a
45-day underwriting cycle exposed in the predecessor.

## CO-1

Must not lock a rate the pricing service has already retired.
