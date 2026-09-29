---
id: obligation/wall-canvas--ac-2--behavioral
kind: obligation
title: "Selecting a card emphasizes its threads and names them"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/wall-canvas" }
frozen: { at: 2026-09-29, commit: 2e4a150d987d6b0fab99a80d1bb83d6130518e44 }
---
# Selecting a card emphasizes its threads and names them

The behavioral evidence is the Playwright file `e2e/tests/89-wall-canvas.spec.ts`. The test clicks a card and asserts
the card and its threaded cards emphasized, the rest receded, and the status pill naming the card and its threads; a
card with no threads says so; clicking the wall and the same card clears the selection; and clicking a thread's chip
selects the thread. No producer grammar exists yet for a Playwright test (SI-228 parses go-test refs only), so this
obligation is declared unresolved design debt (SI-288) and names its evidence here.
