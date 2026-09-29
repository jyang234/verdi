---
id: obligation/wall-strip-and-drawer--ac-4--behavioral
kind: obligation
title: "The branch menu, the readiness pill, and the menu's on-demand counts"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/wall-strip-and-drawer" }
frozen: { at: 2026-09-29, commit: 2e3c21487ee26132146d2aebeec5133fa943cde9 }
---
# The branch menu, the readiness pill, and the menu's on-demand counts

The behavioral evidence is the Playwright file `e2e/tests/91-wall-strip-drawer.spec.ts`. The test opens the branch menu
from the branch text and asserts the guard still refuses a switch over uncommitted changes; the readiness pill opens the
Readiness tab; and no count request is made until the menu opens, after which each item shows its count. No producer
grammar exists yet for a Playwright test (SI-228 parses go-test refs only), so this obligation is declared unresolved
design debt (SI-288) and names its evidence here.
