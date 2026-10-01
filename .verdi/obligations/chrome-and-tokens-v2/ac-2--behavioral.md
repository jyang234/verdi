---
id: obligation/chrome-and-tokens-v2--ac-2--behavioral
kind: obligation
title: "The bar keeps the displayed bytes and clean or dirty state, with the full posture one action away"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `chrome-and-tokens › The bar keeps the displayed bytes and clean or dirty state, with the full posture one action away` in e2e/tests/87-workbench-topbar.spec.ts passes when CI job verify's producer runs that file alone. On a spec wall and its Document page, over a clean and a dirty working tree, the test asserts the displayed-bytes text and the clean or dirty state in the bar at 1440 px and at 320 px; then, with JavaScript disabled, one activation of the posture text reveals checkout, branch, worktree head, accepted head, and ahead and behind, each carrying the same text and state test ids today's posture row carries, including a disclosed-unproven fact on a fixture whose default branch cannot be resolved."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/87-workbench-topbar.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/87-workbench-topbar.spec.ts:chrome-and-tokens › The bar keeps the displayed bytes and clean or dirty state, with the full posture one action away" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/87-workbench-topbar.spec.ts:chrome-and-tokens › The bar keeps the displayed bytes and clean or dirty state, with the full posture one action away in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/chrome-and-tokens-v2" }
frozen: { at: 2026-09-30, commit: ca6ecec16b315d74bdfac2a0f9f422cf141ebcf2 }
---
# The bar keeps the displayed bytes and clean or dirty state, with the full posture one action away

CI job `verify` must record producer `playwright:e2e/tests/87-workbench-topbar.spec.ts:chrome-and-tokens › The bar keeps
the displayed bytes and clean or dirty state, with the full posture one action away` at the exact candidate commit. On a
spec wall and its Document page, over a clean and a dirty working tree, the test asserts the displayed-bytes text and
the clean or dirty state in the bar at 1440 px and at 320 px; then, with JavaScript disabled, one activation of the
posture text reveals checkout, branch, worktree head, accepted head, and ahead and behind, each carrying the same text
and state test ids today's posture row carries, including a disclosed-unproven fact on a fixture whose default branch
cannot be resolved. The test passes when its file runs alone (Playwright producer design §6; BL-98).
