---
id: obligation/index--ac-5--behavioral
kind: obligation
title: "On the shelf, the other-records strip, and the kept directory notices"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/index" }
frozen: { at: 2026-09-29, commit: 3d75b4036991122dd1ee3372ba84b509202170de }
---
# On the shelf, the other-records strip, and the kept directory notices

The behavioral evidence is the Playwright file `e2e/tests/96-index-pipeline.spec.ts`. The test asserts terminal
active-zone specs in On the shelf with archived specs in the collapsed list at its foot; each other-records section
collapsed with its count and expandable without JavaScript to its full listing; the Disclosures toggle; the disclosed
entry for a branch with no draft; and the deleted-branch notice page. No producer grammar exists yet for a Playwright
test (SI-228 parses go-test refs only), so this obligation is declared unresolved design debt (SI-288) and names its
evidence here.
