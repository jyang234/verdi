---
id: obligation/new-story-dialog-v2--ac-4--behavioral
kind: obligation
title: "Nothing written until Create"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `new-story-dialog › Nothing written until Create` in e2e/tests/95-new-story-dialog.spec.ts passes when CI job verify's producer runs that file alone. The test cancels, and separately escapes, a filled dialog and asserts no new branch, file, or ref; then creates a story and asserts the branch and the scaffold written through the existing creation path."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/95-new-story-dialog.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/95-new-story-dialog.spec.ts:new-story-dialog › Nothing written until Create" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/95-new-story-dialog.spec.ts:new-story-dialog › Nothing written until Create in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/new-story-dialog-v2" }
frozen: { at: 2026-09-30, commit: 68c057eb24281c3382baab695234da382f5984ec }
---
# Nothing written until Create

CI job `verify` must record producer `playwright:e2e/tests/95-new-story-dialog.spec.ts:new-story-dialog › Nothing
written until Create` at the exact candidate commit. The test cancels, and separately escapes, a filled dialog and
asserts no new branch, file, or ref; then creates a story and asserts the branch and the scaffold written through the
existing creation path. The test passes when its file runs alone (Playwright producer design §6; BL-98).
