---
id: obligation/wall-strip-and-drawer-v2--ac-6--behavioral
kind: obligation
title: "The rail is gone and every item has a home"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `wall-strip-and-drawer › The rail is gone and every item has a home` in e2e/tests/91-wall-strip-drawer.spec.ts passes when CI job verify's producer runs that file alone. The test asserts no rail element on any wall; the inbox tray docked and visible in review mode; Revise and New story as the primary action on a sealed wall; Instantiate on a stub card's toolbar; the policy guide in the Readiness tab; and every readiness target resolving to an element that exists."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/91-wall-strip-drawer.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/91-wall-strip-drawer.spec.ts:wall-strip-and-drawer › The rail is gone and every item has a home" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/91-wall-strip-drawer.spec.ts:wall-strip-and-drawer › The rail is gone and every item has a home in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/wall-strip-and-drawer-v2" }
frozen: { at: 2026-09-30, commit: da729113549288efdee9961dcfa089c2d4ac199a }
---
# The rail is gone and every item has a home

CI job `verify` must record producer `playwright:e2e/tests/91-wall-strip-drawer.spec.ts:wall-strip-and-drawer › The rail
is gone and every item has a home` at the exact candidate commit. The test asserts no rail element on any wall; the
inbox tray docked and visible in review mode; Revise and New story as the primary action on a sealed wall; Instantiate
on a stub card's toolbar; the policy guide in the Readiness tab; and every readiness target resolving to an element that
exists. The test passes when its file runs alone (Playwright producer design §6; BL-98).
