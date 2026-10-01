---
id: obligation/document-page-v2--ac-1--behavioral
kind: obligation
title: "The temporal stamp, identity card, and contents rail"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The Playwright test `document-page › The temporal stamp, identity card, and contents rail` in e2e/tests/94-document-page.spec.ts passes when CI job verify's producer runs that file alone. For a proposed and an accepted spec, the test asserts the stamp's state, commit, and a refreshed time that changes after Refresh; the identity card's ref, class, branch, owners, and files; the rail's sections and counts; and the Refresh control and the not-authority stamp."
  falsifier: "The test fails, times out, is interrupted, needs another attempt, is skipped, or does not run (its title path absent from the report), or any assertion the claim describes does not hold."
  scope: "e2e/tests/94-document-page.spec.ts, run once with one worker and no retries by CI job verify's Playwright producer, against the e2e harness's fixture stores in Chromium."
  producer: { kind: test, ref: "playwright:e2e/tests/94-document-page.spec.ts:document-page › The temporal stamp, identity card, and contents rail" }
  authoritative_source: { kind: ci-job, ref: "verify" }
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun playwright:e2e/tests/94-document-page.spec.ts:document-page › The temporal stamp, identity card, and contents rail in CI job verify at the exact candidate commit after any governing specification or code change."
links:
  - { type: verifies, ref: "spec/document-page-v2" }
frozen: { at: 2026-09-30, commit: b7d2df0dfe0eaaffedac86d866fac238f7e3c5f1 }
---
# The temporal stamp, identity card, and contents rail

CI job `verify` must record producer `playwright:e2e/tests/94-document-page.spec.ts:document-page › The temporal stamp,
identity card, and contents rail` at the exact candidate commit. For a proposed and an accepted spec, the test asserts
the stamp's state, commit, and a refreshed time that changes after Refresh; the identity card's ref, class, branch,
owners, and files; the rail's sections and counts; and the Refresh control and the not-authority stamp. The test passes
when its file runs alone (Playwright producer design §6; BL-98).
