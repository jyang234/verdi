---
id: obligation/chrome-and-tokens-v2--ac-3--behavioral
kind: obligation
title: "Nothing rotated, stamps drawn as chips, handwriting only on a parked sticky"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `chrome-and-tokens › Nothing rotated, stamps drawn as chips, handwriting only on a parked sticky` in e2e/tests/88-workbench-no-rotation.spec.ts passes when CI job verify's producer runs that file alone. On a wall carrying every card kind and a parked story sticky in the stubs band, the test reads each element's computed `transform` and asserts no rotation; asserts the status, class, and mode marks are chips; and asserts the hand font's computed `font-family` appears only on the parked sticky."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/88-workbench-no-rotation.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/88-workbench-no-rotation.spec.ts:chrome-and-tokens › Nothing rotated, stamps drawn as chips, handwriting only on a parked sticky" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/88-workbench-no-rotation.spec.ts:chrome-and-tokens › Nothing rotated, stamps drawn as chips, handwriting only on a parked sticky in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/chrome-and-tokens-v2" }
frozen: { at: 2026-09-30, commit: 05bdbebd01ba11c2348fb4c4e5ec8d2195274131 }
---
# Nothing rotated, stamps drawn as chips, handwriting only on a parked sticky

CI job `verify` must record producer `playwright:e2e/tests/88-workbench-no-rotation.spec.ts:chrome-and-tokens › Nothing
rotated, stamps drawn as chips, handwriting only on a parked sticky` at the exact candidate commit. On a wall carrying
every card kind and a parked story sticky in the stubs band, the test reads each element's computed `transform` and
asserts no rotation; asserts the status, class, and mode marks are chips; and asserts the hand font's computed
`font-family` appears only on the parked sticky. The test passes when its file runs alone (Playwright producer design
§6; BL-98).
