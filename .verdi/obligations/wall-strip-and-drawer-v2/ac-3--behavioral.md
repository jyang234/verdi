---
id: obligation/wall-strip-and-drawer-v2--ac-3--behavioral
kind: obligation
title: "Commit and push shows the three change states"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `wall-strip-and-drawer › Commit and push shows the three change states` in e2e/tests/92-wall-commit-changes.spec.ts passes when CI job verify's producer runs that file alone. Over harness walls with typed-only, unclassified-only, mixed, and unreadable changes, the test opens the Commit and push popover and asserts each state rendered distinctly with its badges, and asserts the uncommitted indicator stays set on the unclassified-only wall."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/92-wall-commit-changes.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/92-wall-commit-changes.spec.ts:wall-strip-and-drawer › Commit and push shows the three change states" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/92-wall-commit-changes.spec.ts:wall-strip-and-drawer › Commit and push shows the three change states in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/wall-strip-and-drawer-v2" }
frozen: { at: 2026-09-30, commit: da729113549288efdee9961dcfa089c2d4ac199a }
---
# Commit and push shows the three change states

CI job `verify` must record producer `playwright:e2e/tests/92-wall-commit-changes.spec.ts:wall-strip-and-drawer › Commit
and push shows the three change states` at the exact candidate commit. Over harness walls with typed-only,
unclassified-only, mixed, and unreadable changes, the test opens the Commit and push popover and asserts each state
rendered distinctly with its badges, and asserts the uncommitted indicator stays set on the unclassified-only wall. The
test passes when its file runs alone (Playwright producer design §6; BL-98).
