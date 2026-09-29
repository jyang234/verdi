---
id: obligation/document-page--ac-4--behavioral
kind: obligation
title: "The Document page at 320 px, 200 % zoom, and without JavaScript"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/document-page" }
frozen: { at: 2026-09-29, commit: bdf8e882378163cc18731f1367d6080b540e8271 }
---
# The Document page at 320 px, 200 % zoom, and without JavaScript

The behavioral evidence is the Playwright file `e2e/tests/94-document-page.spec.ts`. At 320 px and at 200 % zoom the
test asserts no horizontal scroll; with JavaScript disabled it asserts the stamp's state and commit, the rail, the body,
and no id chip elements. No producer grammar exists yet for a Playwright test (SI-228 parses go-test refs only), so this
obligation is declared unresolved design debt (SI-288) and names its evidence here.
