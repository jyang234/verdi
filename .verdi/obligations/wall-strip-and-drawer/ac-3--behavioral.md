---
id: obligation/wall-strip-and-drawer--ac-3--behavioral
kind: obligation
title: "Commit and push shows the three change states"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - { type: verifies, ref: "spec/wall-strip-and-drawer" }
frozen: { at: 2026-09-29, commit: fe196661a03bda803eaa0863c6514279c102fd7c }
---
# Commit and push shows the three change states

The behavioral evidence is the Playwright file `e2e/tests/92-wall-commit-changes.spec.ts`. Over harness walls with
typed-only, unclassified-only, mixed, and unreadable changes, the test opens the Commit and push popover and asserts
each state rendered distinctly with its badges, and asserts the uncommitted indicator stays set on the unclassified-only
wall. No producer grammar exists yet for a Playwright test (SI-228 parses go-test refs only), so this obligation is
declared unresolved design debt (SI-288) and names its evidence here.
