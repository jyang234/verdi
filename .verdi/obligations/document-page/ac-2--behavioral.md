---
id: obligation/document-page--ac-2--behavioral
kind: obligation
title: "Id chips open the wall with the card selected"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/document-page" }
frozen: { at: 2026-09-29, commit: 88ebe32308f48b37711592d4fc21bcde2d166bbe }
---
# Id chips open the wall with the card selected

The behavioral evidence is the Playwright file `e2e/tests/94-document-page.spec.ts`. For an acceptance criterion, a
constraint, a decision, and an open question, the test clicks its id chip and asserts the wall opens with that card
selected and in view. No producer grammar exists yet for a Playwright test (SI-228 parses go-test refs only), so this
obligation is declared unresolved design debt (SI-288) and names its evidence here.
