---
id: obligation/worktree-manager--ac-1--behavioral
kind: obligation
title: "First call cuts a real worktree with the branch's content; second call reuses it, no re-cut"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/worktree-manager" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# First call cuts a real worktree with the branch's content; second call reuses it, no re-cut

The behavioral evidence must show a Go test over a `fixturegit` repository with a local design branch carrying a committed spec.md distinct from the default branch's own content, asserting a first `EnsureWorktree` call returns a path whose on-disk `spec.md` matches the design branch's content (a real `git worktree add` ran) and that the serving checkout's own working tree is unchanged (byte-identical `HEAD` and working-tree hash before/after, mirroring ref-index's own such proof). A second `EnsureWorktree` call for the same branch must return the identical path, and the test must assert no second `git worktree add` occurred (e.g. via `git worktree list` showing exactly one entry for that branch, or an instrumented/counted runner).
