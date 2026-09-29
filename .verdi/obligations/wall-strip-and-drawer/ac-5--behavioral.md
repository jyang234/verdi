---
id: obligation/wall-strip-and-drawer--ac-5--behavioral
kind: obligation
title: "Every drawer tab renders prose and tables, never JSON"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/wall-strip-and-drawer" }
frozen: { at: 2026-09-29, commit: 2e3c21487ee26132146d2aebeec5133fa943cde9 }
---
# Every drawer tab renders prose and tables, never JSON

The behavioral evidence is the Playwright file `e2e/tests/91-wall-strip-drawer.spec.ts`. For each tab the test asserts
the rendered sections and no raw JSON text, an empty state on a fixture without that projection, and the posture reason
when it is unavailable; each Readiness item's first line is its guidance sentence, with fact, timing, and blocking flag
inside the technical disclosure; clicking a Readiness item selects its card; and the Review tab names the branch and
command and has no control that opens a pull request. No producer grammar exists yet for a Playwright test (SI-228
parses go-test refs only), so this obligation is declared unresolved design debt (SI-288) and names its evidence here.
