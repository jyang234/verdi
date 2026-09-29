---
id: obligation/new-story-dialog--ac-2--behavioral
kind: obligation
title: "Criteria listed with their coverage, claimable"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/new-story-dialog" }
frozen: { at: 2026-09-29, commit: cf4b5ff48ee57337b78232d12be55e59e6c9dcdf }
---
# Criteria listed with their coverage, claimable

The behavioral evidence is the Playwright file `e2e/tests/95-new-story-dialog.spec.ts`. On a feature with covered and
uncovered criteria, the test asserts each criterion's coverage text equals the wall's chip text for it, and claims and
unclaims a criterion. No producer grammar exists yet for a Playwright test (SI-228 parses go-test refs only), so this
obligation is declared unresolved design debt (SI-288) and names its evidence here.
