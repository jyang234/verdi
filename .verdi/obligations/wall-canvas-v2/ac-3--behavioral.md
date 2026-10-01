---
id: obligation/wall-canvas-v2--ac-3--behavioral
kind: obligation
title: "The toolbar offers exactly the legal actions for the selection and mode"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `wall-canvas › The toolbar offers exactly the legal actions for the selection and mode` in e2e/tests/89-wall-canvas.spec.ts passes when CI job verify's producer runs that file alone. For no selection, each card kind, a stub card, and each thread kind, in authoring mode, the test asserts the toolbar's exact action set, including no graduate, delete, or retype on a stub card; in review and read-only modes it asserts only the yarn key and the read actions."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/89-wall-canvas.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/89-wall-canvas.spec.ts:wall-canvas › The toolbar offers exactly the legal actions for the selection and mode" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/89-wall-canvas.spec.ts:wall-canvas › The toolbar offers exactly the legal actions for the selection and mode in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/wall-canvas-v2" }
frozen: { at: 2026-09-30, commit: ca6ecec16b315d74bdfac2a0f9f422cf141ebcf2 }
---
# The toolbar offers exactly the legal actions for the selection and mode

CI job `verify` must record producer `playwright:e2e/tests/89-wall-canvas.spec.ts:wall-canvas › The toolbar offers
exactly the legal actions for the selection and mode` at the exact candidate commit. For no selection, each card kind, a
stub card, and each thread kind, in authoring mode, the test asserts the toolbar's exact action set, including no
graduate, delete, or retype on a stub card; in review and read-only modes it asserts only the yarn key and the read
actions. The test passes when its file runs alone (Playwright producer design §6; BL-98).
