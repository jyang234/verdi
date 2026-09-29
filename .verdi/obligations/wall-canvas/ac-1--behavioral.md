---
id: obligation/wall-canvas--ac-1--behavioral
kind: obligation
title: "Cards at the new footprint with their receipts, and yarn in two layers"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/wall-canvas" }
frozen: { at: 2026-09-29, commit: abb2204688b89aaa71e1e0cb5e1e413c0c98c959 }
---
# Cards at the new footprint with their receipts, and yarn in two layers

The behavioral evidence is the Playwright file `e2e/tests/89-wall-canvas.spec.ts`. Over a fixture wall carrying every
card kind and every receipt, the test asserts each card's footprint and no rotation; the obligation rows, evidence
slots, attestation chips, and coverage chips with their exact existing texts; a stub card's slug as its first line; the
readiness mark on a card named by a Focus next concern; and the base yarn layer under the cards with the selection's
threads drawn in the layer above them. No producer grammar exists yet for a Playwright test (SI-228 parses go-test refs
only), so this obligation is declared unresolved design debt (SI-288) and names its evidence here.
