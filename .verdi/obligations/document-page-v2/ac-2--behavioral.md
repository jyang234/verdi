---
id: obligation/document-page-v2--ac-2--behavioral
kind: obligation
title: "Id chips open the wall with the card selected"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `document-page › Id chips open the wall with the card selected` in e2e/tests/94-document-page.spec.ts passes when CI job verify's producer runs that file alone. For an acceptance criterion, a constraint, a decision, and an open question, the test clicks its id chip and asserts the wall opens with that card selected and in view."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/94-document-page.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/94-document-page.spec.ts:document-page › Id chips open the wall with the card selected" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/94-document-page.spec.ts:document-page › Id chips open the wall with the card selected in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/document-page-v2" }
frozen: { at: 2026-09-30, commit: ca6ecec16b315d74bdfac2a0f9f422cf141ebcf2 }
---
# Id chips open the wall with the card selected

CI job `verify` must record producer `playwright:e2e/tests/94-document-page.spec.ts:document-page › Id chips open the
wall with the card selected` at the exact candidate commit. For an acceptance criterion, a constraint, a decision, and
an open question, the test clicks its id chip and asserts the wall opens with that card selected and in view. The test
passes when its file runs alone (Playwright producer design §6; BL-98).
