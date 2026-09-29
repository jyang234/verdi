---
id: obligation/document-page--ac-1--behavioral
kind: obligation
title: "The temporal stamp, identity card, and contents rail"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/document-page" }
frozen: { at: 2026-09-29, commit: bdf8e882378163cc18731f1367d6080b540e8271 }
---
# The temporal stamp, identity card, and contents rail

The behavioral evidence is the Playwright file `e2e/tests/94-document-page.spec.ts`. For a proposed and an accepted
spec, the test asserts the stamp's state, commit, and a refreshed time that changes after Refresh; the identity card's
ref, class, branch, owners, and files; the rail's sections and counts; and the Refresh control and the not-authority
stamp. No producer grammar exists yet for a Playwright test (SI-228 parses go-test refs only), so this obligation is
declared unresolved design debt (SI-288) and names its evidence here.
