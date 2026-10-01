---
id: obligation/index-v2--ac-3--behavioral
kind: obligation
title: "Filters, and review status disclosed when the forge is unreachable"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `index › Filters, and review status disclosed when the forge is unreachable` in e2e/tests/96-index-pipeline.spec.ts passes when CI job verify's producer runs that file alone. The test applies each filter and asserts the selected cards; with the harness forge unreachable it asserts the in-review chip and filter say review status is unavailable, show no zero, and every card stays in its column."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/96-index-pipeline.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/96-index-pipeline.spec.ts:index › Filters, and review status disclosed when the forge is unreachable" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/96-index-pipeline.spec.ts:index › Filters, and review status disclosed when the forge is unreachable in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/index-v2" }
frozen: { at: 2026-09-30, commit: 790c6f3f8e53f5947dad36c24fb57a9591e219d6 }
---
# Filters, and review status disclosed when the forge is unreachable

CI job `verify` must record producer `playwright:e2e/tests/96-index-pipeline.spec.ts:index › Filters, and review status
disclosed when the forge is unreachable` at the exact candidate commit. The test applies each filter and asserts the
selected cards; with the harness forge unreachable it asserts the in-review chip and filter say review status is
unavailable, show no zero, and every card stays in its column. The test passes when its file runs alone (Playwright
producer design §6; BL-98).
