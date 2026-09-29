---
id: obligation/index--ac-3--behavioral
kind: obligation
title: "Filters, and review status disclosed when the forge is unreachable"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/index" }
frozen: { at: 2026-09-29, commit: 3d75b4036991122dd1ee3372ba84b509202170de }
---
# Filters, and review status disclosed when the forge is unreachable

The behavioral evidence is the Playwright file `e2e/tests/96-index-pipeline.spec.ts`. The test applies each filter and
asserts the selected cards; with the harness forge unreachable it asserts the in-review chip and filter say review
status is unavailable, show no zero, and every card stays in its column. No producer grammar exists yet for a Playwright
test (SI-228 parses go-test refs only), so this obligation is declared unresolved design debt (SI-288) and names its
evidence here.
