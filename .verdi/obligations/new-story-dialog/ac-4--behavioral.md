---
id: obligation/new-story-dialog--ac-4--behavioral
kind: obligation
title: "Nothing written until Create"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/new-story-dialog" }
frozen: { at: 2026-09-29, commit: 1cf137299f6ee2fb04f9fbe848b161468dd0fd31 }
---
# Nothing written until Create

The behavioral evidence is the Playwright file `e2e/tests/95-new-story-dialog.spec.ts`. The test cancels, and separately
escapes, a filled dialog and asserts no new branch, file, or ref; then creates a story and asserts the branch and the
scaffold written through the existing creation path. No producer grammar exists yet for a Playwright test (SI-228 parses
go-test refs only), so this obligation is declared unresolved design debt (SI-288) and names its evidence here.
