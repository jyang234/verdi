---
id: obligation/chrome-and-tokens-v2--ac-5--behavioral
kind: obligation
title: "The bar at 320 px and 200 % zoom, without JavaScript, by keyboard, within budget"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `chrome-and-tokens › The bar at 320 px and 200 % zoom, without JavaScript, by keyboard, within budget` in e2e/tests/87-workbench-topbar.spec.ts passes when CI job verify's producer runs that file alone. At 320 px and at 200 % zoom the test asserts no horizontal scroll and every bar control visible or reachable; with JavaScript disabled it asserts the bar in the initial response; it tabs through the bar asserting reading order; runs the existing axe check on each page; and asserts each page's transferred bytes stay within the Wave 6 budget the existing budget test uses."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/87-workbench-topbar.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/87-workbench-topbar.spec.ts:chrome-and-tokens › The bar at 320 px and 200 % zoom, without JavaScript, by keyboard, within budget" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/87-workbench-topbar.spec.ts:chrome-and-tokens › The bar at 320 px and 200 % zoom, without JavaScript, by keyboard, within budget in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/chrome-and-tokens-v2" }
frozen: { at: 2026-09-30, commit: ca6ecec16b315d74bdfac2a0f9f422cf141ebcf2 }
---
# The bar at 320 px and 200 % zoom, without JavaScript, by keyboard, within budget

CI job `verify` must record producer `playwright:e2e/tests/87-workbench-topbar.spec.ts:chrome-and-tokens › The bar at
320 px and 200 % zoom, without JavaScript, by keyboard, within budget` at the exact candidate commit. At 320 px and at
200 % zoom the test asserts no horizontal scroll and every bar control visible or reachable; with JavaScript disabled it
asserts the bar in the initial response; it tabs through the bar asserting reading order; runs the existing axe check on
each page; and asserts each page's transferred bytes stay within the Wave 6 budget the existing budget test uses. The
test passes when its file runs alone (Playwright producer design §6; BL-98).
