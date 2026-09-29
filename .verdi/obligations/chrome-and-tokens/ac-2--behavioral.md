---
id: obligation/chrome-and-tokens--ac-2--behavioral
kind: obligation
title: "The bar keeps the displayed bytes and clean or dirty state, with the full posture one action away"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/chrome-and-tokens" }
frozen: { at: 2026-09-29, commit: 749cb2ba49949997cf5efa7e563ff9afbfe2b931 }
---
# The bar keeps the displayed bytes and clean or dirty state, with the full posture one action away

The behavioral evidence is the Playwright file `e2e/tests/87-workbench-topbar.spec.ts`. On a spec wall and its Document
page, over a clean and a dirty working tree, the test asserts the displayed-bytes text and the clean or dirty state in
the bar at 1440 px and at 320 px; then, with JavaScript disabled, one activation of the posture text reveals checkout,
branch, worktree head, accepted head, and ahead and behind, each carrying the same text and state test ids today's
posture row carries, including a disclosed-unproven fact on a fixture whose default branch cannot be resolved. No
producer grammar exists yet for a Playwright test (SI-228 parses go-test refs only), so this obligation is declared
unresolved design debt (SI-288) and names its evidence here.
