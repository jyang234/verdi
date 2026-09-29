---
id: obligation/new-story-dialog--ac-3--behavioral
kind: obligation
title: "Opened from the index, the uncovered criterion starts claimed"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/new-story-dialog" }
frozen: { at: 2026-09-29, commit: 1cf137299f6ee2fb04f9fbe848b161468dd0fd31 }
---
# Opened from the index, the uncovered criterion starts claimed

The behavioral evidence is the Playwright file `e2e/tests/95-new-story-dialog.spec.ts`. The test follows an index card's
call to action and asserts the dialog opens with that criterion claimed. No producer grammar exists yet for a Playwright
test (SI-228 parses go-test refs only), so this obligation is declared unresolved design debt (SI-288) and names its
evidence here.
