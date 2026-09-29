---
id: obligation/chrome-and-tokens--ac-1--behavioral
kind: obligation
title: "One top bar on every workbench page, and none of the old header rows"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/chrome-and-tokens" }
frozen: { at: 2026-09-29, commit: b9f225e713231477b90e4c86cdd3f038e16fb249 }
---
# One top bar on every workbench page, and none of the old header rows

The behavioral evidence is the Playwright file `e2e/tests/87-workbench-topbar.spec.ts`. The test visits each workbench
page over a fixture store: the wall on the default branch and on a design branch, the Document page, the diagram editor,
the readiness page, the index, and the matrix, verdict, disclosures, spec import, corpus, not-found, and error pages. On
each it asserts exactly one `[data-testid=topbar]`, no `.site-head`, `.board-head`, or `.asd-posture` element, the
wordmark linking to `/`, the page title, and the branch and posture text; and on a page about one spec, the class and
mode chips. No producer grammar exists yet for a Playwright test (SI-228 parses go-test refs only), so this obligation
is declared unresolved design debt (SI-288) and names its evidence here.
