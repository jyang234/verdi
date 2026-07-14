---
id: obligation/worktree-manager--ac-4--behavioral
kind: obligation
title: "Four fixture worktrees (merged-clean, merged-dirty, merged-locked, unmerged) each get exactly the ratified outcome"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/worktree-manager" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# Four fixture worktrees (merged-clean, merged-dirty, merged-locked, unmerged) each get exactly the ratified outcome

The behavioral evidence must show a Go test over a fixturegit repository carrying four managed worktrees: one merged-and-clean, one merged-but-dirty (an uncommitted edit), one merged-but-currently-lock-held (a live simulated owner), and one still-unmerged — asserting a `gc` run removes exactly the first, and disclosed-and-keeps the other three, each with a message distinguishing which of the three keep-reasons applied (never one undifferentiated "kept" message for all three).
