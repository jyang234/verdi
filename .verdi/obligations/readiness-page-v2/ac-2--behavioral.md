---
id: obligation/readiness-page-v2--ac-2--behavioral
kind: obligation
title: "Focus next ranks every concern, guidance first"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `readiness-page › Focus next ranks every concern, guidance first` in e2e/tests/93-readiness-page.spec.ts passes when CI job verify's producer runs that file alone. The test asserts the current step's concerns rank before the waiting ones with their step and now or later marks; each item's first line is its guidance sentence, with fact, timing, and blocking flag inside the technical disclosure; expanding the waiting concerns shows them inline; and the rendered concerns equal the readiness facts' complete set."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/93-readiness-page.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/93-readiness-page.spec.ts:readiness-page › Focus next ranks every concern, guidance first" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/93-readiness-page.spec.ts:readiness-page › Focus next ranks every concern, guidance first in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/readiness-page-v2" }
frozen: { at: 2026-09-30, commit: ca6ecec16b315d74bdfac2a0f9f422cf141ebcf2 }
---
# Focus next ranks every concern, guidance first

CI job `verify` must record producer `playwright:e2e/tests/93-readiness-page.spec.ts:readiness-page › Focus next ranks
every concern, guidance first` at the exact candidate commit. The test asserts the current step's concerns rank before
the waiting ones with their step and now or later marks; each item's first line is its guidance sentence, with fact,
timing, and blocking flag inside the technical disclosure; expanding the waiting concerns shows them inline; and the
rendered concerns equal the readiness facts' complete set. The test passes when its file runs alone (Playwright producer
design §6; BL-98).
