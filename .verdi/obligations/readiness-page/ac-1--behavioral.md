---
id: obligation/readiness-page--ac-1--behavioral
kind: obligation
title: "The readiness page in the design's layout"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/readiness-page" }
frozen: { at: 2026-09-29, commit: 01c200e50eacb38505ba12850d187be361d1301a }
---
# The readiness page in the design's layout

The behavioral evidence is the Playwright file `e2e/tests/93-readiness-page.spec.ts`. Over fixture specs at each step,
the test asserts the where-you-are header, the four-step stepper with each step's state and line, the ordering sentence,
and the Focus next, Known problems in later steps, and Completed checks sections. No producer grammar exists yet for a
Playwright test (SI-228 parses go-test refs only), so this obligation is declared unresolved design debt (SI-288) and
names its evidence here.
