---
id: obligation/wall-canvas-v2--ac-5--behavioral
kind: obligation
title: "Declaring in place and editing a card"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `wall-canvas › Declaring in place and editing a card` in e2e/tests/89-wall-canvas.spec.ts passes when CI job verify's producer runs that file alone. The test declares an object from each column's slot with Enter and asserts the server's next id and one typed operation; Escape cancels with nothing written; the add-object dialog still works from the keyboard; and Enter and double click edit a selected card, Enter applying and Escape cancelling."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/89-wall-canvas.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/89-wall-canvas.spec.ts:wall-canvas › Declaring in place and editing a card" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/89-wall-canvas.spec.ts:wall-canvas › Declaring in place and editing a card in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/wall-canvas-v2" }
frozen: { at: 2026-09-30, commit: ca6ecec16b315d74bdfac2a0f9f422cf141ebcf2 }
---
# Declaring in place and editing a card

CI job `verify` must record producer `playwright:e2e/tests/89-wall-canvas.spec.ts:wall-canvas › Declaring in place and
editing a card` at the exact candidate commit. The test declares an object from each column's slot with Enter and
asserts the server's next id and one typed operation; Escape cancels with nothing written; the add-object dialog still
works from the keyboard; and Enter and double click edit a selected card, Enter applying and Escape cancelling. The test
passes when its file runs alone (Playwright producer design §6; BL-98).
