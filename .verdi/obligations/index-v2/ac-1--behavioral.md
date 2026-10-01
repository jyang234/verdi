---
id: obligation/index-v2--ac-1--behavioral
kind: obligation
title: "Four columns, every spec once, counts and empty states"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `index › Four columns, every spec once, counts and empty states` in e2e/tests/96-index-pipeline.spec.ts passes when CI job verify's producer runs that file alone. Over a fixture store spanning every status and both zones plus design-branch drafts, the test asserts the four columns in order, each spec exactly once in its group, each column's count, and an empty state on a fixture where a column is empty."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/96-index-pipeline.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/96-index-pipeline.spec.ts:index › Four columns, every spec once, counts and empty states" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/96-index-pipeline.spec.ts:index › Four columns, every spec once, counts and empty states in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/index-v2" }
frozen: { at: 2026-09-30, commit: ca6ecec16b315d74bdfac2a0f9f422cf141ebcf2 }
---
# Four columns, every spec once, counts and empty states

CI job `verify` must record producer `playwright:e2e/tests/96-index-pipeline.spec.ts:index › Four columns, every spec
once, counts and empty states` at the exact candidate commit. Over a fixture store spanning every status and both zones
plus design-branch drafts, the test asserts the four columns in order, each spec exactly once in its group, each
column's count, and an empty state on a fixture where a column is empty. The test passes when its file runs alone
(Playwright producer design §6; BL-98).
