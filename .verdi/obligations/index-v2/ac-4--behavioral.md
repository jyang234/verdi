---
id: obligation/index-v2--ac-4--behavioral
kind: obligation
title: "The New story call to action"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `index › The New story call to action` in e2e/tests/96-index-pipeline.spec.ts passes when CI job verify's producer runs that file alone. On an accepted feature with an uncovered criterion the test asserts the call to action names it and opens the New story dialog with it claimed; a feature with none uncovered shows no call to action."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/96-index-pipeline.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/96-index-pipeline.spec.ts:index › The New story call to action" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/96-index-pipeline.spec.ts:index › The New story call to action in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/index-v2" }
frozen: { at: 2026-09-30, commit: ca6ecec16b315d74bdfac2a0f9f422cf141ebcf2 }
---
# The New story call to action

CI job `verify` must record producer `playwright:e2e/tests/96-index-pipeline.spec.ts:index › The New story call to
action` at the exact candidate commit. On an accepted feature with an uncovered criterion the test asserts the call to
action names it and opens the New story dialog with it claimed; a feature with none uncovered shows no call to action.
The test passes when its file runs alone (Playwright producer design §6; BL-98).
