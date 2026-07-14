---
id: obligation/worktree-manager--ac-1--static
kind: obligation
title: "EnsureWorktree's only git-worktree-mutating call is worktree add, and reuse never re-executes it"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/worktree-manager" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# EnsureWorktree's only git-worktree-mutating call is worktree add, and reuse never re-executes it

The static evidence must show `internal/wtmanager.EnsureWorktree`'s implementation calls `git worktree add` exactly once per never-before-cut branch, that this call targets the deterministic `.verdi/data/worktrees/<name>/` path (dc-1) computed from the branch name with no hashing or second slugging scheme, and that the reuse path (the path already exists on disk) returns early without invoking `git worktree add` again. It must also show no call to `checkout` or `switch` against the serving checkout's own root anywhere in this function.
