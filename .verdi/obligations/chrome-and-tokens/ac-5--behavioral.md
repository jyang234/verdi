---
id: obligation/chrome-and-tokens--ac-5--behavioral
kind: obligation
title: "The bar at 320 px and 200 % zoom, without JavaScript, by keyboard, within budget"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/chrome-and-tokens" }
frozen: { at: 2026-09-29, commit: b59480c53eea58221e12fb01bd58d6a128e4500d }
---
# The bar at 320 px and 200 % zoom, without JavaScript, by keyboard, within budget

The behavioral evidence is the Playwright file `e2e/tests/87-workbench-topbar.spec.ts`. At 320 px and at 200 % zoom the
test asserts no horizontal scroll and every bar control visible or reachable; with JavaScript disabled it asserts the
bar in the initial response; it tabs through the bar asserting reading order; runs the existing axe check on each page;
and asserts each page's transferred bytes stay within the Wave 6 budget the existing budget test uses. No producer
grammar exists yet for a Playwright test (SI-228 parses go-test refs only), so this obligation is declared unresolved
design debt (SI-288) and names its evidence here.
