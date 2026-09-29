---
id: obligation/readiness-page--ac-3--behavioral
kind: obligation
title: "Human review labeled plainly, three-valued states, solo-author language"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/readiness-page" }
frozen: { at: 2026-09-29, commit: 01c200e50eacb38505ba12850d187be361d1301a }
---
# Human review labeled plainly, three-valued states, solo-author language

The behavioral evidence is the Playwright file `e2e/tests/93-readiness-page.spec.ts`. On a fixture with a human-review
concern, the test asserts the plain human-review label with the formal obligation as secondary text, a proven, a
violated-with-witness, and a disclosed-unproven item each labeled so, and the solo-author wording. No producer grammar
exists yet for a Playwright test (SI-228 parses go-test refs only), so this obligation is declared unresolved design
debt (SI-288) and names its evidence here.
