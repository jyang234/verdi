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
frozen: { at: 2026-07-12, commit: 7248a3f6d1322f7df24a65b774ac334fd01e4274 }
---
# Rate lock v2 (fixture, supersedes rate-lock)

**Feature-rung supersession fixture, v2** (spec/feature-supersession-state
dc-4). Supersedes `spec/rate-lock` via the whole-spec `supersedes` edge; its
acceptance is what flips the predecessor's `status` to `superseded` (ac-1). It
is the source of the predecessor's computed `superseded-by` backlink on dex.

A SURFACE fixture: it deliberately omits the rung-4 `supersession:` manifest
(VL-015's lint scope, out of this story's board/dex surface scope, and
testdata/dexoverlay is never linted) so it stays minimal.

## Problem

Borrowers lose a good quoted rate the moment they pause the application.

## Outcome

Borrowers can lock a quoted rate for a configurable window and finish later.

## AC-1

A borrower can lock a quoted rate for a configurable window.

## CO-1

Must not lock a rate the pricing service has already retired.
