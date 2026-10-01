---
id: obligation/index-v2--ac-5--behavioral
kind: obligation
title: "On the shelf, the other-records strip, and the kept directory notices"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `index › On the shelf, the other-records strip, and the kept directory notices` in e2e/tests/96-index-pipeline.spec.ts passes when CI job verify's producer runs that file alone. The test asserts terminal active-zone specs in On the shelf with archived specs in the collapsed list at its foot; each other-records section collapsed with its count and expandable without JavaScript to its full listing; the Disclosures toggle; the disclosed entry for a branch with no draft; and the deleted-branch notice page."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/96-index-pipeline.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/96-index-pipeline.spec.ts:index › On the shelf, the other-records strip, and the kept directory notices" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/96-index-pipeline.spec.ts:index › On the shelf, the other-records strip, and the kept directory notices in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/index-v2" }
frozen: { at: 2026-09-30, commit: ca6ecec16b315d74bdfac2a0f9f422cf141ebcf2 }
---
# On the shelf, the other-records strip, and the kept directory notices

CI job `verify` must record producer `playwright:e2e/tests/96-index-pipeline.spec.ts:index › On the shelf, the
other-records strip, and the kept directory notices` at the exact candidate commit. The test asserts terminal
active-zone specs in On the shelf with archived specs in the collapsed list at its foot; each other-records section
collapsed with its count and expandable without JavaScript to its full listing; the Disclosures toggle; the disclosed
entry for a branch with no draft; and the deleted-branch notice page. The test passes when its file runs alone
(Playwright producer design §6; BL-98).
