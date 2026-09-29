---
id: obligation/index--ac-2--behavioral
kind: obligation
title: "Each card's facts, links, and test ids"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/index" }
frozen: { at: 2026-09-29, commit: c0adfe55a49339a2ba6789f276833544f85d764f }
---
# Each card's facts, links, and test ids

The behavioral evidence is the Playwright file `e2e/tests/96-index-pipeline.spec.ts`. For a default-branch feature with
stories, a component, an archived spec, and local and remote drafts, the test asserts each card's title, ref, badge,
links (board only where servable; matrix and verdict only on the feature with stories), age, source, in-review chip on
the draft with an open pull request, next move, disclosure, and dir-group and dir-entry test ids. No producer grammar
exists yet for a Playwright test (SI-228 parses go-test refs only), so this obligation is declared unresolved design
debt (SI-288) and names its evidence here.
