---
id: obligation/new-story-dialog-v2--ac-3--behavioral
kind: obligation
title: "Opened from the index, the uncovered criterion starts claimed"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `new-story-dialog › Opened from the index, the uncovered criterion starts claimed` in e2e/tests/95-new-story-dialog.spec.ts passes when CI job verify's producer runs that file alone. The test follows an index card's call to action and asserts the dialog opens with that criterion claimed."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/95-new-story-dialog.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/95-new-story-dialog.spec.ts:new-story-dialog › Opened from the index, the uncovered criterion starts claimed" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/95-new-story-dialog.spec.ts:new-story-dialog › Opened from the index, the uncovered criterion starts claimed in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/new-story-dialog-v2" }
frozen: { at: 2026-09-30, commit: ca6ecec16b315d74bdfac2a0f9f422cf141ebcf2 }
---
# Opened from the index, the uncovered criterion starts claimed

CI job `verify` must record producer `playwright:e2e/tests/95-new-story-dialog.spec.ts:new-story-dialog › Opened from
the index, the uncovered criterion starts claimed` at the exact candidate commit. The test follows an index card's call
to action and asserts the dialog opens with that criterion claimed. The test passes when its file runs alone (Playwright
producer design §6; BL-98).
