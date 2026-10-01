---
id: obligation/wall-canvas-v2--ac-6--behavioral
kind: obligation
title: "The keyboard and the minimap reach every card"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `wall-canvas › The keyboard and the minimap reach every card` in e2e/tests/90-wall-keyboard.spec.ts passes when CI job verify's producer runs that file alone. From the keyboard alone the test moves the selection through every column with the arrows and asserts each card revealed, edits with Enter, closes the innermost open thing with Escape, deletes a card and a thread through the confirmation, and asserts the trash and Delete refuse a declared stub with a plain message; it drags the minimap's frame and asserts the viewport moves."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/90-wall-keyboard.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/90-wall-keyboard.spec.ts:wall-canvas › The keyboard and the minimap reach every card" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/90-wall-keyboard.spec.ts:wall-canvas › The keyboard and the minimap reach every card in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/wall-canvas-v2" }
frozen: { at: 2026-09-30, commit: ca6ecec16b315d74bdfac2a0f9f422cf141ebcf2 }
---
# The keyboard and the minimap reach every card

CI job `verify` must record producer `playwright:e2e/tests/90-wall-keyboard.spec.ts:wall-canvas › The keyboard and the
minimap reach every card` at the exact candidate commit. From the keyboard alone the test moves the selection through
every column with the arrows and asserts each card revealed, edits with Enter, closes the innermost open thing with
Escape, deletes a card and a thread through the confirmation, and asserts the trash and Delete refuse a declared stub
with a plain message; it drags the minimap's frame and asserts the viewport moves. The test passes when its file runs
alone (Playwright producer design §6; BL-98).
