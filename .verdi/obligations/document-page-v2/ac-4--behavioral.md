---
id: obligation/document-page-v2--ac-4--behavioral
kind: obligation
title: "The Document page at 320 px, 200 % zoom, and without JavaScript"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `document-page › The Document page at 320 px, 200 % zoom, and without JavaScript` in e2e/tests/94-document-page.spec.ts passes when CI job verify's producer runs that file alone. At 320 px and at 200 % zoom the test asserts no horizontal scroll; with JavaScript disabled it asserts the stamp's state and commit, the rail, the body, and no id chip elements."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/94-document-page.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/94-document-page.spec.ts:document-page › The Document page at 320 px, 200 % zoom, and without JavaScript" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/94-document-page.spec.ts:document-page › The Document page at 320 px, 200 % zoom, and without JavaScript in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/document-page-v2" }
frozen: { at: 2026-09-30, commit: b7d2df0dfe0eaaffedac86d866fac238f7e3c5f1 }
---
# The Document page at 320 px, 200 % zoom, and without JavaScript

CI job `verify` must record producer `playwright:e2e/tests/94-document-page.spec.ts:document-page › The Document page at
320 px, 200 % zoom, and without JavaScript` at the exact candidate commit. At 320 px and at 200 % zoom the test asserts
no horizontal scroll; with JavaScript disabled it asserts the stamp's state and commit, the rail, the body, and no id
chip elements. The test passes when its file runs alone (Playwright producer design §6; BL-98).
