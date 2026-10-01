---
id: obligation/index-v2--ac-2--behavioral
kind: obligation
title: "Each card's facts, links, and test ids"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `index › Each card's facts, links, and test ids` in e2e/tests/96-index-pipeline.spec.ts passes when CI job verify's producer runs that file alone. For a default-branch feature with stories, a component, an archived spec, and local and remote drafts, the test asserts each card's title, ref, badge, links (board only where servable; matrix and verdict only on the feature with stories), age, source, in-review chip on the draft with an open pull request, next move, disclosure, and dir-group and dir-entry test ids."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/96-index-pipeline.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/96-index-pipeline.spec.ts:index › Each card's facts, links, and test ids" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/96-index-pipeline.spec.ts:index › Each card's facts, links, and test ids in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/index-v2" }
frozen: { at: 2026-09-30, commit: ca6ecec16b315d74bdfac2a0f9f422cf141ebcf2 }
---
# Each card's facts, links, and test ids

CI job `verify` must record producer `playwright:e2e/tests/96-index-pipeline.spec.ts:index › Each card's facts, links,
and test ids` at the exact candidate commit. For a default-branch feature with stories, a component, an archived spec,
and local and remote drafts, the test asserts each card's title, ref, badge, links (board only where servable; matrix
and verdict only on the feature with stories), age, source, in-review chip on the draft with an open pull request, next
move, disclosure, and dir-group and dir-entry test ids. The test passes when its file runs alone (Playwright producer
design §6; BL-98).
