---
id: obligation/index--ac-4--behavioral
kind: obligation
title: "The New story call to action"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/index" }
frozen: { at: 2026-09-29, commit: 3d75b4036991122dd1ee3372ba84b509202170de }
---
# The New story call to action

The behavioral evidence is the Playwright file `e2e/tests/96-index-pipeline.spec.ts`. On an accepted feature with an
uncovered criterion the test asserts the call to action names it and opens the New story dialog with it claimed; a
feature with none uncovered shows no call to action. No producer grammar exists yet for a Playwright test (SI-228 parses
go-test refs only), so this obligation is declared unresolved design debt (SI-288) and names its evidence here.
