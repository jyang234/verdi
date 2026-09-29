---
id: obligation/wall-canvas--ac-6--behavioral
kind: obligation
title: "The keyboard and the minimap reach every card"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/wall-canvas" }
frozen: { at: 2026-09-29, commit: abb2204688b89aaa71e1e0cb5e1e413c0c98c959 }
---
# The keyboard and the minimap reach every card

The behavioral evidence is the Playwright file `e2e/tests/90-wall-keyboard.spec.ts`. From the keyboard alone the test
moves the selection through every column with the arrows and asserts each card revealed, edits with Enter, closes the
innermost open thing with Escape, deletes a card and a thread through the confirmation, and asserts the trash and Delete
refuse a declared stub with a plain message; it drags the minimap's frame and asserts the viewport moves. No producer
grammar exists yet for a Playwright test (SI-228 parses go-test refs only), so this obligation is declared unresolved
design debt (SI-288) and names its evidence here.
