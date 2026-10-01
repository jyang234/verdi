---
id: obligation/index-v2--ac-6--behavioral
kind: obligation
title: "The list view, keyboard movement, and an honest failure"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `index › The list view, keyboard movement, and an honest failure` in e2e/tests/97-index-list.spec.ts passes when CI job verify's producer runs that file alone. The test switches to the list view and asserts the same groups, entries, and labels; moves between cards with the keyboard and opens one with Enter; and, with the harness index computation failing, asserts one disclosure and no partial groups in either view."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/97-index-list.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/97-index-list.spec.ts:index › The list view, keyboard movement, and an honest failure" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/97-index-list.spec.ts:index › The list view, keyboard movement, and an honest failure in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/index-v2" }
frozen: { at: 2026-09-30, commit: 790c6f3f8e53f5947dad36c24fb57a9591e219d6 }
---
# The list view, keyboard movement, and an honest failure

CI job `verify` must record producer `playwright:e2e/tests/97-index-list.spec.ts:index › The list view, keyboard
movement, and an honest failure` at the exact candidate commit. The test switches to the list view and asserts the same
groups, entries, and labels; moves between cards with the keyboard and opens one with Enter; and, with the harness index
computation failing, asserts one disclosure and no partial groups in either view. The test passes when its file runs
alone (Playwright producer design §6; BL-98).
