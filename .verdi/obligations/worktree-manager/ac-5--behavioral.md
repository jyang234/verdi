---
id: obligation/worktree-manager--ac-5--behavioral
kind: obligation
title: "verdi gc's real CLI output names its own scope limitation alongside a real reclaim/keep report"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/worktree-manager" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# verdi gc's real CLI output names its own scope limitation alongside a real reclaim/keep report

The behavioral evidence must show an end-to-end CLI test invoking the built `verdi gc` binary against a fixture checkout with at least one reclaimable managed worktree, asserting: exit code 0, the worktree actually removed on disk, a printed line reporting the reclaim, and a printed line disclosing that derived-cache/layout-cache pruning were not run by this invocation (verbatim scope disclosure, not merely implied by omission).
