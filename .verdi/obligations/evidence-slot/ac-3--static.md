---
id: obligation/evidence-slot--ac-3--static
kind: obligation
title: "One per-kind row: the obligation column extended, not duplicated"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/evidence-slot" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# One per-kind row: the obligation column extended, not duplicated

The static evidence must show the record-state chip emitted inside the
card's existing per-kind obligation row renderer (the writeObligations
path in internal/workbench/boardspecrender.go) — one renderer producing
one row per declared kind carrying both obligation content and record
state — with no second per-kind list renderer added to the card, and the
page and fragment sharing that single code path.
