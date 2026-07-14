---
id: obligation/worktree-manager--ac-3--static
kind: obligation
title: "Lock acquisition happens before any git worktree add, and a lock-loss path reuses rather than races"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/worktree-manager" }
frozen: { at: 2026-07-14, commit: c464efb6133e449257c48738ab66ae93a0e071e5 }
---
# Lock acquisition happens before any git worktree add, and a lock-loss path reuses rather than races

The static evidence must show `EnsureWorktree`'s code path acquires the per-worktree lock (`internal/filelock`, dc-2 — the same `O_CREATE|O_EXCL` `{pid,start}` algorithm as `internal/mcpserve`'s existing per-checkout lock, extracted to the shared package) strictly before any `git worktree add` call, and that losing the acquisition race (the lock is already held live) leads to a reuse/wait path that returns the winner's path rather than proceeding to a second, competing `git worktree add`. It must also show `gc`'s removal path checks this same lock's liveness before calling `git worktree remove`.
