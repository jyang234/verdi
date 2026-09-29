---
id: obligation/wall-canvas--ac-4--behavioral
kind: obligation
title: "Drag-to-thread offers only the legal edge types"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/wall-canvas" }
frozen: { at: 2026-09-29, commit: 2e4a150d987d6b0fab99a80d1bb83d6130518e44 }
---
# Drag-to-thread offers only the legal edge types

The behavioral evidence is the Playwright file `e2e/tests/89-wall-canvas.spec.ts`. For each source and target kind pair
on the fixture, the test drags a pin and asserts the picker lists exactly the legal types with their consequence labels,
a gate-bearing type asks for confirmation, and choosing writes the edge through the typed-edge path and selects it; a
drag from a stub pin opens no picker; and a sticky's attribution yarn and graduation drop still write what they write
today. No producer grammar exists yet for a Playwright test (SI-228 parses go-test refs only), so this obligation is
declared unresolved design debt (SI-288) and names its evidence here.
