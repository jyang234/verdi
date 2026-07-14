---
id: obligation/worktree-manager--ac-2--behavioral
kind: obligation
title: "Remote-only, nonexistent, and already-checked-out-elsewhere branches each produce the right named refusal, no worktree created"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/worktree-manager" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# Remote-only, nonexistent, and already-checked-out-elsewhere branches each produce the right named refusal, no worktree created

The behavioral evidence must show a Go test over a fixture repository asserting three cases each refuse without creating a worktree directory: (1) a branch existing only as a remote-tracking ref, (2) a branch name that resolves to no ref at all, and (3) a branch already checked out in the serving checkout's own root. Each must return the specific named error from the static obligation's type (or a case-distinguishable wrapped form) rather than an undifferentiated generic error, and `.verdi/data/worktrees/` must contain no new entry for any of the three.
