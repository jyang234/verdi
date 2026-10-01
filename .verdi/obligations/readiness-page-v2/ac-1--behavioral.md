---
id: obligation/readiness-page-v2--ac-1--behavioral
kind: obligation
title: "The readiness page in the design's layout"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `readiness-page › The readiness page in the design's layout` in e2e/tests/93-readiness-page.spec.ts passes when CI job verify's producer runs that file alone. Over fixture specs at each step, the test asserts the where-you-are header, the four-step stepper with each step's state and line, the ordering sentence, and the Focus next, Known problems in later steps, and Completed checks sections."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/93-readiness-page.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/93-readiness-page.spec.ts:readiness-page › The readiness page in the design's layout" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/93-readiness-page.spec.ts:readiness-page › The readiness page in the design's layout in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/readiness-page-v2" }
frozen: { at: 2026-09-30, commit: dbea2070d3c3ec5f5599f20d10dda4eedb218582 }
---
# The readiness page in the design's layout

CI job `verify` must record producer `playwright:e2e/tests/93-readiness-page.spec.ts:readiness-page › The readiness page
in the design's layout` at the exact candidate commit. Over fixture specs at each step, the test asserts the
where-you-are header, the four-step stepper with each step's state and line, the ordering sentence, and the Focus next,
Known problems in later steps, and Completed checks sections. The test passes when its file runs alone (Playwright
producer design §6; BL-98).
