---
id: obligation/wall-strip-and-drawer-v2--ac-1--behavioral
kind: obligation
title: "The one-line case-file strip, edited in place"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `wall-strip-and-drawer › The one-line case-file strip, edited in place` in e2e/tests/91-wall-strip-drawer.spec.ts passes when CI job verify's producer runs that file alone. The test asserts the strip's problem and outcome lines, edits each in place with Enter and asserts one set-problem or set-outcome operation, cancels with Escape with nothing written, opens the full case file, and asserts the badges, flags, and disclosures as chips in the strip."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/91-wall-strip-drawer.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/91-wall-strip-drawer.spec.ts:wall-strip-and-drawer › The one-line case-file strip, edited in place" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/91-wall-strip-drawer.spec.ts:wall-strip-and-drawer › The one-line case-file strip, edited in place in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/wall-strip-and-drawer-v2" }
frozen: { at: 2026-09-30, commit: da729113549288efdee9961dcfa089c2d4ac199a }
---
# The one-line case-file strip, edited in place

CI job `verify` must record producer `playwright:e2e/tests/91-wall-strip-drawer.spec.ts:wall-strip-and-drawer › The
one-line case-file strip, edited in place` at the exact candidate commit. The test asserts the strip's problem and
outcome lines, edits each in place with Enter and asserts one set-problem or set-outcome operation, cancels with Escape
with nothing written, opens the full case file, and asserts the badges, flags, and disclosures as chips in the strip.
The test passes when its file runs alone (Playwright producer design §6; BL-98).
