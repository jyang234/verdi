---
id: obligation/index--ac-6--behavioral
kind: obligation
title: "The list view, keyboard movement, and an honest failure"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/index" }
frozen: { at: 2026-09-29, commit: c0adfe55a49339a2ba6789f276833544f85d764f }
---
# The list view, keyboard movement, and an honest failure

The behavioral evidence is the Playwright file `e2e/tests/97-index-list.spec.ts`. The test switches to the list view and
asserts the same groups, entries, and labels; moves between cards with the keyboard and opens one with Enter; and, with
the harness index computation failing, asserts one disclosure and no partial groups in either view. No producer grammar
exists yet for a Playwright test (SI-228 parses go-test refs only), so this obligation is declared unresolved design
debt (SI-288) and names its evidence here.
