---
id: obligation/wall-canvas--ac-3--behavioral
kind: obligation
title: "The toolbar offers exactly the legal actions for the selection and mode"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/wall-canvas" }
frozen: { at: 2026-09-29, commit: abb2204688b89aaa71e1e0cb5e1e413c0c98c959 }
---
# The toolbar offers exactly the legal actions for the selection and mode

The behavioral evidence is the Playwright file `e2e/tests/89-wall-canvas.spec.ts`. For no selection, each card kind, a
stub card, and each thread kind, in authoring mode, the test asserts the toolbar's exact action set, including no
graduate, delete, or retype on a stub card; in review and read-only modes it asserts only the yarn key and the read
actions. No producer grammar exists yet for a Playwright test (SI-228 parses go-test refs only), so this obligation is
declared unresolved design debt (SI-288) and names its evidence here.
