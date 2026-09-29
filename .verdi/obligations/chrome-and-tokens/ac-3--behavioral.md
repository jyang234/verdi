---
id: obligation/chrome-and-tokens--ac-3--behavioral
kind: obligation
title: "Nothing rotated, stamps drawn as chips, handwriting only on a parked sticky"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/chrome-and-tokens" }
frozen: { at: 2026-09-29, commit: 749cb2ba49949997cf5efa7e563ff9afbfe2b931 }
---
# Nothing rotated, stamps drawn as chips, handwriting only on a parked sticky

The behavioral evidence is the Playwright file `e2e/tests/88-workbench-no-rotation.spec.ts`. On a wall carrying every
card kind and a parked story sticky in the stubs band, the test reads each element's computed `transform` and asserts no
rotation; asserts the status, class, and mode marks are chips; and asserts the hand font's computed `font-family`
appears only on the parked sticky. No producer grammar exists yet for a Playwright test (SI-228 parses go-test refs
only), so this obligation is declared unresolved design debt (SI-288) and names its evidence here.
