---
id: obligation/readiness-page--ac-2--behavioral
kind: obligation
title: "Focus next ranks every concern, guidance first"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/readiness-page" }
frozen: { at: 2026-09-29, commit: 01c200e50eacb38505ba12850d187be361d1301a }
---
# Focus next ranks every concern, guidance first

The behavioral evidence is the Playwright file `e2e/tests/93-readiness-page.spec.ts`. The test asserts the current
step's concerns rank before the waiting ones with their step and now or later marks; each item's first line is its
guidance sentence, with fact, timing, and blocking flag inside the technical disclosure; expanding the waiting concerns
shows them inline; and the rendered concerns equal the readiness facts' complete set. No producer grammar exists yet for
a Playwright test (SI-228 parses go-test refs only), so this obligation is declared unresolved design debt (SI-288) and
names its evidence here.
