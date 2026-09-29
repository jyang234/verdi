---
id: obligation/wall-canvas--ac-5--behavioral
kind: obligation
title: "Declaring in place and editing a card"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/wall-canvas" }
frozen: { at: 2026-09-29, commit: abb2204688b89aaa71e1e0cb5e1e413c0c98c959 }
---
# Declaring in place and editing a card

The behavioral evidence is the Playwright file `e2e/tests/89-wall-canvas.spec.ts`. The test declares an object from each
column's slot with Enter and asserts the server's next id and one typed operation; Escape cancels with nothing written;
the add-object dialog still works from the keyboard; and Enter and double click edit a selected card, Enter applying and
Escape cancelling. No producer grammar exists yet for a Playwright test (SI-228 parses go-test refs only), so this
obligation is declared unresolved design debt (SI-288) and names its evidence here.
