---
id: obligation/wall-strip-and-drawer--ac-6--behavioral
kind: obligation
title: "The rail is gone and every item has a home"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/wall-strip-and-drawer" }
frozen: { at: 2026-09-29, commit: d64d2d637fda0b28635c0760ac23a8d7093638fb }
---
# The rail is gone and every item has a home

The behavioral evidence is the Playwright file `e2e/tests/91-wall-strip-drawer.spec.ts`. The test asserts no rail
element on any wall; the inbox tray docked and visible in review mode; Revise and New story as the primary action on a
sealed wall; Instantiate on a stub card's toolbar; the policy guide in the Readiness tab; and every readiness target
resolving to an element that exists. No producer grammar exists yet for a Playwright test (SI-228 parses go-test refs
only), so this obligation is declared unresolved design debt (SI-288) and names its evidence here.
