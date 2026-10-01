---
id: obligation/wall-strip-and-drawer-v2--ac-5--behavioral
kind: obligation
title: "Every drawer tab renders prose and tables, never JSON"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `wall-strip-and-drawer › Every drawer tab renders prose and tables, never JSON` in e2e/tests/91-wall-strip-drawer.spec.ts passes when CI job verify's producer runs that file alone. For each tab the test asserts the rendered sections and no raw JSON text, an empty state on a fixture without that projection, and the posture reason when it is unavailable; each Readiness item's first line is its guidance sentence, with fact, timing, and blocking flag inside the technical disclosure; clicking a Readiness item selects its card; and the Review tab names the branch and command and has no control that opens a pull request."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/91-wall-strip-drawer.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/91-wall-strip-drawer.spec.ts:wall-strip-and-drawer › Every drawer tab renders prose and tables, never JSON" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/91-wall-strip-drawer.spec.ts:wall-strip-and-drawer › Every drawer tab renders prose and tables, never JSON in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/wall-strip-and-drawer-v2" }
frozen: { at: 2026-09-30, commit: da729113549288efdee9961dcfa089c2d4ac199a }
---
# Every drawer tab renders prose and tables, never JSON

CI job `verify` must record producer `playwright:e2e/tests/91-wall-strip-drawer.spec.ts:wall-strip-and-drawer › Every
drawer tab renders prose and tables, never JSON` at the exact candidate commit. For each tab the test asserts the
rendered sections and no raw JSON text, an empty state on a fixture without that projection, and the posture reason when
it is unavailable; each Readiness item's first line is its guidance sentence, with fact, timing, and blocking flag
inside the technical disclosure; clicking a Readiness item selects its card; and the Review tab names the branch and
command and has no control that opens a pull request. The test passes when its file runs alone (Playwright producer
design §6; BL-98).
