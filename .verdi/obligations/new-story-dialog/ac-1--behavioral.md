---
id: obligation/new-story-dialog--ac-1--behavioral
kind: obligation
title: "The branch preview, inline grammar report, and gated Create"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/new-story-dialog" }
frozen: { at: 2026-09-29, commit: 1cf137299f6ee2fb04f9fbe848b161468dd0fd31 }
---
# The branch preview, inline grammar report, and gated Create

The behavioral evidence is the Playwright file `e2e/tests/95-new-story-dialog.spec.ts`. The test types a valid name and
asserts the design/<slug> preview; types a name that breaks the grammar and asserts the inline report; and asserts
Create disabled, with the matching status line, until the name is valid and a criterion is claimed. No producer grammar
exists yet for a Playwright test (SI-228 parses go-test refs only), so this obligation is declared unresolved design
debt (SI-288) and names its evidence here.
