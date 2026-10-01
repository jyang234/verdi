---
id: obligation/new-story-dialog-v2--ac-2--behavioral
kind: obligation
title: "Criteria listed with their coverage, claimable"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `new-story-dialog › Criteria listed with their coverage, claimable` in e2e/tests/95-new-story-dialog.spec.ts passes when CI job verify's producer runs that file alone. On a feature with covered and uncovered criteria, the test asserts each criterion's coverage text equals the wall's chip text for it, and claims and unclaims a criterion."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/95-new-story-dialog.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/95-new-story-dialog.spec.ts:new-story-dialog › Criteria listed with their coverage, claimable" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/95-new-story-dialog.spec.ts:new-story-dialog › Criteria listed with their coverage, claimable in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/new-story-dialog-v2" }
frozen: { at: 2026-09-30, commit: ca6ecec16b315d74bdfac2a0f9f422cf141ebcf2 }
---
# Criteria listed with their coverage, claimable

CI job `verify` must record producer `playwright:e2e/tests/95-new-story-dialog.spec.ts:new-story-dialog › Criteria
listed with their coverage, claimable` at the exact candidate commit. On a feature with covered and uncovered criteria,
the test asserts each criterion's coverage text equals the wall's chip text for it, and claims and unclaims a criterion.
The test passes when its file runs alone (Playwright producer design §6; BL-98).
