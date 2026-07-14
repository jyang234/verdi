---
id: obligation/evidence-slot--ac-3--behavioral
kind: obligation
title: "Playwright proves demand and holdings read as one row"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/evidence-slot" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# Playwright proves demand and holdings read as one row

The behavioral evidence must include a Playwright e2e (verdi/e2e/) over a
story AC declaring at least two kinds — one with an authored obligation
and no record, one with neither — asserting each kind renders exactly ONE
row containing both its obligation half (title or the disclosed "no
obligation") and its record-state half (the empty-slot chip), and that no
card element repeats a kind: the DOM must contain no second per-kind list.
