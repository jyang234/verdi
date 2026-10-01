---
id: obligation/chrome-and-tokens-v2--ac-1--behavioral
kind: obligation
title: "One top bar on every workbench page, and none of the old header rows"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `chrome-and-tokens › One top bar on every workbench page, and none of the old header rows` in e2e/tests/87-workbench-topbar.spec.ts passes when CI job verify's producer runs that file alone. The test visits each workbench page over a fixture store: the wall on the default branch and on a design branch, the Document page, the diagram editor, the readiness page, the index, and the matrix, verdict, disclosures, spec import, corpus, not-found, and error pages. On each it asserts exactly one `[data-testid=topbar]`, no `.site-head`, `.board-head`, or `.asd-posture` element, the wordmark linking to `/`, the page title, and the branch and posture text; on a page about one spec, the class and mode chips; and on the diagram editor, the `diagram-exit` affordance in the bar, distinct from the index and artifact links."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/87-workbench-topbar.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/87-workbench-topbar.spec.ts:chrome-and-tokens › One top bar on every workbench page, and none of the old header rows" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/87-workbench-topbar.spec.ts:chrome-and-tokens › One top bar on every workbench page, and none of the old header rows in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/chrome-and-tokens-v2" }
frozen: { at: 2026-09-30, commit: 05bdbebd01ba11c2348fb4c4e5ec8d2195274131 }
---
# One top bar on every workbench page, and none of the old header rows

CI job `verify` must record producer `playwright:e2e/tests/87-workbench-topbar.spec.ts:chrome-and-tokens › One top bar
on every workbench page, and none of the old header rows` at the exact candidate commit. The test visits each workbench
page over a fixture store: the wall on the default branch and on a design branch, the Document page, the diagram editor,
the readiness page, the index, and the matrix, verdict, disclosures, spec import, corpus, not-found, and error pages. On
each it asserts exactly one `[data-testid=topbar]`, no `.site-head`, `.board-head`, or `.asd-posture` element, the
wordmark linking to `/`, the page title, and the branch and posture text; on a page about one spec, the class and mode
chips; and on the diagram editor, the `diagram-exit` affordance in the bar, distinct from the index and artifact links.
The test passes when its file runs alone (Playwright producer design §6; BL-98).
