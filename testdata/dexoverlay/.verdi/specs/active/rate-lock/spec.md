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
frozen: { at: 2026-07-11, commit: 93ddc5bbbb398cf747151e1c466afb83114398df }
---
# Rate lock (fixture, superseded feature)

**Feature-rung supersession fixture, v1** (spec/feature-supersession-state
dc-4). A feature predecessor whose `status` was flipped to `superseded` when
its successor `spec/rate-lock-v2` was accepted — the same-ritual accept flip
ac-1 performs (03 §rung 3). It exists so the SURFACES can be proven to render
that terminal state — the board's status badge and dex's `badge-superseded` —
at the FEATURE rung, where verdi's own corpus has no real superseded feature
(dc-4: honestly fixtured, never a flipped real feature).

Surface fixture only: the successor carries no rung-4 `supersession:` manifest
because nothing here is linted (testdata/dexoverlay is never walked by the
corpus-clean gate — see its README).

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
