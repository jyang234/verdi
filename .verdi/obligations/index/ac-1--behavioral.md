---
id: obligation/index--ac-1--behavioral
kind: obligation
title: "Four columns, every spec once, counts and empty states"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/index" }
frozen: { at: 2026-09-29, commit: c0adfe55a49339a2ba6789f276833544f85d764f }
---
# Four columns, every spec once, counts and empty states

The behavioral evidence is the Playwright file `e2e/tests/96-index-pipeline.spec.ts`. Over a fixture store spanning
every status and both zones plus design-branch drafts, the test asserts the four columns in order, each spec exactly
once in its group, each column's count, and an empty state on a fixture where a column is empty. No producer grammar
exists yet for a Playwright test (SI-228 parses go-test refs only), so this obligation is declared unresolved design
debt (SI-288) and names its evidence here.
