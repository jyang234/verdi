---
id: obligation/readiness-page-v2--ac-3--behavioral
kind: obligation
title: "Human review labeled plainly, three-valued states, solo-author language"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `readiness-page › Human review labeled plainly, three-valued states, solo-author language` in e2e/tests/93-readiness-page.spec.ts passes when CI job verify's producer runs that file alone. On a fixture with a human-review concern, the test asserts the plain human-review label with the formal obligation as secondary text, a proven, a violated-with-witness, and a disclosed-unproven item each labeled so, and the solo-author wording."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/93-readiness-page.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/93-readiness-page.spec.ts:readiness-page › Human review labeled plainly, three-valued states, solo-author language" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/93-readiness-page.spec.ts:readiness-page › Human review labeled plainly, three-valued states, solo-author language in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/readiness-page-v2" }
frozen: { at: 2026-09-30, commit: dbea2070d3c3ec5f5599f20d10dda4eedb218582 }
---
# Human review labeled plainly, three-valued states, solo-author language

CI job `verify` must record producer `playwright:e2e/tests/93-readiness-page.spec.ts:readiness-page › Human review
labeled plainly, three-valued states, solo-author language` at the exact candidate commit. On a fixture with a
human-review concern, the test asserts the plain human-review label with the formal obligation as secondary text, a
proven, a violated-with-witness, and a disclosed-unproven item each labeled so, and the solo-author wording. The test
passes when its file runs alone (Playwright producer design §6; BL-98).
